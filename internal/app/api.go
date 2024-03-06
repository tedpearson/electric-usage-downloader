package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuth holds a json web token parsed from the auth response.
type OAuth struct {
	AuthorizationToken string `json:"authorizationToken"`
}

// Auth authenticates with the api and returns a json web token for use with the api.
func Auth(config SmartHubConfig) (string, error) {
	client := &http.Client{}
	formData := url.Values{}
	formData.Set("userId", config.Username)
	formData.Set("password", config.Password)
	authUrl := fmt.Sprintf("%s/services/oauth/auth/v2", config.ApiUrl)
	parsed, err := url.Parse(config.ApiUrl)
	authority := parsed.Hostname()
	req, err := http.NewRequest("POST", authUrl, strings.NewReader(formData.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("authority", authority)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	decoder := json.NewDecoder(resp.Body)
	oauth := &OAuth{}
	err = decoder.Decode(oauth)
	if err != nil {
		return "", err
	}
	if oauth.AuthorizationToken == "" {
		return "", errors.New("auth response did not include auth token")
	}
	return oauth.AuthorizationToken, nil
}

// PollRequest is request information sent to the api to fetch data.
type PollRequest struct {
	TimeFrame       string   `json:"timeFrame"`
	UserId          string   `json:"userId"`
	Screen          string   `json:"screen"`
	IncludeDemand   bool     `json:"includeDemand"`
	ServiceLocation string   `json:"serviceLocationNumber"`
	Account         string   `json:"accountNumber"`
	Industries      []string `json:"industries"`
	StartDateTime   int64    `json:"startDateTime"`
	EndDateTime     int64    `json:"endDateTime"`
}

// FetchData calls the api to get data for a particular time period.
// Note that the api may return a PENDING status or actual data.
// However, parsing of the response is handled in ParseReader.
func FetchData(start, end time.Time, config SmartHubConfig, jwt string) (io.ReadCloser, error) {
	client := http.Client{}
	pollRequest := PollRequest{
		TimeFrame:       "HOURLY",
		UserId:          config.Username,
		Screen:          "USAGE_EXPLORER",
		IncludeDemand:   false,
		ServiceLocation: config.ServiceLocation,
		Account:         config.Account,
		Industries:      []string{"ELECTRIC"},
		StartDateTime:   start.UnixMilli(),
		EndDateTime:     end.UnixMilli(),
	}
	buffer := new(bytes.Buffer)
	err := json.NewEncoder(buffer).Encode(pollRequest)
	if err != nil {
		return nil, err
	}
	pollUrl := fmt.Sprintf("%s/services/secured/utility-usage/poll", config.ApiUrl)
	parsed, err := url.Parse(config.ApiUrl)
	authority := parsed.Hostname()
	req, err := http.NewRequest("POST", pollUrl, buffer)
	req.Header.Set("authority", authority)
	req.Header.Set("authorization", "Bearer "+jwt)
	req.Header.Set("x-nisc-smarthub-username", config.Username)
	req.Header.Set("content-type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
