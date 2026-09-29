package waf

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"

	"git.gammaspectra.live/git/go-away/lib/settings"
)

func TestInspectDenyAndDetectionOnly(t *testing.T) {
	on, err := New(`
SecRuleEngine On
SecRule ARGS:test "@contains evil" "id:1,phase:1,deny,status:403,log,msg:'evil'"
`, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = on.Close() })

	req := httptestRequest(http.MethodGet, "/?test=evil", nil)
	it, err := on.Inspect(req, "req-1", netip.MustParseAddrPort("203.0.113.5:443"))
	if err != nil {
		t.Fatal(err)
	}
	if it == nil || it.Status != http.StatusForbidden || it.RuleID != 1 {
		t.Fatalf("interruption = %#v", it)
	}

	detect, err := New(`
SecRuleEngine DetectionOnly
SecRule ARGS:test "@contains evil" "id:1,phase:1,deny,status:403,log,msg:'evil'"
`, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = detect.Close() })

	req = httptestRequest(http.MethodGet, "/?test=evil", nil)
	it, err = detect.Inspect(req, "req-2", netip.MustParseAddrPort("203.0.113.5:443"))
	if err != nil {
		t.Fatal(err)
	}
	if it != nil {
		t.Fatalf("detection-only interrupted: %#v", it)
	}
}

func TestInspectUsesContextAddress(t *testing.T) {
	engine, err := New(`
SecRuleEngine On
SecRule REMOTE_ADDR "@streq 2001:db8::1" "id:2,phase:1,deny,status:403,log,msg:'ipv6'"
`, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.Close() })

	req := httptestRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	it, err := engine.Inspect(req, "req-3", netip.MustParseAddrPort("[2001:db8::1]:443"))
	if err != nil {
		t.Fatal(err)
	}
	if it == nil || it.RuleID != 2 {
		t.Fatalf("expected ipv6 match, got %#v", it)
	}
}

func TestInspectRestoresBody(t *testing.T) {
	engine, err := New(`
SecRuleEngine On
SecRequestBodyAccess On
SecRule ARGS:name "@contains ok" "id:3,phase:2,pass,nolog"
`, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.Close() })

	req := httptestRequest(http.MethodPost, "/", strings.NewReader("name=ok"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	it, err := engine.Inspect(req, "req-4", netip.MustParseAddrPort("203.0.113.8:1"))
	if err != nil {
		t.Fatal(err)
	}
	if it != nil {
		t.Fatalf("unexpected interruption %#v", it)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "name=ok" {
		t.Fatalf("body = %q", body)
	}
}

func TestInspectBodyReadError(t *testing.T) {
	engine, err := New(`
SecRuleEngine On
SecRequestBodyAccess On
`, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.Close() })

	req := httptestRequest(http.MethodPost, "/", errReader{})
	_, err = engine.Inspect(req, "req-5", netip.MustParseAddrPort("203.0.113.9:1"))
	if err == nil {
		t.Fatal("expected body read error")
	}
}

func TestCompileRejectsEngine(t *testing.T) {
	_, err := Compile(settings.WAF{Engine: "off"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGhostDirectivesCompile(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	path, err := filepath.Abs("../../examples/waf-ghost.conf")
	if err != nil {
		t.Fatal(err)
	}
	engine, err := Compile(settings.WAF{Engine: "on", DirectivesFile: path}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = engine.Close()
}

func TestCompileCRS(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	engine, err := Compile(settings.WAF{Engine: "on"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.Close() })

	req := httptestRequest(http.MethodGet, "/?id=1'+OR+'1'%3D'1", nil)
	it, err := engine.Inspect(req, "req-crs", netip.MustParseAddrPort("203.0.113.10:443"))
	if err != nil {
		t.Fatal(err)
	}
	if it == nil {
		t.Fatal("expected CRS interruption")
	}
}

func httptestRequest(method, target string, body io.Reader) *http.Request {
	var r io.Reader = body
	if body == nil {
		r = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, target, r)
	if err != nil {
		panic(err)
	}
	if body == nil {
		req.Body = http.NoBody
	}
	return req
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("boom")
}
