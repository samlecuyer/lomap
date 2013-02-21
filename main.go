package main

import (
	"html/template"
	"net/http"
	"errors"
	"github.com/bmizerany/pat"
	// "github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"

	"code.google.com/p/rsc/imap"
)

var (
	templ   *template.Template
	store   *sessions.CookieStore
	clients map[string]*imap.Client
	NoClientPresent = errors.New("No imap client present")
)

type Context struct {
	Session *sessions.Session
}

func NewContext(req *http.Request) (*Context, error) {
	sess, err := store.Get(req, "imap")
	return &Context{sess}, err
}

func (ctx *Context) GetClient() (*imap.Client, error) {
	clientKey, ok := ctx.Session.Values["client"].(string)
	if !ok {
		return nil, NoClientPresent
	}
	client, ok := clients[clientKey]
	if !ok {
		user, ok := ctx.Session.Values["username"].(string)
		if !ok { return nil, NoClientPresent }
		pass, ok := ctx.Session.Values["pass"].(string)
		if !ok { return nil, NoClientPresent }
		host, ok := ctx.Session.Values["host"].(string)
		if !ok { return nil, NoClientPresent }
		return imap.NewClient(imap.TLS, host, user, pass, "")
	}
	return client, nil
}

func (ctx *Context) PerformLogin(host, name, pass string) (*imap.Client, error) {
	client, err := imap.NewClient(imap.TLS, host, name, pass, "")
	if err != nil {
		return nil, err
	}
	ctx.Session.Values["username"] = name
	ctx.Session.Values["pass"] = pass
	ctx.Session.Values["host"] = host

	clientKey := name + "@" + host
	ctx.Session.Values["client"] = clientKey
	clients[clientKey] = client
	return client, err
}

func (ctx *Context) Logout() error {
	if clientKey, ok := ctx.Session.Values["client"].(string); ok {
		if client, ok := clients[clientKey]; ok {
			client.Close()
			delete(clients, clientKey)
		}
	}
	delete(ctx.Session.Values, "username")
	delete(ctx.Session.Values, "pass")
	delete(ctx.Session.Values, "host")
	return nil
}


func main() {
	store = sessions.NewCookieStore([]byte("this-is-super-secret"))
		// securecookie.GenerateRandomKey(32),
		// securecookie.GenerateRandomKey(32))
	store.Options.HttpOnly = true
	store.Options.Secure = true

	clients = make(map[string]*imap.Client)

	m := pat.New()
	m.Get("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	m.Get("/login", handler(LoginForm))
	m.Post("/login", handler(LoginHandler))
	
	m.Get("/logout", handler(Logout))

	m.Get("/mail", handler(InboxHandler))
	m.Get("/mail/messages/", handler(MessagesHandler))
	m.Get("/mail/message/:id", handler(MessageHandler))
	m.Get("/mail/attachment/:msg/:id", handler(AttachmentHandler))

	m.Post("/mail/archive/:id", handler(ArchiveHandler))
	m.Post("/mail/delete/:id", handler(DeleteHandler))

	m.Get("/", handler(root))
	http.Handle("/", m)
	http.ListenAndServeTLS(":5000", "certs/newcert.pem",  "certs/privkey.pem", nil)
}
