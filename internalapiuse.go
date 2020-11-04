package main

import (
	"encoding/json"
	"fmt"
	"time"
)

type userCompact struct {
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

func getCurrentUser(token string) (*userCompact, error) {
	body, err := rmtAPIRequest("https://osu.ppy.sh/api/v2/me/osu", token)
	if err != nil {
		return nil, fmt.Errorf("Error with request. %v", err)
	}

	var user userCompact
	err = json.Unmarshal([]byte(body), &user)
	if err != nil {
		return nil, fmt.Errorf("Error parsing user request. %v", err)
	}

	if len(user.Error) > 0 {
		return &user, fmt.Errorf("Error with request. %v", user.Error)
	}

	return &user, nil
}
