(function ($) {

	var fromToDl = function(froms) {
		return froms.reduceRight(function(prev, from) {
			$('<dt>').text(from.Name || from.Email).appendTo(prev);
			$('<dd>').text(from.Email).appendTo(prev);
			return prev;
		}, $('<dl>'));
	};

	var fromToString = function(froms) {
		return froms.map(function(from) {
			return from.Name || from.Email;
		}).join(', ');
	};

	var handleAlternative = function(body) {
		for (var part in body.Children) {
			if (body.Children[part].Type.substr(0,5) === 'text/') {
				return handleText(body.Children[part])
			}
		}
	}

	var handleMixed = function(body) {
		for (var part in body.Children) {
			if (body.Children[part].Type.substr(0,5) === 'text/') {
				return handleText(body.Children[part])
			} else if (body.Children[part].Type === 'multipart/alternative') {
				return handleAlternative(body.Children[part])
			}
		}
	}

	var handleText = function(body) {
		switch (body.Type) {
			case 'text/plain':
				return '<pre>'+body.Contents.autoLink({ target: "_blank", rel: "nofollow" })+'</pre>';
			case 'text/html':
				return body.Contents;
		}
	}

	var textFromBody = function(body) {
		switch (body.Type) {
			case 'text/plain':
				return '<pre>'+body.Contents.autoLink({ target: "_blank", rel: "nofollow" })+'</pre>';
			case 'text/html':
				return body.Contents;
			case 'multipart/alternative':
				return handleAlternative(body);
			case 'multipart/mixed':
				return handleMixed(body);
			default:
				return "I don't know how to load this message. "+body.Type
		}
	};

	var findAttachments = function(body) {
		if (body.Type !== 'multipart/mixed') return [];
		var attachments = []
		for (var part in body.Children) {
			if (body.Children[part].Type.substr(0,4) !== 'text') {
				attachments.push(body.Children[part]);
			}
		}
		return attachments;
	};

	jQuery.widget('lomap.reader', {
		options: {},
		_create: function() {
			return this;
		},

		loadMessage: function(fetch) {
			var self = this,
				msg = fetch.msg;
			this.element.empty().scrollTop(0);

			$('<article>').addClass('emailReader')
					.append($('<header>').addClass('emailHeader')
						.append($('<div class="from">').append(fromToDl(msg.Hdr.From)))
						.append($('<div class="to">').append(fromToDl(msg.Hdr.To)))
						.append($('<div class="subject">')
							.text(msg.Hdr.Subject || '(no subject)')))
						// TODO: show attachments and allow archiving
						// .append($('<div class="attachments">')
						// 	.html(findAttachments(msg.Body).map(function(att) {
						// 		return $('<div>').append($('<a/>')
						// 			.attr('href', '/inbox/attachment/'+msg.Uid+'/'+att.ID)
						// 			.attr('target', '_blank')
						// 			.text(att.Name)).html();
						// 	}))))
						// $('[data-role~=reader]')
						// 	.delegate('a[data-role~=archiver]', 'click',
						// 		function(evt) {
						// 			var archiveUrl = $(evt.target).attr('href')
						// 			$.post(archiveUrl, function() {
						// 				var selected = $(self.element).find('.selected'),
						// 					next = selected.prev() || selected.next();
						// 				selected.remove();
						// 				next.click()
						// 			});	
						// 			return false; 
						// 		});
					.append($('<section>').addClass('emailBody')
							.append($('<div data-role="alert">')
									.text('loading...')
										.append($('<span>').addClass('icon-spinner icon-spin pull-left'))))
					.appendTo(self.element);
			fetch.body.then(function(body) {
				// This is absolutely bizarre, but sometimes this comes through as a string
				if (typeof body === "string") body = JSON.parse(body);
				$('section.emailBody').empty().html(textFromBody(body))
			}).fail(function() {
				$('section.emailBody').text("could not load");
			});
		}

	});

	jQuery.widget('lomap.message', {
		options: {
			selected: false,
		},

		_create: function() {
			this.refresh(this.options.message)
			return this;
		},

		refresh: function(msg) {
			this.element.empty()
				.addClass('email')
				.attr('data-role', 'msg_listing')
				.append($('<span>').addClass('from').text(fromToString(msg.Hdr.From)))
				.append($('<span>').addClass('subject').text(msg.Hdr.Subject ||'(no subject)'))
				.data( 'inbox.msg', msg )
			if ( ! msg.Seen ) {
				this.element.addClass('unseen');
			}
			return this;
		},

		select: function() {
			var self = this;
			this.element.addClass('selected')
				.siblings().removeClass('selected');
			var msg = this.options.message
			if (msg.Body) {
				var fetch = $.Deferred();
				$('[data-role~=reader]').reader('loadMessage', { msg: msg, body: fetch })
				fetch.resolve(msg.Body);
			} else {
				var uid = msg.Uid,
					box = msg.BoxName;
				var fetch = $.get('/mail/message/'+uid+'?box='+encodeURIComponent(box), function(content) {
					msg.Body = JSON.parse(content);
					self.element.removeClass('unseen');
					return msg.Body;
				});
				$('[data-role~=reader]').reader('loadMessage', { msg: msg, body: fetch } );
			}
			return this;
		}

	});

	jQuery.widget('lomap.messages', {
		options: {
			mailbox: 'inbox'
		},

		_create: function() {
			var self = this;
			this.list = $('<ul data-role="msg_list">')
				.addClass('inbox_list')
				.appendTo(this.element)
				.delegate('li', 'click', $.proxy(this._itemClick));
			if (this.options.fetch) {
				this.refresh(this.options.fetch)
			}
			return this;
		},

		refresh: function(fetch) {
			var self = this;
			$('<div data-role="alert">')
				.text('loading...')
				.append( $('<span>').addClass('icon-spinner icon-spin pull-left') )
				.prependTo(self.element);
			self.items = self.items || $();
			fetch.then(function(messages) {
				self.list.empty();
				self._msgs = messages.reverse();
				if (self._msgs.length == 0) {
					$('<div data-role="alert">')
						.text('Mailbox is empty')
						.prependTo(self.element);
				}
				self._msgs.forEach($.proxy(function(el,i) {
					$('<li>').message({ message: el }).appendTo(this.list);
				}, self));
			}).fail(function() {
				$('<div data-role="alert">')
						.text('Mailbox is empty')
						.prependTo(self.element);
			}).always(function() {
				$('[data-role~=alert]', self.element).remove();
			});
		},

		_itemClick: function( event ) {
			$(event.target).closest(':lomap-message').message('select');
		},

	});

	jQuery.widget('lomap.vimbar', {
		_create: function() {
			var self = this;
			self.input = $('<input type="text"></input>');
			self.element.append(self.input);

			self.input.bind('keyup', 'esc', function(evt){
				self.close()
			});
			self.input.bind('keyup', 'return', function(evt){
				self._execute()
			});

			$(document).bind('keypress', 'shift+:', function(){
				self.open();
			});
			$(this.options.wrapper).append(this.element);
			return this;
		},

		open: function() {
			this.element.show();
			this.input.val('').select();
		},

		close: function() {
			this.element.hide();
		},

		_execute: function() {
			var self = this;
			var textToParse = this.input.val(),
				arg1;
			if (textToParse.substr(0,4) === ':mb ') {
				var mailbox = extractMailboxName(textToParse.substr(4))
				$('[data-role~=messages]').messages('refresh',
					$.ajax({
						url: '/mail/messages/?box='+encodeURIComponent(mailbox), 
						dataType: 'json'
					})
				);
			}
		},

	});

	var extractMailboxName = function(arg) {
		if (arg[0] === '~') {
			var name = new RegExp(arg.substr(1))
			for (var i in mailboxNames) {
				if (name.test(mailboxNames[i])) {
					return mailboxNames[i];
				}
			}
		} else {
			return arg;
		}
	}

	$(document).ready(function() {
		$('[data-role~=reader]').reader();

		$('<div id="vimbar">').vimbar({
			wrapper: $('[data-role=wrapper]')
		});
	});

})(jQuery);