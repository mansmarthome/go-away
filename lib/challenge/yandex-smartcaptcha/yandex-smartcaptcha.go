package yandex_smartcaptcha

import (
	_ "embed"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"git.gammaspectra.live/git/go-away/lib/challenge"
	"github.com/goccy/go-yaml/ast"
	"html/template"
	"io"
	"net/http"
	"github.com/goccy/go-yaml"
	"time"
)

func init() {
	challenge.Runtimes["yandex-smartcaptcha"] = FillRegistration
}

//go:embed yandex-smartcaptcha.mjs
var jsData []byte

var jsTemplate = template.Must(template.New("yandex-smartcaptcha.mjs").Parse(string(jsData)))

type Parameters struct {
	Sitekey   string `yaml:"sitekey"`
	Secretkey string `yaml:"secretkey"`
	Hl        string `yaml:"hl"`
}

var DefaultParameters = Parameters{
	Hl: "ru",
}

func FillRegistration(state challenge.StateInterface, reg *challenge.Registration, parameters ast.Node) error {
	params := DefaultParameters

	if parameters != nil {
		ymlData, err := parameters.MarshalYAML()
		if err != nil {
			return err
		}
		err = yaml.Unmarshal(ymlData, &params)
		if err != nil {
			return err
		}
	}

	if params.Sitekey == "" {
		return errors.New("sitekey not provided")
	}
	if params.Secretkey == "" {
		return errors.New("secretkey not provided")
	}

	reg.Class = challenge.ClassBlocking

	reg.Verify = func(key challenge.Key, token []byte, r *http.Request) (challenge.VerifyResult, error) {
		data := challenge.RequestDataFromContext(r.Context())
		url := fmt.Sprintf("https://smartcaptcha.yandexcloud.net/validate?secret=%s&token=%s&ip=%s",
			params.Secretkey, string(token), data.RemoteAddress.Addr().String())

		resp, err := state.Client().Get(url)
		if err != nil {
			return challenge.VerifyResultFail, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return challenge.VerifyResultFail, fmt.Errorf("validation failed with status %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return challenge.VerifyResultFail, err
		}

		var valResp struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(body, &valResp); err != nil {
			return challenge.VerifyResultFail, err
		}
		if valResp.Status == "ok" {
			return challenge.VerifyResultOK, nil
		}
		return challenge.VerifyResultFail, nil
	}

	reg.IssueChallenge = func(w http.ResponseWriter, r *http.Request, key challenge.Key, expiry time.Time) challenge.VerifyResult {
		data := challenge.RequestDataFromContext(r.Context())

		var buf bytes.Buffer
		if err := jsTemplate.Execute(&buf, map[string]any{
			"Sitekey":    params.Sitekey,
			"Hl":         params.Hl,
			"Challenge":  reg.Name,
			"VerifyPath": reg.Path + challenge.VerifyChallengeUrlSuffix,
			"Id":         data.Id.String(),
			"Strings":    data.State.Strings(),
		}); err != nil {
			return challenge.VerifyResultFail
		}

		state.ChallengePage(w, r, state.Settings().ChallengeResponseCode, reg, map[string]any{
			"HeaderTags": []template.HTML{
				template.HTML(`<script src="https://smartcaptcha.yandexcloud.net/captcha.js?render=onload&onload=yandexSmartCaptchaOnload" async defer></script>`),
			},
			"EndTags": []template.HTML{
				template.HTML(`<div id="yandex-smartcaptcha-container" style="margin: auto;"></div>`),
				template.HTML(`<script type="module">` + buf.String() + `</script>`),
			},
		})
		return challenge.VerifyResultNone
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+reg.Path+challenge.VerifyChallengeUrlSuffix, challenge.VerifyHandlerFunc(state, reg, nil, nil))
	reg.Handler = mux

	return nil
}
