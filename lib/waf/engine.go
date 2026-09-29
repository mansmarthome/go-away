package waf

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"

	"git.gammaspectra.live/git/go-away/lib/settings"
	"github.com/corazawaf/coraza-coreruleset/v4"
	"github.com/corazawaf/coraza/v3"
	"github.com/corazawaf/coraza/v3/experimental"
	"github.com/corazawaf/coraza/v3/types"
	"github.com/jcchavezs/mergefs"
	mergefsio "github.com/jcchavezs/mergefs/io"
)

var ErrRequestBody = errors.New("read request body")

type Holder interface {
	WAF() *Engine
}

type Engine struct {
	waf coraza.WAF
}

func Compile(cfg settings.WAF, onMatch func(types.MatchedRule)) (*Engine, error) {
	engine, err := ruleEngine(cfg.Engine)
	if err != nil {
		return nil, err
	}
	if err = validateDirectivesFile(cfg.DirectivesFile); err != nil {
		return nil, err
	}

	conf := coraza.NewWAFConfig().WithRootFS(mergefs.Merge(coreruleset.FS, mergefsio.OSFS))
	if onMatch != nil {
		conf = conf.WithErrorCallback(onMatch)
	}
	conf = conf.
		WithDirectives("Include @coraza.conf-recommended").
		WithDirectives(fmt.Sprintf("SecRuleEngine %s\nSecAuditEngine Off\nSecResponseBodyAccess Off\nSecAction \"id:11,phase:1,pass,nolog,setvar:tx.crs_skip_response_analysis=1\"", engine)).
		WithDirectives("Include @crs-setup.conf.example")
	if cfg.DirectivesFile != "" {
		conf = conf.WithDirectives(fmt.Sprintf("Include %q", cfg.DirectivesFile))
	}
	conf = conf.WithDirectives("Include @owasp_crs/*.conf")

	waf, err := coraza.NewWAF(conf)
	if err != nil {
		return nil, err
	}
	return &Engine{waf: waf}, nil
}

func New(directives string, onMatch func(types.MatchedRule)) (*Engine, error) {
	conf := coraza.NewWAFConfig().WithDirectives(directives)
	if onMatch != nil {
		conf = conf.WithErrorCallback(onMatch)
	}
	waf, err := coraza.NewWAF(conf)
	if err != nil {
		return nil, err
	}
	return &Engine{waf: waf}, nil
}

func (e *Engine) Close() error {
	if e == nil || e.waf == nil {
		return nil
	}
	closer, ok := e.waf.(experimental.WAFCloser)
	if !ok {
		return nil
	}
	return closer.Close()
}

func (e *Engine) Inspect(r *http.Request, id string, client netip.AddrPort) (*types.Interruption, error) {
	if e == nil || e.waf == nil {
		return nil, fmt.Errorf("waf engine is not configured")
	}
	tx := e.waf.NewTransactionWithID(id)
	defer func() {
		tx.ProcessLogging()
		_ = tx.Close()
	}()
	if tx.IsRuleEngineOff() {
		return nil, nil
	}
	return processRequest(tx, r, client)
}

func processRequest(tx types.Transaction, r *http.Request, client netip.AddrPort) (*types.Interruption, error) {
	tx.ProcessConnection(client.Addr().String(), int(client.Port()), "", 0)
	tx.ProcessURI(r.URL.String(), r.Method, r.Proto)
	for key, values := range r.Header {
		for _, value := range values {
			tx.AddRequestHeader(key, value)
		}
	}
	if r.Host != "" {
		tx.AddRequestHeader("Host", r.Host)
		tx.SetServerName(r.Host)
	}
	for _, te := range r.TransferEncoding {
		tx.AddRequestHeader("Transfer-Encoding", te)
	}

	if it := tx.ProcessRequestHeaders(); it != nil {
		return it, nil
	}

	if tx.IsRequestBodyAccessible() && r.Body != nil && r.Body != http.NoBody {
		it, _, err := tx.ReadRequestBodyFrom(r.Body)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrRequestBody, err)
		}
		if it != nil {
			return it, nil
		}
		reader, err := tx.RequestBodyReader()
		if err != nil {
			return nil, fmt.Errorf("request body reader: %w", err)
		}
		buffered, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("buffer request body: %w", err)
		}
		r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(buffered), r.Body))
	}

	it, err := tx.ProcessRequestBody()
	if err != nil {
		return nil, fmt.Errorf("process request body: %w", err)
	}
	return it, nil
}

func ruleEngine(engine string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "", "detection-only":
		return "DetectionOnly", nil
	case "on":
		return "On", nil
	default:
		return "", fmt.Errorf("waf engine %q must be detection-only or on", engine)
	}
}

func validateDirectivesFile(path string) error {
	if path == "" {
		return nil
	}
	if strings.ContainsAny(path, "\r\n") {
		return fmt.Errorf("waf directives-file contains a newline")
	}
	return nil
}
