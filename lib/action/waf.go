package action

import (
	"errors"
	"fmt"
	"git.gammaspectra.live/git/go-away/lib/challenge"
	"git.gammaspectra.live/git/go-away/lib/policy"
	"git.gammaspectra.live/git/go-away/lib/waf"
	"github.com/corazawaf/coraza/v3/types"
	"github.com/goccy/go-yaml/ast"
	"log/slog"
	"net/http"
	"strings"
)

func init() {
	Register[policy.RuleActionWAF] = func(state challenge.StateInterface, ruleName, ruleHash string, settings ast.Node) (Handler, error) {
		holder, ok := state.(waf.Holder)
		if !ok || holder.WAF() == nil {
			return nil, fmt.Errorf("waf action requires waf.enabled")
		}
		return WAF{
			Engine:   holder.WAF(),
			RuleHash: ruleHash,
		}, nil
	}
}

type WAF struct {
	Engine   *waf.Engine
	RuleHash string
}

func (a WAF) Handle(logger *slog.Logger, w http.ResponseWriter, r *http.Request, done func() (backend http.Handler)) (next bool, err error) {
	data := challenge.RequestDataFromContext(r.Context())
	if data == nil {
		return false, fmt.Errorf("waf: missing request data")
	}

	it, err := a.Engine.Inspect(r, data.Id.String(), data.RemoteAddress)
	if err != nil {
		if errors.Is(err, waf.ErrRequestBody) {
			logger.Error("waf request body read failed", "err", err)
			data.State.ErrorPage(w, r, http.StatusBadRequest, fmt.Errorf("bad request: could not read request body %s/%s", data.Id.String(), a.RuleHash), "")
			return false, nil
		}
		logger.Error("waf inspect failed", "err", err)
		data.State.ErrorPage(w, r, http.StatusInternalServerError, fmt.Errorf("internal error: waf inspection failed %s/%s", data.Id.String(), a.RuleHash), "")
		return false, nil
	}
	if it == nil {
		return true, nil
	}

	waf.RecordMatch(it.RuleID, true)
	logger.Warn("waf interruption", "waf_rule_id", it.RuleID, "waf_action", it.Action, "waf_status", it.Status, "uri", r.URL.RequestURI())
	interrupt(data, a.RuleHash, logger, w, r, it)
	return false, nil
}

func interrupt(data *challenge.RequestData, ruleHash string, logger *slog.Logger, w http.ResponseWriter, r *http.Request, it *types.Interruption) {
	switch strings.ToLower(it.Action) {
	case "drop":
		logger.Info("request dropped")
		if hj, ok := w.(http.Hijacker); ok {
			if conn, _, err := hj.Hijack(); err == nil {
				_ = conn.Close()
				return
			}
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", "0")
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusForbidden)
	case "deny":
		status := it.Status
		if status == 0 {
			status = http.StatusForbidden
		}
		logger.Info("request denied")
		data.State.ErrorPage(w, r, status, fmt.Errorf("access denied: denied by administrative rule %s/%s", data.Id.String(), ruleHash), "")
	default:
		logger.Info("request denied")
		data.State.ErrorPage(w, r, http.StatusForbidden, fmt.Errorf("access denied: denied by administrative rule %s/%s", data.Id.String(), ruleHash), "")
	}
}
