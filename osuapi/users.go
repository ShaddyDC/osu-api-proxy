package osuapi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"
)

// UserCompact has some basic user data
type UserCompact struct {
	AvatarURL     string    `json:"avatar_url"`
	CountryCode   string    `json:"country_code"`
	DefaultGroup  string    `json:"default_group"`
	ID            int64     `json:"id"`
	IsActive      bool      `json:"is_active"`
	IsBot         bool      `json:"is_bot"`
	IsOnline      bool      `json:"is_online"`
	IsSupporter   bool      `json:"is_supporter"`
	LastVisit     time.Time `json:"last_visit"`
	PmFriendsOnly bool      `json:"pm_friends_only"`
	ProfileColour string    `json:"profile_colour"`
	Username      string    `json:"username"`
	Error         string    `json:"error"`
}

// GetCurrentUserParsed returns info about the user the token belongs to
func (osuAPI *OsuAPI) GetCurrentUserParsed(token string) (*UserCompact, error) {
	body, err := osuAPI.GetCurrentUser(token)
	if err != nil {
		return nil, fmt.Errorf("Error with request. %v", err)
	}

	var user UserCompact
	err = json.Unmarshal(body, &user)
	if err != nil {
		return nil, fmt.Errorf("Error parsing user request. %v", err)
	}

	if len(user.Error) > 0 {
		return &user, fmt.Errorf("Error with request. %v", user.Error)
	}

	return &user, nil
}

// GetCurrentUser returns info about the user the token belongs to
func (osuAPI *OsuAPI) GetCurrentUser(token string) ([]byte, error) {
	resp, err := apiGetRequest("https://osu.ppy.sh/api/v2/me/osu", nil, &token)
	if err != nil {
		return nil, fmt.Errorf("Error getting user. %v", err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading request. %v", err)
	}

	return body, nil
}

// GetUser fetches a user's data from the api
func (osuAPI *OsuAPI) GetUser(token string, id string) ([]byte, error) {
	resp, err := apiGetRequest("https://osu.ppy.sh/api/v2/users/"+id+"/osu", nil, &token)
	if err != nil {
		return nil, fmt.Errorf("Error getting user. %v", err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading request. %v", err)
	}

	return body, nil
}
