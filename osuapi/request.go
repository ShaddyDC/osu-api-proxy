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

func apiRequest(reqType requestType, url string, body io.Reader, token *string) (*http.Response, error) {
	var requestTypeString string
	if reqType == postRequestVal {
		requestTypeString = "POST"
	} else {
		requestTypeString = "GET"
	}

	req, err := http.NewRequest(requestTypeString, url, body)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create request. %v", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	if token != nil {
		req.Header.Set("Authorization", "Bearer "+*token)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return resp, fmt.Errorf("Couldn't execute request with client. %v", err)
	}

	return resp, nil
}

func apiPostRequest(url string, body io.Reader, token *string) (*http.Response, error) {
	return apiRequest(postRequestVal, url, body, token)
}

func apiGetRequest(url string, body io.Reader, token *string) (*http.Response, error) {
	return apiRequest(getRequestVal, url, body, token)
}
