package lib

import (
	"git.gammaspectra.live/git/go-away/lib/policy"
	"git.gammaspectra.live/git/go-away/lib/waf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type stateMetrics struct {
	rules      *prometheus.CounterVec
	actions    *prometheus.CounterVec
	challenges *prometheus.CounterVec
}

func newMetrics() *stateMetrics {
	return &stateMetrics{
		rules: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "go-away_rule_results",
			Help: "The number of rule hits or misses",
		}, []string{"rule", "result"}),
		actions: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "go-away_action_results",
			Help: "The number of each action issued",
		}, []string{"action"}),
		challenges: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "go-away_challenge_results",
			Help: "The number of challenges issued, passed or explicitly failed",
		}, []string{"challenge", "action"}),
	}
}

func (metrics *stateMetrics) Rule(name, result string) {
	metrics.rules.With(prometheus.Labels{"rule": name, "result": result}).Inc()
}

func (metrics *stateMetrics) Action(action policy.RuleAction) {
	metrics.actions.With(prometheus.Labels{"action": string(action)}).Inc()
}

func (metrics *stateMetrics) Challenge(name, result string) {
	metrics.challenges.With(prometheus.Labels{"challenge": name, "action": result}).Inc()
}

func (metrics *stateMetrics) Reset() {
	metrics.rules.Reset()
	metrics.actions.Reset()
	metrics.challenges.Reset()
	waf.ResetMetrics()
}

var metrics = newMetrics()
