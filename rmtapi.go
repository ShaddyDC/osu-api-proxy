package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func rmtAPIRequest(url string, token string) (string, error) {
	fmt.Println("Fetching remote", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("couldn't create request. %v", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("couldn't execute request with client. %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading request. %v", err)
	}

	// Check if it's json containing an error
	var jsonError map[string]interface{}
	err = json.Unmarshal(body, &jsonError)
	rmtErr, hasKey := jsonError["error"]

	// If body doesn't contain json or no error
	if err != nil || (err == nil && !hasKey) {
		return string(body), nil
	}

	rmtErrDesc, _ := jsonError["error_description"]

	return string(body), fmt.Errorf(fmt.Sprintf("remote error %v, description: %v", rmtErr, rmtErrDesc))
}
