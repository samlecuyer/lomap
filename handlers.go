package main

import (
	"fmt"
	"encoding/json"
	"net/http"
	"strings"
	"errors"
	"log"

	"strconv"
	"sort"

	"github.com/goods/httpbuf"
	"github.com/samlecuyer/redactomat"
	"code.google.com/p/rsc/imap"
)

type handler func(http.ResponseWriter, *http.Request, *Context) error
func (h handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	//create the context
	ctx, err := NewContext(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	//run the handler and grab the error, and report it
	buf := new(httpbuf.Buffer)
	err = h(buf, req, ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	//save the session
	if err = ctx.Session.Save(req, buf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	//apply the buffered response to the writer
	buf.Apply(w)
}

func root(w http.ResponseWriter, r *http.Request, ctx *Context) error {
	if _, err := ctx.GetClient(); err == nil {
		http.Redirect(w, r, "/mail", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
	return nil
}

func Logout(w http.ResponseWriter, r *http.Request, ctx *Context) error {
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return ctx.Logout()
}

func LoginForm(w http.ResponseWriter, r *http.Request, ctx *Context) error {
	return T("login.html").Execute(w, map[string]interface{}{
		"ctx": ctx,
	})
}

func LoginHandler(w http.ResponseWriter, r *http.Request, ctx *Context) error {
	host := r.FormValue("host")
	name := r.FormValue("username")
	pass := r.FormValue("password")

	_, err := ctx.PerformLogin(host, name, pass)
	w.Header().Add("Location", "/mail")
	return err
}

func InboxHandler(w http.ResponseWriter, r *http.Request, ctx *Context) error {
	client, _ := ctx.GetClient()
	return T("inbox.html").Execute(w, map[string]interface{}{
		"ctx": ctx,
		"bxs": client.Boxes(),
	})
}


func max(a, b uint32) uint32 {
	if a > b { return a }
	return b
}
func minusf(a, b uint32) uint32 {
	if a - b > a { return 0 }
	return a - b
}

type Message struct {
	Uid string
	Hdr *imap.MsgHdr
	Seen bool
	BoxName string
}

// TODO: more than just the last 50 of the inbox
// TODO: offline mode.  I think google throttles my requests
// after spending the day killing and restarting the server
// TODO: possibly split the imap client into a separate process
func MessagesHandler(w http.ResponseWriter, r *http.Request, ctx *Context) error {
	client, err := ctx.GetClient()
	if err != nil { return err }

	var mailbox *imap.Box
	boxName := r.URL.Query().Get("box")
	log.Println(r.URL)
	if boxName == "" {
		mailbox = client.Inbox()
	} else {
		mailbox = client.Box(boxName)
	}
	if mailbox == nil {
		w.WriteHeader(404)
		return nil
	}

	mailbox.Check()
	msgs := mailbox.Msgs()

	n := len(msgs)
	var headers []*Message
	if n > 0 {
		mn := max(1, (minusf(uint32(n), 50)))
		headers = make([]*Message, uint32(n) - mn)
		for i, msg := range msgs[mn:] {
			headers[i] = &Message{
				fmt.Sprintf("%v",msg.UID),
				msg.Hdr,
				(msg.Flags & imap.FlagSeen) == imap.FlagSeen,
				msg.Box.Name,
			}
		}
	} else {
		headers = make([]*Message, 0)
	}

	b, err := json.Marshal(headers)
	if err != nil {
		return err
	}
	w.WriteHeader(200)
	w.Write(b)
	return nil
}

func handleMixed(msg *imap.MsgPart) (parts []*MessagePart) {
	parts = make([]*MessagePart, len(msg.Child))
	for i, part := range msg.Child {
		parts[i] = &MessagePart{ ID: part.ID, Name: part.Name, Type: part.Type }
		switch part.Type {
		case "text/plain":
			parts[i].Contents = string(part.Text())
		case "text/html":
			parts[i].Contents, _ = redactomat.RedactString(string(part.Text()))
		case "multipart/alternative":
			parts[i].Children = handleAlternative(msg)
		}
	}
	return parts
}

func handleAlternative(msg *imap.MsgPart) (parts []*MessagePart) {
	parts = make([]*MessagePart, len(msg.Child))
	for i, part := range msg.Child {
		parts[i] = &MessagePart{ ID: part.ID, Name: part.Name, Type: part.Type }
		switch part.Type {
		case "text/plain":
			parts[i].Contents = string(part.Text())
		case "text/html":
			parts[i].Contents, _ = redactomat.RedactString(string(part.Text()))
		}
	}
	return parts
}

func handleMail(msg *imap.MsgPart) (parts *MessagePart) {
	parts = &MessagePart{ ID: msg.ID, Name: msg.Name, Type: msg.Type }
	switch msg.Type {
	case "text/plain":
		parts.Contents = string(msg.Text())
	case "text/html":
		parts.Contents, _ = redactomat.RedactString(string(msg.Text()))
	case "multipart/alternative":
		parts.Children = handleAlternative(msg)
	case "multipart/mixed":
		parts.Children = handleMixed(msg)
	}
	return parts
}

type MessagePart struct {
	ID, Name, Type string
	Contents string
	Children []*MessagePart
}

func MessageHandler(w http.ResponseWriter, r *http.Request, ctx *Context) error {
	client, err := ctx.GetClient()
	if err != nil { return err }

	uidstr := r.URL.Query().Get(":id")

	var mailbox *imap.Box
	boxName := r.URL.Query().Get("box")
	if boxName == "" {
		mailbox = client.Inbox()
	} else {
		mailbox = client.Box(boxName)
	}
	if mailbox == nil {
		w.WriteHeader(404)
		return nil
	}

	mailbox.Check()
	msgs := mailbox.Msgs()

	uid, err := strconv.ParseUint(uidstr, 10, 64)
	if err != nil {
		return err
	}
	exists := len(msgs)
	entryId := sort.Search(exists, 
		func(i int) bool { 
			return msgs[i].UID >= uid 
		})
	if entryId < exists && msgs[entryId] != nil {
		parts := handleMail(&(msgs[entryId].Root))
		b, err := json.Marshal(parts)
		if err != nil {
			return err
		}
		w.WriteHeader(200)
		w.Write(b)
	} else {
		w.WriteHeader(404)
	}
	return nil
}

func ArchiveHandler(w http.ResponseWriter, r *http.Request, ctx *Context) error {
	client, err := ctx.GetClient()
	if err != nil { return err }

	uidstr := r.URL.Query().Get(":id")

	boxname := r.URL.Query().Get("box")
	mailbox := client.Box(boxname)
	if mailbox == nil {
		w.WriteHeader(404)
		return nil
	}
	mailbox.Check()
	msgs := mailbox.Msgs()

	uid, err := strconv.ParseUint(uidstr, 10, 64)
	if err != nil {
		return err
	}
	exists := len(msgs)
	entryId := sort.Search(exists, 
		func(i int) bool { 
			return msgs[i].UID >= uid 
		})
	if entryId < exists && msgs[entryId] != nil {
		allMail := client.Box("[Gmail]/All Mail")
		if allMail == nil {
			return errors.New("Could not find all mail")
		}
		err = allMail.Copy(msgs[entryId:entryId+1])
		if err != nil { return err }
		err = mailbox.Delete(msgs[entryId:entryId+1])
		if err != nil { return err }
		w.WriteHeader(200)
	} else {
		w.WriteHeader(404)
	}
	return nil
}

func DeleteHandler(w http.ResponseWriter, r *http.Request, ctx *Context) error {
	client, err := ctx.GetClient()
	if err != nil { return err }

	uidstr := r.URL.Query().Get(":id")

	boxname := r.URL.Query().Get(":box")
	mailbox := client.Box(boxname)
	if mailbox == nil {
		w.WriteHeader(404)
		return nil
	}
	mailbox.Check()
	msgs := mailbox.Msgs()

	uid, err := strconv.ParseUint(uidstr, 10, 64)
	if err != nil {
		return err
	}
	exists := len(msgs)
	entryId := sort.Search(exists, 
		func(i int) bool { 
			return msgs[i].UID >= uid 
		})
	if entryId < exists && msgs[entryId] != nil {
		err = mailbox.Delete(msgs[entryId:entryId+1])
		if err != nil { return err }
		w.WriteHeader(200)
	} else {
		w.WriteHeader(404)
	}
	return nil
}

func AttachmentHandler(w http.ResponseWriter, r *http.Request, ctx *Context) error {
	client, err := ctx.GetClient()
	if err != nil { return err }

	mailbox := client.Inbox()
	mailbox.Check()
	msgs := mailbox.Msgs()


	msgIdStr := r.URL.Query().Get(":msg")
	attId := r.URL.Query().Get(":id")

	uid, err := strconv.ParseUint(msgIdStr, 10, 64)
	if err != nil { return err }

	entryId := sort.Search(len(msgs), func(i int) bool { return msgs[i].UID >= uid })

	root := &(msgs[entryId].Root)
	var attachment []byte
	var contenttype, dispo string
	if strings.Contains(root.Type, "multipart/mixed") {
		for _, part := range root.Child {
			if part.ID == attId {
				attachment = part.Text()
				contenttype = part.Type
				dispo = fmt.Sprintf("inline; filename=\"%s\"", part.Name)
			}
		}
	}

	w.Header().Add("Content-Type", contenttype)
	w.Header().Add("Content-Disposition", dispo)
	w.WriteHeader(200)
	w.Write(attachment)
	return nil
}