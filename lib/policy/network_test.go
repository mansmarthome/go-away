package policy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchPrefixesFollowsRedirect(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"prefixes":["10.0.0.0/8","2001:db8::/32"]}`))
	}))
	t.Cleanup(final.Close)

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL, http.StatusMovedPermanently)
	}))
	t.Cleanup(redirect.Close)

	url := redirect.URL
	jq := `.prefixes[]`
	n := Network{Url: &url, JqPath: &jq}
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	prefixes, err := n.FetchPrefixes(client)
	if err != nil {
		t.Fatal(err)
	}
	if len(prefixes) != 2 {
		t.Fatalf("got %d prefixes, want 2", len(prefixes))
	}
}
