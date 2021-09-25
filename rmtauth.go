package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
)

type tokenPost struct {
	ClientID     int    `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code"`
	GrantType    string `json:"grant_type"`
	RedirectURI  string `json:"redirect_uri"`
}

type refreshPost struct {
	ClientID     int    `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RefreshToken string `json:"refresh_token"`
	GrantType    string `json:"grant_type"`
	Scope        string `json:"scope"`
}

// TokenResult contains user authentication stuff
type TokenResult struct {
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func osuRequestAuthURL(cfg *osuAPIConfig) (string, error) {
	base, err := url.Parse("https://osu.ppy.sh/oauth/authorize")
	if err != nil {
		return "", err
	}

	params := url.Values{}
	params.Add("client_id", strconv.Itoa(cfg.ClientID))
	params.Add("redirect_uri", cfg.RedirectURI)
	params.Add("response_type", "code")
	params.Add("scope", "public")
	params.Add("state", "")

	base.RawQuery = params.Encode()

	return base.String(), nil
}

func getTokenImpl(code interface{}) (*TokenResult, error) {
	buf, err := json.Marshal(code)
	if err != nil {
		return nil, fmt.Errorf("error JSONifying object %v. %v", code, err)
	}

	resp, err := postRequestWithBody("https://osu.ppy.sh/oauth/token", bytes.NewBuffer(buf))
	if err != nil {
		return nil, fmt.Errorf("error executing auth request. %v", err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading auth request. %v", err)
	}

	var token TokenResult
	err = json.Unmarshal(body, &token)
	if err != nil {
		return nil, fmt.Errorf("error parsing auth request. %v", err)
	}

	if len(token.Error) > 0 {
		return &token, fmt.Errorf("error with auth request. %v, description: %v", token.Error, token.ErrorDescription)
	}

	if len(token.AccessToken) < 5 {
		return &token, fmt.Errorf("broken token. %v", token.AccessToken)
	}

	return &token, nil
}

func postRequestWithBody(url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("couldn't create request. %v", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return resp, fmt.Errorf("couldn't execute request with client. %v", err)
	}

	return resp, nil
}

func getNewToken(cfg *osuAPIConfig, code string) (*TokenResult, error) {
	if len(code) <= 5 {
		return nil, fmt.Errorf("Code too short! " + code)
	}

	tokenPost := &tokenPost{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Code:         code,
		GrantType:    "authorization_code",
		RedirectURI:  cfg.RedirectURI}

	return getTokenImpl(tokenPost)
}

func refreshToken(cfg *osuAPIConfig, token string) (*TokenResult, error) {
	if len(token) <= 5 {
		return nil, fmt.Errorf("Token too short! " + token)
	}
	// https://laravel.com/docs/master/passport#refreshing-tokens

	refreshPost := &refreshPost{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		GrantType:    "refresh_token",
		Scope:        "public",
		RefreshToken: token}

	return getTokenImpl(refreshPost)
}
