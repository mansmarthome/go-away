package lib

import (
	"bytes"
	"git.gammaspectra.live/git/go-away/embed"
	"git.gammaspectra.live/git/go-away/lib/challenge"
	"git.gammaspectra.live/git/go-away/utils"
	"html/template"
	"maps"
	"net/http"
)

var templates map[string]*template.Template

func init() {

	templates = make(map[string]*template.Template)

	dir, err := embed.TemplatesFs.ReadDir(".")
	if err != nil {
		panic(err)
	}
	for _, e := range dir {
		if e.IsDir() {
			continue
		}
		data, err := embed.TemplatesFs.ReadFile(e.Name())
		if err != nil {
			panic(err)
		}
		err = initTemplate(e.Name(), string(data))
		if err != nil {
			panic(err)
		}
	}
}

func initTemplate(name, data string) error {
	tpl := template.New(name).Funcs(template.FuncMap{
		"attr": func(s string) template.HTMLAttr {
			return template.HTMLAttr(s)
		},
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
	})
	_, err := tpl.Parse(data)
	if err != nil {
		return err
	}
	templates[name] = tpl
	return nil
}

func (state *State) addCachedTags(data *challenge.RequestData, r *http.Request, input map[string]any) {
	proxyMeta := data.GetOptBool(challenge.RequestOptProxyMetaTags, false)
	proxyLink := data.GetOptBool(challenge.RequestOptProxySafeLinkTags, false)

	if !proxyMeta && !proxyLink {
		return
	}

	backend, host := data.BackendHost()
	tags := state.fetchTags(host, backend, r, proxyMeta, proxyLink)
	if len(tags) == 0 {
		return
	}

	// Split back into meta and link
	var metaTags, linkTags []map[string]string
	for _, n := range tags {
		m := make(map[string]string, len(n.Attr))
		for _, a := range n.Attr {
			m[a.Key] = a.Val
		}
		if n.Data == "meta" {
			metaTags = append(metaTags, m)
		} else {
			linkTags = append(linkTags, m)
		}
	}

	if len(metaTags) > 0 {
		if existing, ok := input["MetaTags"]; ok {
			input["MetaTags"] = append(existing.([]map[string]string), metaTags...)
		} else {
			input["MetaTags"] = metaTags
		}
	}
	if len(linkTags) > 0 {
		if existing, ok := input["LinkTags"]; ok {
			input["LinkTags"] = append(existing.([]map[string]string), linkTags...)
		} else {
			input["LinkTags"] = linkTags
		}
	}
}

func (state *State) ChallengePage(w http.ResponseWriter, r *http.Request, status int, reg *challenge.Registration, params map[string]any) {
	data := challenge.RequestDataFromContext(r.Context())
	input := make(map[string]any)
	input["Id"] = data.Id.String()
	input["Random"] = utils.CacheBust()

	input["Path"] = state.UrlPath()
	input["Links"] = state.opt.Links
	input["Strings"] = state.opt.Strings
	for k, v := range state.opt.ChallengeTemplateOverrides {
		input[k] = v
	}

	if reg != nil {
		input["Challenge"] = reg.Name
	}

	maps.Copy(input, params)

	if _, ok := input["Title"]; !ok {
		input["Title"] = state.opt.Strings.Get("title_challenge")
	}

	state.addCachedTags(data, r, input)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	buf := bytes.NewBuffer(make([]byte, 0, 8192))

	err := templates["challenge-"+state.opt.ChallengeTemplate+".gohtml"].Execute(buf, input)
	if err != nil {
		state.ErrorPage(w, r, http.StatusInternalServerError, err, "")
	} else {
		data.ResponseHeaders(w)
		w.WriteHeader(status)
		_, _ = w.Write(buf.Bytes())
	}
}

func (state *State) ErrorPage(w http.ResponseWriter, r *http.Request, status int, err error, redirect string) {
	data := challenge.RequestDataFromContext(r.Context())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	buf := bytes.NewBuffer(make([]byte, 0, 8192))

	input := map[string]any{
		"Id":        data.Id.String(),
		"Random":    utils.CacheBust(),
		"Error":     err.Error(),
		"Path":      state.UrlPath(),
		"Theme":     "",
		"Title":     template.HTML(string(state.opt.Strings.Get("title_error")) + " " + http.StatusText(status)),
		"Challenge": "",
		"Redirect":  redirect,
		"Links":     state.opt.Links,
		"Strings":   state.opt.Strings,
	}
	for k, v := range state.opt.ChallengeTemplateOverrides {
		input[k] = v
	}

	state.addCachedTags(data, r, input)

	err2 := templates["challenge-"+state.opt.ChallengeTemplate+".gohtml"].Execute(buf, input)
	if err2 != nil {
		// nested errors!
		panic(err2)
	} else {
		data.ResponseHeaders(w)
		w.WriteHeader(status)
		_, _ = w.Write(buf.Bytes())
	}
}
