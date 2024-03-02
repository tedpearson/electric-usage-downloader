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

type OAuth struct {
	AuthorizationToken string `json:"authorizationToken"`
}

func Auth(config UtilityConfig) (string, error) {
	client := &http.Client{}
	postData := fmt.Sprintf("userId=%s&password=%s", config.Username, config.Password)
	authUrl := fmt.Sprintf("%s/services/oauth/auth/v2", config.ApiUrl)
	parsed, err := url.Parse(config.ApiUrl)
	authority := parsed.Hostname()
	req, err := http.NewRequest("POST", authUrl, strings.NewReader(postData))
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

func FetchData(start, end time.Time, config UtilityConfig, jwt string) (io.ReadCloser, error) {
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
