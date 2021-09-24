package main

import (
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

	return string(body), nil
}
