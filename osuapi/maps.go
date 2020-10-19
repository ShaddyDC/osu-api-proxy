package osuapi

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

// GetMap fetches a map's osu file from the api
func (osuAPI *OsuAPI) GetMap(id string) ([]byte, error) {
	if !notRateLimited() {
		return nil, fmt.Errorf("Server rate limited")
	}

	req, err := http.NewRequest("GET", "https://osu.ppy.sh/osu/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create request. %v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Couldn't execute request with client. %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading request. %v", err)
	}

	return body, nil
}
