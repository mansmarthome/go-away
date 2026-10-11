package policy

import (
	"git.gammaspectra.live/git/go-away/utils"
	"net/http"
)

type StateSettings struct {
	Cache           utils.Cache
	Backends        map[string]http.Handler
	PrivateKeySeed  []byte
	MainName        string
	MainVersion     string
	BasePath        string
	ClientIpHeader  string
	BackendIpHeader string

	ChallengeResponseCode int

	ASNDatabasePath string
}
