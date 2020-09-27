package osuapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

type anyPost interface{}

// TokenResult contains user authentication stuff
type TokenResult struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Error        string `json:"error"`
}

// OsuRequestAuthURL returns the url users should be redirected to to init authentication
func (osuAPI *OsuAPI) OsuRequestAuthURL() (string, error) {
	base, err := url.Parse("https://osu.ppy.sh/oauth/authorize")
	if err != nil {
		return "", err
	}

	params := url.Values{}
	params.Add("client_id", strconv.Itoa(osuAPI.ClientID))
	params.Add("redirect_uri", osuAPI.RedirectURI)
	params.Add("response_type", "code")
	params.Add("scope", "public")
	params.Add("state", "")

	base.RawQuery = params.Encode()

	return base.String(), nil
}

func getToken(code anyPost) (*TokenResult, error) {
	buf, err := json.Marshal(code)
	if err != nil {
		return nil, fmt.Errorf("Error JSONifying object %v. %v", code, err)
	}

	resp, err := postRequestWithBody("https://osu.ppy.sh/oauth/token", bytes.NewBuffer(buf))
	if err != nil {
		return nil, fmt.Errorf("Error executing auth request. %v", err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading auth request. %v", err)
	}

	var token TokenResult
	err = json.Unmarshal(body, &token)
	if err != nil {
		return nil, fmt.Errorf("Error parsing auth request. %v", err)
	}

	if len(token.Error) > 0 {
		return &token, fmt.Errorf("Error with auth request. %v", token.Error)
	}

	if len(token.AccessToken) < 5 {
		return &token, fmt.Errorf("Broken token. %v", token.AccessToken)
	}

	return &token, nil
}

// GetToken converts a given code into a token
func (osuAPI *OsuAPI) GetToken(code string) (*TokenResult, error) {
	if len(code) <= 5 {
		return nil, fmt.Errorf("Code too short! " + code)
	}

	tokenPost := &tokenPost{
		ClientID:     osuAPI.ClientID,
		ClientSecret: osuAPI.ClientSecret,
		Code:         code,
		GrantType:    "authorization_code",
		RedirectURI:  osuAPI.RedirectURI}

	return getToken(tokenPost)
}

// RefreshToken turns a refresh token into a new set of tokens
func (osuAPI *OsuAPI) RefreshToken(token string) (*TokenResult, error) {
	if len(token) <= 5 {
		return nil, fmt.Errorf("Token too short! " + token)
	}
	// https://laravel.com/docs/master/passport#refreshing-tokens

	refreshPost := &refreshPost{
		ClientID:     osuAPI.ClientID,
		ClientSecret: osuAPI.ClientSecret,
		GrantType:    "refresh_token",
		Scope:        "public",
		RefreshToken: token}

	return getToken(refreshPost)
}
