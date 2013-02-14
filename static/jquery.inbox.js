(function ($) {

	var fromToString = function(froms) {
		return froms.map(function(from) {
			return from.Name || from.Email
		}).join(', ');
	};

	var textFromBody = function(body) {
		switch (body.Type) {
			case 'text/plain':
				return '<pre>'+body.Contents+'</pre>';
			case 'text/html':
				// TODO: redact images and scripts and stuff
				return body.Contents;
			case 'multipart/alternative':
				for (var part in body.Children) {
					if (body.Children[part].Type == 'text/plain') {
						return '<pre>'+body.Children[part].Contents+'</pre>';
					} else if (body.Children[part].Type == 'text/html') {
						return body.Children[part].Contents;
					}
				}
			case 'multipart/mixed':
				for (var part in body.Children) {
					if (body.Children[part].Type == 'text/plain') {
						return '<pre>'+body.Children[part].Contents+'</pre>';
					}
				}
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

	var loadEmail = function(msg) {
		var reader = $('[data-role~=reader]')
						.empty().scrollTop(0);
		
		$('<article>').addClass('emailReader')
			.append($('<header>').addClass('emailHeader')
						.append($('<div class="from">').text(fromToString(msg.Hdr.From)))
						.append($('<div class="to">').text(fromToString(msg.Hdr.To)))
						.append($('<div class="subject">')
							.text(msg.Hdr.Subject || '(no subject)'))
						.append($('<div class="attachments">')
							.html(findAttachments(msg.Body).map(function(att) {
								return $('<div>').append($('<a/>')
									.attr('href', '/inbox/attachment/'+msg.Uid+'/'+att.ID)
									.attr('target', '_blank')
									.text(att.Name)).html();
							}))))
			.append($('<section>').addClass('emailBody')
						.html(textFromBody(msg.Body)))
			.appendTo(reader);
	};

	jQuery.widget('lomap.inbox', {
		options: {

		},

		_create: function() {
			var self = this;
			this.list = $('<ul data-role="msg_list">')
				.addClass('inbox_list')
				.appendTo(this.element)
				.delegate('li', 'click',
					$.proxy(this._itemClick));
			this.refresh();
		},

		refresh: function() {
			var self = this;
			$('<div data-role="alert">').text('loading...').prependTo(self.element);
			self.items = self.items || $();
			$.get('/inbox/messages', function(data) {
				self.msgs = JSON.parse(data).reverse();
				self.msgs.forEach($.proxy(function(el,i) {
					var $el = $(el).addClass('email'),
						item = $('<li class="email" data-role="msg_listing"/>')
							.append($('<span>').addClass('from').text(fromToString(el.Hdr.From)))
							.append($('<span>').addClass('subject').text(el.Hdr.Subject ||'(no subject)'))
							.data( 'inbox.msg', el )
							.appendTo(this.list);
						if ( ! el.Seen ) {
							item.addClass('unseen');
						}
				}, self));
			}).fail(function() {
				$('[data-role~=mailbox]').text('Could not load messages.');
			}).always(function() {
				$('[data-role~=alert]', self.element).remove()
			});

			this.items = this.items || $();

		},

		_itemClick: function( event ) {
			var self = $(this).closest(':lomap-inbox'),
				$target = $(event.target).closest('[data-role~=msg_listing]')
							.addClass('selected');
			$target.siblings().removeClass('selected');

			if ($target.data('inbox.msg').Body) {
				loadEmail($target.data('inbox.msg'));
			} else {
				var uid = $target.data('inbox.msg').Uid;
				$.get('/inbox/message/'+uid, function(content) {
					$target.data('inbox.msg').Body = JSON.parse(content);
					loadEmail( $target.data( 'inbox.msg' ) );
					$target.removeClass('unseen');
				}).fail(function() {
					$('[data-role~=reader]').text("could not load");
				});
			}
		},

	});

})(jQuery);