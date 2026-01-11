package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	userDataURL    = "https://www.breadfast.com/wp-json/breadfast/v3/user/data"
	cardLoginURL   = "https://card-panel.breadfast.tech/api/v1/mobile/wallet_users/login"
	cardBalanceURL = "https://card-panel.breadfast.tech/api/v1/mobile/wallet_users/getBalance"
	gameballURL    = "https://api.gameball.co/api/v3.0/integrations/player/%d"
)

var (
	passcreatorAPIKey = os.Getenv("PASSCREATOR_API_KEY")
	passcreatorPassID = os.Getenv("PASSCREATOR_PASS_ID")
	token             = os.Getenv("BREADFAST_TOKEN")
	gameballAPIKey    = os.Getenv("BREADFAST_GAMEBALL_API_KEY")
	mobileNumber      = os.Getenv("BREADFAST_CARD_MOBILE_NUMBER")
	passcode          = os.Getenv("BREADFAST_CARD_PASSCODE")
	deviceID          = os.Getenv("BREADFAST_CARD_DEVICE_ID")
	publicKey         = os.Getenv("BREADFAST_CARD_PUBLIC_KEY")
)

func main() {
	userName, _, points, err := fetchData()
	if err != nil {
		log.Fatal(err)
	}
	balance, err := fetchCardData()
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

func fetchCardData() (string, error) {
	client := retryablehttp.NewClient()
	encBody, err := func() (string, error) {
		payload, err := json.Marshal(map[string]interface{}{
			"mobile_number": mobileNumber,
			"mpin":          passcode,
			"scheme_id":     1,
			"device_info": map[string]string{
				"device_id": deviceID,
			},
		})
		if err != nil {
			return "", err
		}
		return func(payload []byte) (string, error) {
			block, _ := pem.Decode([]byte(strings.ReplaceAll(publicKey, "\\n", "\n")))
			pub, err := x509.ParsePKIXPublicKey(block.Bytes)
			if err != nil {
				return "", err
			}
			enc, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub.(*rsa.PublicKey), []byte(payload), nil)
			if err != nil {
				return "", err
			}
			return base64.StdEncoding.EncodeToString(enc), nil
		}(payload)
	}()
	if err != nil {
		return "", err
	}
	resBody, err := json.Marshal(map[string]string{"data": encBody})
	loginReq, err := retryablehttp.NewRequest(http.MethodPost, cardLoginURL, resBody)
	if err != nil {
		return "", err
	}
	loginRes, err := client.Do(loginReq)
	if err != nil {
		return "", err
	}
	defer loginRes.Body.Close()
	var loginResBody struct {
		Token   string  `json:"token"`
		Balance float64 `json:"current_balance"`
	}
	if err := json.NewDecoder(loginRes.Body).Decode(&loginResBody); err != nil {
		return "", err
	}

	balanceReq, err := retryablehttp.NewRequest(http.MethodPost, cardBalanceURL, nil)
	if err != nil {
		return "", err
	}
	balanceRes, err := client.Do(balanceReq)
	if err != nil {
		return "", err
	}
	defer balanceRes.Body.Close()
	var balanceResBody struct {
		Token   string  `json:"token"`
		Balance float64 `json:"current_balance"`
	}
	if err := json.NewDecoder(balanceRes.Body).Decode(&balanceResBody); err != nil {
		return "", err
	}

	balance := loginResBody.Balance
	if cardBalance := balanceResBody.Balance; cardBalance != 0 {
		balance = cardBalance
	}
	return fmt.Sprintf("%.2f", balance), nil
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
