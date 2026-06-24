package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type ZohoServiceTest struct {
	clientID     string
	clientSecret string
	refreshToken string
	redirectURI  string
}

func (s *ZohoServiceTest) GetAccessToken() (string, error) {
	params := url.Values{}
	params.Add("refresh_token", s.refreshToken)
	params.Add("client_id", s.clientID)
	params.Add("client_secret", s.clientSecret)
	params.Add("redirect_uri", s.redirectURI)
	params.Add("grant_type", "refresh_token")

	resp, err := http.PostForm("https://accounts.zoho.com/oauth/v2/token", params)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("zoho oauth error: %s", result.Error)
	}
	return result.AccessToken, nil
}

func main() {
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	svc := &ZohoServiceTest{
		clientID:     os.Getenv("ZOHO_CLIENT_ID"),
		clientSecret: os.Getenv("ZOHO_CLIENT_SECRET"),
		refreshToken: os.Getenv("ZOHO_REFRESH_TOKEN"),
		redirectURI:  os.Getenv("ZOHO_REDIRECT_URI"),
	}

	token, err := svc.GetAccessToken()
	if err != nil {
		log.Fatalf("Error getting access token: %v", err)
	}
	fmt.Printf("Access Token: %s\n", token[:15]+"...")

	ticketID := "794157000007188574"
	orgID := "799550421" // from previous logs

	// Get Ticket Detail
	ticketURL := fmt.Sprintf("https://desk.zoho.com/api/v1/tickets/%s", ticketID)
	req, _ := http.NewRequest("GET", ticketURL, nil)
	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error calling tickets API: %v", err)
	}
	defer resp.Body.Close()

	ticketBytes, _ := io.ReadAll(resp.Body)
	var ticketData map[string]interface{}
	json.Unmarshal(ticketBytes, &ticketData)

	fmt.Println("--- Zoho Ticket Details ---")
	fmt.Printf("Subject: %v\n", ticketData["subject"])
	fmt.Printf("Status: %v\n", ticketData["status"])
	fmt.Printf("AssigneeId: %v\n", ticketData["assigneeId"])
	fmt.Printf("Channel: %v\n", ticketData["channel"])

	source, _ := ticketData["source"].(map[string]interface{})
	sourceJSON, _ := json.MarshalIndent(source, "", "  ")
	fmt.Printf("Source:\n%s\n", string(sourceJSON))

	// If source contains extId, try to fetch IM Session Details
	if source != nil && source["extId"] != nil {
		sessionID := source["extId"].(string)
		fmt.Printf("\n--- IM Session Details for Session ID: %s ---\n", sessionID)

		sessionURL := fmt.Sprintf("https://desk.zoho.com/api/v1/im/sessions/%s", sessionID)
		sReq, _ := http.NewRequest("GET", sessionURL, nil)
		sReq.Header.Set("Authorization", "Zoho-oauthtoken "+token)
		sReq.Header.Set("orgId", orgID)

		sResp, err := client.Do(sReq)
		if err != nil {
			fmt.Printf("Error fetching session: %v\n", err)
		} else {
			defer sResp.Body.Close()
			sBytes, _ := io.ReadAll(sResp.Body)
			var sData map[string]interface{}
			json.Unmarshal(sBytes, &sData)
			sJSON, _ := json.MarshalIndent(sData, "", "  ")
			fmt.Printf("IM Session API Response Status: %d\nResponse Body:\n%s\n", sResp.StatusCode, string(sJSON))
		}
	}
}
