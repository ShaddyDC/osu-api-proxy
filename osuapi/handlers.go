package osuapi

import (
	"fmt"
	"strings"
)

// APIHandler is the interface for an api call function
type APIHandler interface {
	ServeAPI(*OsuAPI, string, string) (string, error)
}

// APIFunc is the type of an api call function
type APIFunc func(*OsuAPI, string, string) (string, error)

// ServeAPI makes APIFunc instances fulfil APIHandler interface capabilities
func (f APIFunc) ServeAPI(osuAPI *OsuAPI, api string, token string) (string, error) {
	return f(osuAPI, api, token)
}

func (api *OsuAPI) setupHandlers() {
	api.Handlers = make(map[string]APIFunc)
	api.Handlers["/api/user/"] = getUserFunc
}

func getUserFunc(osuAPI *OsuAPI, path string, token string) (string, error) {
	id := strings.TrimPrefix(path, "/api/user/")
	fmt.Println("Requesting user", id)

	user, err := osuAPI.GetUser(token, id)
	if err != nil {
		return "", fmt.Errorf("{Error:\"Error with api call: %v\"}", err)
	}
	return string(user), nil
}
