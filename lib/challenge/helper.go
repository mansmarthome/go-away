package challenge

import (
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"git.gammaspectra.live/git/go-away/utils"
	"net/http"
	"net/url"
	"strings"
)

var ErrInvalidToken = errors.New("invalid token")
var ErrMismatchedToken = errors.New("mismatched token")
var ErrMismatchedTokenHappyEyeballs = errors.New("mismatched token: IPv4 to IPv6 upgrade detected, retrying")

func NewKeyVerifier() (verify VerifyFunc, issue func(key Key) string) {
	return func(key Key, token []byte, r *http.Request) (VerifyResult, error) {
			expectedKey, err := hex.DecodeString(string(token))
			if err != nil {
				return VerifyResultFail, err
			}
			if len(expectedKey) != KeySize {
				return VerifyResultFail, ErrInvalidToken
			}
			if subtle.ConstantTimeCompare(key[:], expectedKey) == 1 {
				return VerifyResultOK, nil
			}

			kk := Key(expectedKey)
			// IPv4 -> IPv6 Happy Eyeballs
			if key.Get(KeyFlagIsIPv4) == 0 && kk.Get(KeyFlagIsIPv4) > 0 {
				return VerifyResultOK, ErrMismatchedTokenHappyEyeballs
			}

			return VerifyResultFail, ErrMismatchedToken
		}, func(key Key) string {
			return hex.EncodeToString(key[:])
		}
}

const (
	QueryArgPrefix    = "__goaway"
	QueryArgReferer   = QueryArgPrefix + "_referer"
	QueryArgRedirect  = QueryArgPrefix + "_redirect"
	QueryArgRequestId = QueryArgPrefix + "_id"
	QueryArgChallenge = QueryArgPrefix + "_challenge"
	QueryArgToken     = QueryArgPrefix + "_token"
)

const MakeChallengeUrlSuffix = "/make-challenge"
const VerifyChallengeUrlSuffix = "/verify-challenge"

func GetVerifyInformation(r *http.Request, reg *Registration) (requestId RequestId, redirect, token string, err error) {

	q := r.URL.Query()

	if q.Get(QueryArgChallenge) != reg.Name {
		return RequestId{}, "", "", fmt.Errorf("unexpected challenge: got \"%s\"", q.Get(QueryArgChallenge))
	}

	requestIdHex := q.Get(QueryArgRequestId)

	if len(requestId) != hex.DecodedLen(len(requestIdHex)) {
		return RequestId{}, "", "", errors.New("invalid request id")
	}
	n, err := hex.Decode(requestId[:], []byte(requestIdHex))
	if err != nil {
		return RequestId{}, "", "", err
	} else if n != len(requestId) {
		return RequestId{}, "", "", errors.New("invalid request id")
	}

	token = q.Get(QueryArgToken)
	redirect, err = utils.EnsureNoOpenRedirect(q.Get(QueryArgRedirect))
	if err != nil {
		return RequestId{}, "", "", err
	}
	return
}

func VerifyUrl(r *http.Request, reg *Registration, token string) (*url.URL, error) {

	redirectUrl, err := RedirectUrl(r, reg)
	if err != nil {
		return nil, err
	}

	uri := new(url.URL)
	uri.Path = reg.Path + VerifyChallengeUrlSuffix

	data := RequestDataFromContext(r.Context())
	values := uri.Query()
	values.Set(QueryArgRequestId, data.Id.String())
	values.Set(QueryArgRedirect, redirectUrl.String())
	values.Set(QueryArgToken, token)
	values.Set(QueryArgChallenge, reg.Name)
	uri.RawQuery = values.Encode()

	return uri, nil
}

func RedirectUrl(r *http.Request, reg *Registration) (*url.URL, error) {
	uri, err := url.ParseRequestURI(r.URL.String())
	if err != nil {
		return nil, err
	}

	data := RequestDataFromContext(r.Context())
	values := uri.Query()
	values.Set(QueryArgRequestId, data.Id.String())
	if ref := r.Referer(); ref != "" {
		values.Set(QueryArgReferer, ref)
		values.Set("utm_referrer", ref)
	}
	values.Set(QueryArgChallenge, reg.Name)
	uri.RawQuery = values.Encode()

	return uri, nil
}

func VerifyHandlerChallengeResponseFunc(state StateInterface, data *RequestData, w http.ResponseWriter, r *http.Request, verifyResult VerifyResult, err error, redirect string) {
	if err != nil {
		// Happy Eyeballs! auto retry
		if errors.Is(err, ErrMismatchedTokenHappyEyeballs) {
			reqUri := *r.URL
			q := reqUri.Query()

			ref := q.Get(QueryArgReferer)
			// delete query parameters that were set by go-away
			for k := range q {
				if strings.HasPrefix(k, QueryArgPrefix) {
					q.Del(k)
				}
			}
			if ref != "" {
				q.Set(QueryArgReferer, ref)
			}
			reqUri.RawQuery = q.Encode()

			data.ResponseHeaders(w)

			http.Redirect(w, r, reqUri.String(), http.StatusTemporaryRedirect)
			return
		}
		state.ErrorPage(w, r, http.StatusBadRequest, err, redirect)
		return
	} else if !verifyResult.Ok() {
		state.ErrorPage(w, r, http.StatusForbidden, fmt.Errorf("access denied: failed challenge"), redirect)
		return
	}
	data.ResponseHeaders(w)
	http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
}

func VerifyHandlerFunc(state StateInterface, reg *Registration, verify VerifyFunc, responseFunc func(state StateInterface, data *RequestData, w http.ResponseWriter, r *http.Request, verifyResult VerifyResult, err error, redirect string)) http.HandlerFunc {
	if verify == nil {
		verify = reg.Verify
	}
	if responseFunc == nil {
		responseFunc = VerifyHandlerChallengeResponseFunc
	}
	return func(w http.ResponseWriter, r *http.Request) {
		data := RequestDataFromContext(r.Context())
		requestId, redirect, token, err := GetVerifyInformation(r, reg)
		if err != nil {
			state.ChallengeFailed(r, reg, err, "", nil)
			responseFunc(state, data, w, r, VerifyResultFail, fmt.Errorf("internal error: %w", err), "")
			return
		}
		data.Id = requestId

		err = func() (err error) {
			expiration := data.Expiration(reg.Duration)
			key := GetChallengeKeyForRequest(state, reg, expiration, r)

			verifyResult, err := verify(key, []byte(token), r)
			if err != nil {
				return err
			} else if !verifyResult.Ok() {
				state.ChallengeFailed(r, reg, nil, redirect, nil)
				responseFunc(state, data, w, r, verifyResult, nil, redirect)
				return nil
			}

			data.IssueChallengeToken(reg, key, []byte(token), expiration, true)
			data.ChallengeVerify[reg.id] = verifyResult
			state.ChallengePassed(r, reg, redirect, nil)

			responseFunc(state, data, w, r, verifyResult, nil, redirect)
			return nil
		}()
		if err != nil {
			state.ChallengeFailed(r, reg, err, redirect, nil)
			responseFunc(state, data, w, r, VerifyResultFail, fmt.Errorf("access denied: error in challenge %s: %w", reg.Name, err), redirect)
			return
		}
	}
}
