package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	loginURL      = "https://www.egyptairplus.com/api/login"
	membershipURL = "https://www.egyptairplus.com/api/memberships/me"
)

var (
	passcreatorAPIKey = os.Getenv("PASSCREATOR_API_KEY")
	passcreatorPassID = os.Getenv("PASSCREATOR_PASS_ID")
	membershipNum     = os.Getenv("MEMBERSHIP_NUM")
	membershipPin     = os.Getenv("MEMBERSHIP_PIN")

	membershipTiers = map[string][2]string{
		"":     [...]string{"Unknown", "ffffff"},
		"BLUE": [...]string{"Blue", "0090d6"},
		"SILV": [...]string{"Silver", "939698"},
		"GOLD": [...]string{"Gold", "c4a55e"},
		"ELIT": [...]string{"Elite", "913531"},
		"PLAT": [...]string{"Platinum", "373636"},
	}
)

func main() {
	httpClient := retryablehttp.NewClient()
	httpClient.HTTPClient.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	b, err := json.Marshal(map[string]string{
		"userName": membershipNum,
		"password": membershipPin,
	})
	if err != nil {
		log.Fatal(err)
	}
	res, err := httpClient.Post(loginURL, "application/json", bytes.NewReader(b))
	if err != nil {
		log.Fatal(err)
	}
	var accessToken string
	for _, c := range res.Cookies() {
		if c.Name != "accessToken" {
			continue
		}
		accessToken = c.Value
		break
	}

	req, err := retryablehttp.NewRequest(http.MethodGet, membershipURL, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: "accessToken", Value: accessToken})
	res, err = httpClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	membershipName, membershipTier, membershipTierColor, membershipMiles, err := parseCard(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	if err := updatePass(membershipName, membershipTier, membershipTierColor, membershipMiles); err != nil {
		log.Fatal(err)
	}
}

func parseCard(r io.Reader) (string, string, string, int, error) {
	var doc struct {
		Data struct {
			Individual struct {
				FulfillmentDetail struct {
					NameOnCard string `json:"nameOnCard"`
				} `json:"fulfillmentDetail"`
			} `json:"individual"`
			MainTier struct {
				AllianceTier struct {
					FfpTierCode string `json:"ffpTierCode"`
				} `json:"allianceTier"`
			} `json:"mainTier"`
			LoyaltyAward []struct {
				Code   string `json:"code"`
				Amount string `json:"amount"`
			} `json:"loyaltyAward"`
		} `json:"data"`
	}
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return "", "", "", 0, err
	}

	loginName := doc.Data.Individual.FulfillmentDetail.NameOnCard

	loginDetailsTier, loginDetailsTierColor := parseCardTier(doc.Data.MainTier.AllianceTier.FfpTierCode)

	var loginAwd int
	for _, award := range doc.Data.LoyaltyAward {
		if award.Code != "AWM" {
			continue
		}
		var err error
		loginAwd, err = strconv.Atoi(strings.TrimSpace(award.Amount))
		if err != nil {
			return "", "", "", 0, err
		}
	}

	return loginName, loginDetailsTier, loginDetailsTierColor, loginAwd, nil
}

func parseCardTier(tier string) (string, string) {
	if t, ok := membershipTiers[tier]; ok {
		return t[0], t[1]
	}
	return membershipTiers[""][0], membershipTiers[""][1]
}

func updatePass(membershipName, membershipTier, membershipTierColor string, membershipMiles int) error {
	var reqBody bytes.Buffer
	if err := json.NewEncoder(&reqBody).Encode(map[string]interface{}{
		"secondaryFields_0_Name":  membershipName,
		"primaryFields_0_Tier":    membershipTier,
		"secondaryFields_1_Miles": membershipMiles,
		"backgroundColor":         membershipTierColor,
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
