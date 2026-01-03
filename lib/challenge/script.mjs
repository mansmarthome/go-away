import {setup, challenge} from "{{ .ChallengeScript }}";


// from Xeact
const u = (url = "", params = {}) => {
    let result = new URL(url, window.location.href);
    Object.entries(params).forEach((kv) => {
        let [k, v] = kv;
        result.searchParams.set(k, v);
    });
    return result.toString();
};

(async () => {
    const status = document.getElementById('status');
    const title = document.getElementById('title');

    status.innerText = '{{ .Strings.Get "status_starting_challenge" }} {{ .Challenge }}...';

    try {
        const info = await setup({
            Path: "{{ .Path }}",
            Parameters: "{{ .Parameters }}"
        });

        if (info != "") {
            status.innerText = '{{ .Strings.Get "status_calculating" }} ' + info
        } else {
            status.innerText = '{{ .Strings.Get "status_calculating" }}';
        }
    } catch (err) {
        title.innerHTML = '{{ .Strings.Get "title_error" }}';
        status.innerHTML = `{{ .Strings.Get "status_error" }} ${err.message}`;
        return
    }


    try {
        const t0 = Date.now();
        const { result, info } = await challenge();
        const t1 = Date.now();
        console.log({ result, info });

        title.innerHTML = '{{ .Strings.Get "status_challenge_success" }}';
        if (info != "") {
            status.innerHTML = `{{ .Strings.Get "status_challenge_done_took" }} ${t1 - t0}ms, ${info}`;
        } else {
            status.innerHTML = `{{ .Strings.Get "status_challenge_done_took" }} ${t1 - t0}ms`;
        }

        setTimeout(() => {
            let redir = new URL(window.location.href);
            if (document.referrer) {
                redir.searchParams.set("utm_referrer", document.referrer);
            }
            window.location.href = u("{{ .Path }}/verify-challenge", {
                __goaway_token: result,
                __goaway_challenge: "{{ .Challenge }}",
                __goaway_redirect: redir.toString(),
                __goaway_id: "{{ .Id }}",
                __goaway_elapsedTime: t1 - t0,
            });
        }, 500);
    } catch (err) {
        title.innerHTML = '{{ .Strings.Get "title_error" }}';
        status.innerHTML = `{{ .Strings.Get "status_error" }} ${err.message}`;
    }
})();
