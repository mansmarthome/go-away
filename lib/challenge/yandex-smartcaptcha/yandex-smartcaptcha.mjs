const u = (url = "", params = {}) => {
	let result = new URL(url, window.location.href);
	Object.entries(params).forEach((kv) => {
		let [k, v] = kv;
		result.searchParams.set(k, v);
	});
	return result.toString();
};

const status = document.getElementById('status');
status.innerHTML = '{{ .Strings.Get "details_title" }}';

window.smartCaptchaCallback = function (token) {
	let redir = new URL(window.location.href);
	if (document.referrer) {
			redir.searchParams.set("utm_referrer", document.referrer);
	}
	window.location.href = u("{{ .VerifyPath }}", {
		__goaway_token: token,
		__goaway_challenge: "{{ .Challenge }}",
		__goaway_redirect: redir.toString(),
		__goaway_id: "{{ .Id }}",
		__goaway_bust: Date.now(),
	});
};

window.yandexSmartCaptchaOnload = function () {
	const loader = document.getElementById('loading-indicator');
	if (loader) {
		loader.classList.add('hidden');
	}

	try {
		window.smartCaptcha.render('yandex-smartcaptcha-container', {
			sitekey: "{{ .Sitekey }}",
			hl: "{{ .Hl }}",
			callback: 'smartCaptchaCallback',
			theme: "auto",
		});
	} catch(err) {
		window.yandexSmartCaptchaOnerror();
	}
};

window.yandexSmartCaptchaOnerror = function () {
	status.innerHTML = '{{ .Strings.Get "status_captcha_unavailable" }}';

	const loader = document.getElementById('loading-indicator');
	if (loader) {
		loader.classList.add('hidden');
	}

	const refreshButton = document.getElementById('refresh-button');
	if (refreshButton) {
		refreshButton.classList.remove('hidden');
	}
};
