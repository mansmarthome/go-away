const u = (url = "", params = {}) => {
	let result = new URL(url, window.location.href);
	Object.entries(params).forEach((kv) => {
		let [k, v] = kv;
		result.searchParams.set(k, v);
	});
	return result.toString();
};

window.smartCaptchaCallback = function (token) {
	const redir = window.location.href;
	window.location.href = u("{{ .VerifyPath }}", {
		__goaway_token: token,
		__goaway_challenge: "{{ .Challenge }}",
		__goaway_redirect: redir,
		__goaway_id: "{{ .Id }}",
		__goaway_bust: Date.now(),
	});
};

window.yandexSmartCaptchaOnload = function () {
	if (window.smartCaptcha) {
		window.smartCaptcha.render('yandex-smartcaptcha-container', {
			sitekey: "{{ .Sitekey }}",
			hl: "{{ .Hl }}",
			callback: 'smartCaptchaCallback',
		});
	}
};
