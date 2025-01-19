package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	userDataURL = "https://www.breadfast.com/wp-json/breadfast/v3/user/data"
	gameballURL = "https://api.gameball.co/api/v3.0/integrations/player/%d"
)

var (
	passcreatorAPIKey = os.Getenv("PASSCREATOR_API_KEY")
	passcreatorPassID = os.Getenv("PASSCREATOR_PASS_ID")
	token             = os.Getenv("BREADFAST_TOKEN")
	gameballAPIKey    = os.Getenv("BREADFAST_GAMEBALL_API_KEY")
)

func main() {
	userName, balance, points, err := fetchData()
	if err != nil {
		log.Fatal(err)
	}

	if err := updatePass(userName, balance, points); err != nil {
		log.Fatal(err)
	}
}

func fetchData() (string, string, int, error) {
	client := retryablehttp.NewClient()
	req, err := retryablehttp.NewRequest(http.MethodPost, userDataURL, nil)
	if err != nil {
		return "", "", 0, err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return "", "", 0, err
	}
	defer res.Body.Close()
	var userDataRes struct {
		Data struct {
			ID        int    `json:"id"`
			FirstName string `json:"fname"`
			LastName  string `json:"lname"`
			Balance   string `json:"balance"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&userDataRes); err != nil {
		return "", "", 0, err
	}

	req, err = retryablehttp.NewRequest(http.MethodGet, fmt.Sprintf(gameballURL, userDataRes.Data.ID), nil)
	if err != nil {
		return "", "", 0, err
	}
	req.Header.Add("APIKey", gameballAPIKey)
	res, err = client.Do(req)
	if err != nil {
		return "", "", 0, err
	}
	defer res.Body.Close()
	var gameballRes struct {
		Balance struct {
			Points int `json:"pointsBalance"`
		} `json:"balance"`
	}
	if err := json.NewDecoder(res.Body).Decode(&gameballRes); err != nil {
		return "", "", 0, err
	}

	return strings.TrimSpace(userDataRes.Data.FirstName + " " + userDataRes.Data.LastName), userDataRes.Data.Balance, gameballRes.Balance.Points, nil
}

func updatePass(userName string, balance string, points int) error {
	var reqBody bytes.Buffer
	if err := json.NewEncoder(&reqBody).Encode(map[string]interface{}{
		"678d41c3335b08.42917475": userName,
		"678d41c3335a66.22982034": balance,
		"678d3e8a84da01.54533543": points,
	}); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("https://app.passcreator.com/api/pass/%s?zapierStyle=true", passcreatorPassID), &reqBody)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", passcreatorAPIKey)

	_, err = http.DefaultClient.Do(req)
	return err
}
