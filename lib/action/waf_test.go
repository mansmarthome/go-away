package action

import (
	"crypto/ed25519"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.gammaspectra.live/git/go-away/lib/challenge"
	"git.gammaspectra.live/git/go-away/lib/policy"
	"git.gammaspectra.live/git/go-away/lib/waf"
	"git.gammaspectra.live/git/go-away/utils"
	"github.com/google/cel-go/cel"
	"log/slog"
)

func TestWAFRequiresEnabled(t *testing.T) {
	_, err := Register[policy.RuleActionWAF](stubState{}, "coraza", "hash", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWAFBodyErrorDoesNotContinue(t *testing.T) {
	engine, err := waf.New(`
SecRuleEngine On
SecRequestBodyAccess On
`, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.Close() })

	state := stubState{engine: engine}
	handler, err := Register[policy.RuleActionWAF](state, "coraza", "hash", nil)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", errReader{})
	req, _ = challenge.CreateRequestData(req, state)
	rec := httptest.NewRecorder()
	called := false
	next, err := handler.Handle(slog.Default(), rec, req, func() http.Handler {
		called = true
		return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	})
	if err != nil {
		t.Fatal(err)
	}
	if next || called {
		t.Fatalf("next=%v called=%v", next, called)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("boom")
}

type stubState struct {
	engine *waf.Engine
}

func (s stubState) WAF() *waf.Engine { return s.engine }

func (stubState) RegisterCondition(string, ...string) (cel.Program, error) { return nil, nil }
func (stubState) Client() *http.Client                                     { return nil }
func (stubState) PrivateKeyFingerprint() []byte                            { return nil }
func (stubState) PrivateKey() ed25519.PrivateKey                           { return nil }
func (stubState) PublicKey() ed25519.PublicKey                             { return nil }
func (stubState) UrlPath() string                                          { return "" }
func (stubState) ChallengeFailed(*http.Request, *challenge.Registration, error, string, *slog.Logger) {
}
func (stubState) ChallengePassed(*http.Request, *challenge.Registration, string, *slog.Logger) {
}
func (stubState) ChallengeIssued(*http.Request, *challenge.Registration, string, *slog.Logger) {
}
func (stubState) ChallengeChecked(*http.Request, *challenge.Registration, string, *slog.Logger) {
}
func (stubState) RuleHit(*http.Request, string, *slog.Logger)              {}
func (stubState) RuleMiss(*http.Request, string, *slog.Logger)             {}
func (stubState) ActionHit(*http.Request, policy.RuleAction, *slog.Logger) {}
func (stubState) Logger(*http.Request) *slog.Logger                        { return slog.Default() }
func (stubState) ChallengePage(http.ResponseWriter, *http.Request, int, *challenge.Registration, map[string]any) {
}
func (stubState) ErrorPage(w http.ResponseWriter, r *http.Request, status int, err error, redirect string) {
	http.Error(w, err.Error(), status)
}
func (stubState) GetChallenge(challenge.Id) (*challenge.Registration, bool) { return nil, false }
func (stubState) GetChallengeByName(string) (*challenge.Registration, bool) { return nil, false }
func (stubState) GetChallenges() challenge.Register                         { return nil }
func (stubState) Settings() policy.StateSettings                            { return policy.StateSettings{} }
func (stubState) Strings() utils.Strings                                    { return nil }
func (stubState) GetBackend(string) http.Handler                            { return nil }
