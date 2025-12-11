package lib

import (
	"codeberg.org/meta/gzipped/v2"
	"git.gammaspectra.live/git/go-away/embed"
	"git.gammaspectra.live/git/go-away/lib/action"
	"git.gammaspectra.live/git/go-away/lib/challenge"
	"git.gammaspectra.live/git/go-away/lib/policy"
	"git.gammaspectra.live/git/go-away/utils"
	"golang.org/x/net/html"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"
)

func GetLoggerForRequest(r *http.Request) *slog.Logger {
	data := challenge.RequestDataFromContext(r.Context())
	args := []any{
		"request_id", data.Id.String(),
		"remote_address", data.RemoteAddress.Addr().String(),
		"user_agent", r.UserAgent(),
		"host", r.Host,
		"path", r.URL.Path,
		"query", r.URL.RawQuery,
	}

	if fp := utils.GetTLSFingerprint(r); fp != nil {
		if ja3n := fp.JA3N(); ja3n != nil {
			args = append(args, "ja3n", ja3n.String())
		}
		if ja4 := fp.JA4(); ja4 != nil {
			args = append(args, "ja4", ja4.String())
		}
	}
	return slog.With(args...)
}

func (state *State) fetchTags(host string, backend http.Handler, r *http.Request, meta, link bool) (result []html.Node) {
	uri := *r.URL
	q := uri.Query()
	for k := range q {
		if strings.HasPrefix(k, challenge.QueryArgPrefix) {
			q.Del(k)
		}
	}
	uri.RawQuery = q.Encode()

	key := host + uri.String()

	if cached, ok := state.tagCache.Get(key); ok {
		return cached
	}

	metaTags, linkTags := utils.FetchTags(backend, r, meta, link)

	// Combine and filter to safe tags only
	var safe []html.Node
	for _, node := range append(metaTags, linkTags...) {
		attrs := make(map[string]string, len(node.Attr))
		for _, a := range node.Attr {
			attrs[a.Key] = a.Val
		}

		keep := false
		switch node.Data {
		case "meta":
			name := attrs["name"] + attrs["property"] + attrs["itemprop"] + attrs["http-equiv"]
			if strings.HasPrefix(name, "og:") ||
				strings.HasPrefix(name, "twitter:") ||
				strings.HasPrefix(name, "fb:") ||
				strings.HasPrefix(name, "article:") ||
				strings.HasPrefix(name, "profile:") ||
				strings.Contains(":description|keywords|author|theme-color|color-scheme|robots|viewport", name) {
				keep = true
			}
		case "link":
			rel := strings.ToLower(attrs["rel"])
			if slices.Contains([]string{"canonical", "icon", "shortcut icon", "apple-touch-icon", "manifest", "alternate", "author", "license"}, rel) {
				keep = true
			}
		}

		if keep {
			safe = append(safe, node)
		}
	}

	state.tagCache.Set(key, safe, time.Hour*6)
	return safe
}

func (state *State) handleRequest(w http.ResponseWriter, r *http.Request) {
	host := r.Host

	data := challenge.RequestDataFromContext(r.Context())

	lg := state.Logger(r)

	backend := state.GetBackend(host)
	if backend == nil {
		lg.Debug("no backend for host", "host", host)
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	getBackend := func() http.Handler {
		if opt := data.GetOpt(challenge.RequestOptBackendHost, ""); opt != "" && opt != host {
			b := state.GetBackend(host)
			if b == nil {
				http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
				// return empty backend
				return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
			}
			return b
		}
		return backend
	}

	cleanupRequest := func(r *http.Request, fromChallenge bool, ruleName string, ruleAction policy.RuleAction) {
		if fromChallenge {
			r.Header.Del("Referer")
		}
		q := r.URL.Query()

		if ref := q.Get(challenge.QueryArgReferer); ref != "" {
			r.Header.Set("Referer", ref)
		}

		// delete query parameters that were set by go-away
		for k := range q {
			if strings.HasPrefix(k, challenge.QueryArgPrefix) {
				q.Del(k)
			}
		}
		r.URL.RawQuery = q.Encode()

		data.ExtraHeaders.Set("X-Away-Rule", ruleName)
		data.ExtraHeaders.Set("X-Away-Action", string(ruleAction))

		// delete cookies set by go-away to prevent user tracking that way
		cookies := r.Cookies()
		r.Header.Del("Cookie")
		for _, c := range cookies {
			if !strings.HasPrefix(c.Name, utils.DefaultCookiePrefix) {
				r.AddCookie(c)
			}
		}

		// set response headers
		data.ResponseHeaders(w)
	}

	for _, rule := range state.rules {
		next, err := rule.Evaluate(lg, w, r, func() http.Handler {
			cleanupRequest(r, true, rule.Name, rule.Action)
			return getBackend()
		})
		if err != nil {
			state.ErrorPage(w, r, http.StatusInternalServerError, err, "")
			panic(err)
			return
		}

		if !next {
			return
		}
	}

	state.RuleHit(r, "DEFAULT", lg)
	data.State.ActionHit(r, policy.RuleActionPASS, lg)

	// default pass
	_, _ = action.Pass{}.Handle(lg, w, r, func() http.Handler {
		cleanupRequest(r, false, "DEFAULT", policy.RuleActionPASS)
		return getBackend()
	})
}

func (state *State) setupRoutes() error {

	state.Mux.HandleFunc("/", state.handleRequest)

	state.Mux.Handle("GET "+state.urlPath+"/assets/", http.StripPrefix(state.UrlPath()+"/assets/", gzipped.FileServer(gzipped.FS(embed.AssetsFs))))

	for _, reg := range state.challenges {

		if reg.Handler != nil {
			state.Mux.Handle(reg.Path+"/", reg.Handler)
		} else if reg.Verify != nil {
			// default verify
			state.Mux.HandleFunc(reg.Path+challenge.VerifyChallengeUrlSuffix, challenge.VerifyHandlerFunc(state, reg, nil, nil))
		}
	}

	return nil
}

func (state *State) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r, data := challenge.CreateRequestData(r, state)

	data.EvaluateChallenges(w, r)

	state.Mux.ServeHTTP(w, r)
}
