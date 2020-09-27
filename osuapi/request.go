package osuapi

import (
	"fmt"
	"io"
	"net/http"
)

type requestType int

const (
	postRequestVal requestType = iota
	getRequestVal
)

func postRequestWithBody(url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create request. %v", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return resp, fmt.Errorf("Couldn't execute request with client. %v", err)
	}

	return resp, nil
}

func apiRequest(reqType requestType, url string, token string) (*http.Response, error) {
	if !notRateLimited() {
		return nil, fmt.Errorf("Server rate limited")
	}

	var requestTypeString string
	if reqType == postRequestVal {
		requestTypeString = "POST"
	} else {
		requestTypeString = "GET"
	}

	req, err := http.NewRequest(requestTypeString, url, nil)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create request. %v", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return resp, fmt.Errorf("Couldn't execute request with client. %v", err)
	}

	return resp, nil
}

func apiPostRequest(url string, token string) (*http.Response, error) {
	return apiRequest(postRequestVal, url, token)
}

func apiGetRequest(url string, token string) (*http.Response, error) {
	return apiRequest(getRequestVal, url, token)
}
