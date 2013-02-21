$(document).ready(function($) {

	$('#login_form').submit(function(event) {
		$('#errors').empty()
		var data = $(event.target).serialize();
		$('#login_form').find('input, select').prop('disabled', true);
		$.post('/login', data, function(contents, textStatus, jqXHR) {
			window.location = jqXHR.getResponseHeader('location')
		}).fail(function(data) {
				$('#errors').text(arguments[0].responseText)
		}).always(function() {
				$('#login_form').find('input, select').prop('disabled', false);
		});
		return false;
	});
});