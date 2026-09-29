package waf

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var matches = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "go-away_waf_matches",
	Help: "Coraza rule matches. disruptive is true only when an interruption is enforced",
}, []string{"rule_id", "disruptive"})

func RecordMatch(ruleID int, disruptive bool) {
	matches.WithLabelValues(strconv.Itoa(ruleID), strconv.FormatBool(disruptive)).Inc()
}

func ResetMetrics() {
	matches.Reset()
}
