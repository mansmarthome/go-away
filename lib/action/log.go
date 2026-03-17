package action

import (
	"git.gammaspectra.live/git/go-away/lib/challenge"
	"git.gammaspectra.live/git/go-away/lib/policy"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"log/slog"
	"net/http"
	"strings"
)

func init() {
	Register[policy.RuleActionLOG] = func(state challenge.StateInterface, ruleName, ruleHash string, settings ast.Node) (Handler, error) {
		params := LogDefaultSettings

		if settings != nil {
			ymlData, err := settings.MarshalYAML()
			if err != nil {
				return nil, err
			}
			err = yaml.Unmarshal(ymlData, &params)
			if err != nil {
				return nil, err
			}
		}

		// parse level (accepts "debug", "info", "warn", "error", and short forms)
		var lvl slog.Level
		switch strings.ToUpper(params.Level) {
		case "DEBUG", "DBG":
			lvl = slog.LevelDebug
		case "INFO", "INF":
			lvl = slog.LevelInfo
		case "WARN", "WARNING", "WRN":
			lvl = slog.LevelWarn
		case "ERROR", "ERR":
			lvl = slog.LevelError
		default:
			lvl = slog.LevelInfo
		}

		return Log{
			Level:   lvl,
			Message: params.Message,
		}, nil
	}
}

var LogDefaultSettings = LogSettings{
	Level:   "info",
	Message: "request logged",
}

type LogSettings struct {
	// Level can be: debug, info, warn, error (case-insensitive)
	Level string `yaml:"level"`

	// Custom message (defaults to "request logged")
	Message string `yaml:"message"`
}

type Log struct {
	Level   slog.Level
	Message string
}

func (a Log) Handle(logger *slog.Logger, w http.ResponseWriter, r *http.Request, done func() (backend http.Handler)) (next bool, err error) {
	// (rule name, rule hash, request id, etc. are already attached by the engine)
	logger.Log(r.Context(), a.Level, a.Message)

	// continue to the next handler / backend
	return true, nil
}
