package osuapi

import (
	"fmt"
	"io/ioutil"
)

// GetScore fetches a score's data from the api
func (osuAPI *OsuAPI) GetScore(token string, id string) ([]byte, error) {
	resp, err := apiGetRequest("https://osu.ppy.sh/api/v2/scores/osu/"+id+"/download", token)
	if err != nil {
		return nil, fmt.Errorf("Error getting score. %v", err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading request. %v", err)
	}

	return body, nil
}
