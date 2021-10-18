package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

var (
	membershipNum     = os.Getenv("MEMBERSHIP_NUM")
	membershipPin     = os.Getenv("MEMBERSHIP_PIN")
	passcreatorAPIKey = os.Getenv("PASSCREATOR_API_KEY")
	passcreatorPassID = os.Getenv("PASSCREATOR_PASS_ID")

	membershipTiers = map[string]string{
		"":     "Unknown",
		"BLUE": "Blue",
		"SILV": "Silver",
		"GOLD": "Gold",
		"ELIT": "Elite",
		"PLAT": "Platinum",
	}
)

func main() {
	reqBody := url.Values{}
	reqBody.Add("countrySelect", "EG")
	reqBody.Add("txtUser", membershipNum)
	reqBody.Add("txtPass", membershipPin)
	reqBody.Add("clickedButton", "Login")
	res, err := http.Post("https://www.egyptairplus.com/StandardWebsite/Login.jsp", "application/x-www-form-urlencoded", strings.NewReader(reqBody.Encode()))
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	membershipName, membershipTier, membershipMiles, err := parseCard(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	if err := updatePass(membershipName, membershipTier, membershipMiles); err != nil {
		log.Fatal(err)
	}
}

func parseCard(r io.Reader) (string, string, int, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", "", 0, err
	}

	readAttr := func(n *html.Node, attrName string) string {
		for _, attr := range n.Attr {
			if attr.Key == attrName {
				return attr.Val
			}
		}
		return ""
	}

	var findByClassName func(n *html.Node, className string) *html.Node
	findByClassName = func(n *html.Node, className string) *html.Node {
		if n.Type == html.ElementNode && readAttr(n, "class") == className {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if x := findByClassName(c, className); x != nil {
				return x
			}
		}
		return nil
	}

	loginContentElm := findByClassName(doc, "LoginContent")

	loginNameElm := findByClassName(loginContentElm, "LoginName")
	loginName := strings.Title(strings.ToLower(loginNameElm.FirstChild.Data))

	loginDetailsElm := findByClassName(loginContentElm, "LoginDetails")
	loginDetailsTier := parseCardTier(strings.Fields(loginDetailsElm.FirstChild.NextSibling.NextSibling.Data)[1])

	loginAwdElm := findByClassName(loginContentElm, "LoginAwd")
	loginAwd, _ := strconv.Atoi(strings.ReplaceAll(strings.Fields(loginAwdElm.FirstChild.Data)[3], ",", ""))

	return loginName, loginDetailsTier, loginAwd, nil
}

func parseCardTier(tier string) string {
	if t, ok := membershipTiers[tier]; ok {
		return t
	}
	return membershipTiers[""]
}

func updatePass(membershipName, membershipTier string, membershipMiles int) error {
	var reqBody bytes.Buffer
	if err := json.NewEncoder(&reqBody).Encode(map[string]interface{}{
		"secondaryFields_0_Name":  membershipName,
		"primaryFields_0_Tier":    membershipTier,
		"secondaryFields_1_Miles": membershipMiles,
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
