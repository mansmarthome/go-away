package utils

import (
	"mime"
	"net/http"
	"net/http/httptest"
	"golang.org/x/net/html"
)

// FetchTags fetches <meta> and/or <link> tags from the backend for the given request.
// It performs an internal GET request that mimics a real browser/social crawler.
func FetchTags(backend http.Handler, r *http.Request, wantMeta, wantLink bool) (metaTags, linkTags []html.Node) {
	// Clean URL – remove all __goaway_* parameters
	uri := *r.URL
	q := uri.Query()
	for k := range q {
		if len(k) > 8 && k[:8] == "__goaway" {
			q.Del(k)
		}
	}
	uri.RawQuery = q.Encode()

	// Use httptest recorder to call the backend internally
	w := httptest.NewRecorder()

	// Clone the original request – this is the key fix!
	fetchReq := r.Clone(r.Context())
	fetchReq.Method = http.MethodGet
	fetchReq.URL = &uri
	fetchReq.RequestURI = ""
	fetchReq.Close = true

	// Make it look like a real social media crawler (helps with dynamic backends)
	fetchReq.Header = http.Header{
		"User-Agent": []string{"Mozilla/5.0 (compatible; go-away/1.0 +https://git.gammaspectra.live/git/go-away)"},
		"Accept":     []string{"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
		"Accept-Language": []string{"en-US,en;q=0.9"},
	}
	// Keep original Host header – crucial for virtual hosting
	if r.Host != "" {
		fetchReq.Host = r.Host
	}

	backend.ServeHTTP(w, fetchReq)
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	ct, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if ct != "text/html" && ct != "application/xhtml+xml" {
		return nil, nil
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, nil
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if wantMeta && n.Data == "meta" {
				metaTags = append(metaTags, html.Node{
					Type:     html.ElementNode,
					Data:     n.Data,
					DataAtom: n.DataAtom,
					Attr:     n.Attr,
				})
			}
			if wantLink && n.Data == "link" {
				linkTags = append(linkTags, html.Node{
					Type:     html.ElementNode,
					Data:     n.Data,
					DataAtom: n.DataAtom,
					Attr:     n.Attr,
				})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return metaTags, linkTags
}
