package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type AgentInfo struct {
	Name  string
	Email string
}

type ZohoService struct {
	clientID     string
	clientSecret string
	refreshToken string
	redirectURI  string
	accessToken  string
	expiryTime   time.Time
	orgID        string
	agentCache   map[string]AgentInfo
	mu           sync.RWMutex
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func looksLikePhone(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}

	digits := 0
	for _, r := range trimmed {
		if r >= '0' && r <= '9' {
			digits++
			continue
		}
		if !strings.ContainsRune("+ -().", r) {
			return false
		}
	}
	return digits >= 7
}

func cleanContactName(values ...string) string {
	name := firstNonEmpty(values...)
	if looksLikePhone(name) || name == "." || name == "-" {
		return ""
	}
	return name
}

func NewZohoService() *ZohoService {
	s := &ZohoService{
		clientID:     os.Getenv("ZOHO_CLIENT_ID"),
		clientSecret: os.Getenv("ZOHO_CLIENT_SECRET"),
		refreshToken: os.Getenv("ZOHO_REFRESH_TOKEN"),
		redirectURI:  os.Getenv("ZOHO_REDIRECT_URI"),
		agentCache:   make(map[string]AgentInfo),
	}
	return s
}

// GetAccessToken returns a valid access token, auto-refreshing if expired
func (s *ZohoService) GetAccessToken() (string, error) {
	s.mu.RLock()
	// If token exists and has more than 2 minutes of validity, return it
	if s.accessToken != "" && time.Now().Add(2*time.Minute).Before(s.expiryTime) {
		token := s.accessToken
		s.mu.RUnlock()
		return token, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check inside lock to prevent parallel refreshes
	if s.accessToken != "" && time.Now().Add(2*time.Minute).Before(s.expiryTime) {
		return s.accessToken, nil
	}

	log.Println("[ZohoService] Access Token is expired or missing. Refreshing...")

	params := url.Values{}
	params.Add("refresh_token", s.refreshToken)
	params.Add("client_id", s.clientID)
	params.Add("client_secret", s.clientSecret)
	params.Add("redirect_uri", s.redirectURI)
	params.Add("grant_type", "refresh_token")

	resp, err := http.PostForm("https://accounts.zoho.com/oauth/v2/token", params)
	if err != nil {
		return "", fmt.Errorf("failed to call Zoho oauth: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("zoho oauth returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode oauth response: %w", err)
	}

	if result.Error != "" {
		return "", fmt.Errorf("zoho oauth error: %s", result.Error)
	}

	s.accessToken = result.AccessToken
	// Deduct a tiny buffer (10s) just in case
	s.expiryTime = time.Now().Add(time.Duration(result.ExpiresIn-10) * time.Second)

	log.Println("[ZohoService] Access Token successfully refreshed.")

	// Fetch Org ID on first successful refresh if not present
	if s.orgID == "" {
		go func() {
			if err := s.fetchOrgID(); err != nil {
				log.Printf("[ZohoService] Failed to pre-fetch Organization ID: %v", err)
			}
		}()
	}

	return s.accessToken, nil
}

func (s *ZohoService) fetchOrgID() error {
	token, err := s.GetAccessToken()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", "https://desk.zoho.com/api/v1/organizations", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to get organizations: status %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Printf("[ZohoService] fetchOrgID Raw Response: %s\n", string(body))

	var orgsResponse struct {
		Data []struct {
			ID json.Number `json:"id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &orgsResponse); err != nil {
		return err
	}

	if len(orgsResponse.Data) > 0 {
		s.mu.Lock()
		s.orgID = orgsResponse.Data[0].ID.String()
		s.mu.Unlock()
		log.Printf("[ZohoService] Retrieved and set Org ID: %s", s.orgID)
	}

	return nil
}

func (s *ZohoService) getOrgID() (string, error) {
	s.mu.RLock()
	org := s.orgID
	s.mu.RUnlock()

	if org != "" {
		return org, nil
	}

	err := s.fetchOrgID()
	if err != nil {
		return "", err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.orgID, nil
}

type ZohoTicketContact struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Mobile    string `json:"mobile"`
	Phone     string `json:"phone"`
}

// ZohoTicket represents a ticket item mapped from Zoho Desk API
type ZohoTicket struct {
	ID           string    `json:"id"`
	TicketNumber string    `json:"ticketNumber"`
	Subject      string    `json:"subject"`
	Status       string    `json:"status"`
	StatusType   string    `json:"statusType"` // open, closed, etc.
	CreatedTime  time.Time `json:"createdTime"`
	ModifiedTime time.Time `json:"modifiedTime"`
	ContactID    string    `json:"contactId"`
	ContactName  string    `json:"contactName"`
	AssigneeID   string    `json:"assigneeId"`
	DepartmentID string    `json:"departmentId"`
	Priority     string    `json:"priority"`
	Channel      string    `json:"channel"`
	Category     string    `json:"category"`
	Description  string    `json:"description"`
	Phone        string    `json:"phone,omitempty"`
	Email        string    `json:"email,omitempty"`
	Sentiment    string    `json:"sentiment,omitempty"`
	CustomerTone string    `json:"customerTone,omitempty"`
	IsEscalated  bool      `json:"isEscalated,omitempty"`
	WebURL       string    `json:"webUrl,omitempty"`
	ContactInfo  *ZohoTicketContact `json:"contact,omitempty"`
	// Contact details fetched separately
	ContactPhone string `json:"-"`
	ContactEmail string `json:"-"`
	// Assignee/owner details fetched separately
	AssigneeName  string `json:"-"`
	AssigneeEmail string `json:"-"`
}

type ZohoTicketStatus struct {
	Value      string `json:"value"`
	Label      string `json:"label"`
	StatusType string `json:"status_type,omitempty"`
}

func uniqueTicketStatuses(tickets []ZohoTicket) []ZohoTicketStatus {
	statuses := make([]ZohoTicketStatus, 0)
	seen := make(map[string]bool)
	for _, ticket := range tickets {
		status := strings.TrimSpace(ticket.Status)
		if status == "" || seen[strings.ToLower(status)] {
			continue
		}
		seen[strings.ToLower(status)] = true
		statuses = append(statuses, ZohoTicketStatus{
			Value:      status,
			Label:      status,
			StatusType: ticket.StatusType,
		})
	}
	return statuses
}

func defaultTicketStatuses() []ZohoTicketStatus {
	return []ZohoTicketStatus{
		{Value: "Open", Label: "Open", StatusType: "Open"},
		{Value: "OnHold", Label: "On Hold", StatusType: "OnHold"},
		{Value: "Escalated", Label: "Escalated", StatusType: "Open"},
		{Value: "Closed", Label: "Closed", StatusType: "Closed"},
	}
}

func mergeTicketStatuses(groups ...[]ZohoTicketStatus) []ZohoTicketStatus {
	statuses := make([]ZohoTicketStatus, 0)
	seen := make(map[string]bool)
	for _, group := range groups {
		for _, status := range group {
			value := strings.TrimSpace(status.Value)
			if value == "" {
				continue
			}
			key := strings.ToLower(value)
			if seen[key] {
				continue
			}
			seen[key] = true
			if strings.TrimSpace(status.Label) == "" {
				status.Label = value
			}
			status.Value = value
			statuses = append(statuses, status)
		}
	}
	return statuses
}

func addAllowedStatus(statuses *[]ZohoTicketStatus, seen map[string]bool, raw interface{}) {
	var value, label, statusType string

	switch item := raw.(type) {
	case string:
		value = strings.TrimSpace(item)
		label = value
	case map[string]interface{}:
		for _, key := range []string{"value", "name", "apiName", "status", "statusName"} {
			if v, ok := item[key].(string); ok && strings.TrimSpace(v) != "" {
				value = strings.TrimSpace(v)
				break
			}
		}
		for _, key := range []string{"displayValue", "displayLabel", "label", "name", "status"} {
			if v, ok := item[key].(string); ok && strings.TrimSpace(v) != "" {
				label = strings.TrimSpace(v)
				break
			}
		}
		for _, key := range []string{"type", "statusType"} {
			if v, ok := item[key].(string); ok && strings.TrimSpace(v) != "" {
				statusType = strings.TrimSpace(v)
				break
			}
		}
	default:
		return
	}

	if value == "" {
		return
	}
	if label == "" {
		label = value
	}
	key := strings.ToLower(value)
	if seen[key] {
		return
	}
	seen[key] = true
	*statuses = append(*statuses, ZohoTicketStatus{Value: value, Label: label, StatusType: statusType})
}

// ListTicketStatuses retrieves the status picklist configured in Zoho Desk.
// If the token lacks Desk.fields.READ, it falls back to statuses observed in recent tickets.
func (s *ZohoService) ListTicketStatuses() ([]ZohoTicketStatus, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}

	orgID, err := s.getOrgID()
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	for _, urlStr := range []string{
		"https://desk.zoho.com/api/v1/fields?module=tickets",
		"https://desk.zoho.com/api/v1/organizationFields?module=tickets",
	} {
		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
		req.Header.Set("orgId", orgID)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("[ZohoService] Could not read ticket status field from %s: status %d", urlStr, resp.StatusCode)
			continue
		}

		var fieldsResponse struct {
			Data []map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(body, &fieldsResponse); err != nil {
			continue
		}

		for _, field := range fieldsResponse.Data {
			apiName, _ := field["apiName"].(string)
			displayLabel, _ := field["displayLabel"].(string)
			if !strings.EqualFold(apiName, "status") && !strings.EqualFold(displayLabel, "status") {
				continue
			}

			statuses := make([]ZohoTicketStatus, 0)
			seen := make(map[string]bool)
			if allowedValues, ok := field["allowedValues"].([]interface{}); ok {
				for _, allowed := range allowedValues {
					addAllowedStatus(&statuses, seen, allowed)
				}
			}
			if len(statuses) > 0 {
				return statuses, nil
			}
		}
	}

	tickets, err := s.ListTickets("")
	if err != nil {
		return nil, err
	}
	return mergeTicketStatuses(defaultTicketStatuses(), uniqueTicketStatuses(tickets)), nil
}

// ListTickets retrieves the active tickets list from Zoho Desk API
func (s *ZohoService) ListTickets(assigneeID string) ([]ZohoTicket, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}

	orgID, err := s.getOrgID()
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("sortBy", "-modifiedTime")
	params.Set("limit", "50")
	params.Set("include", "contacts")
	if assigneeID != "" {
		params.Set("assigneeId", assigneeID)
	}

	req, err := http.NewRequest("GET", "https://desk.zoho.com/api/v1/tickets?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list tickets failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fmt.Printf("[ZohoService] ListTickets Raw JSON Snippet: %s\n", string(body))

	var ticketsResponse struct {
		Data []ZohoTicket `json:"data"`
	}

	if err := json.Unmarshal(body, &ticketsResponse); err != nil {
		return nil, err
	}

	// Warm up agent cache if empty to avoid multiple single GetAgent calls
	s.mu.RLock()
	cacheLen := len(s.agentCache)
	s.mu.RUnlock()
	if cacheLen == 0 {
		if _, err := s.ListAgents(); err != nil {
			log.Printf("[ZohoService] Failed to warm up agent cache: %v", err)
		}
	}

	for i := range ticketsResponse.Data {
		t := &ticketsResponse.Data[i]
		if t.AssigneeID != "" {
			if agentInfo, err := s.GetAgent(t.AssigneeID); err == nil {
				t.AssigneeName = agentInfo.Name
				t.AssigneeEmail = agentInfo.Email
			}
		}
	}

	return ticketsResponse.Data, nil
}

// GetTicketDetail retrieves detailed single ticket info including details
func (s *ZohoService) GetTicketDetail(ticketID string) (*ZohoTicket, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}

	orgID, err := s.getOrgID()
	if err != nil {
		return nil, err
	}

	urlStr := fmt.Sprintf("https://desk.zoho.com/api/v1/tickets/%s", ticketID)
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get ticket details failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var ticket ZohoTicket
	if err := json.Unmarshal(body, &ticket); err != nil {
		return nil, err
	}

	if ticket.ContactID != "" {
		contactURL := fmt.Sprintf("https://desk.zoho.com/api/v1/contacts/%s", ticket.ContactID)
		cReq, err := http.NewRequest("GET", contactURL, nil)
		if err == nil {
			cReq.Header.Set("Authorization", "Zoho-oauthtoken "+token)
			cReq.Header.Set("orgId", orgID)
			cResp, cErr := client.Do(cReq)
			if cErr == nil && cResp.StatusCode == http.StatusOK {
				var contact struct {
					FirstName      string `json:"firstName"`
					LastName       string `json:"lastName"`
					FullName       string `json:"fullName"`
					Phone          string `json:"phone"`
					Mobile         string `json:"mobile"`
					Email          string `json:"email"`
					SecondaryEmail string `json:"secondaryEmail"`
				}
				if json.NewDecoder(cResp.Body).Decode(&contact) == nil {
					if ticket.Phone == "" {
						ticket.Phone = firstNonEmpty(contact.Phone, contact.Mobile)
					}
					ticket.ContactPhone = firstNonEmpty(ticket.ContactPhone, ticket.Phone, contact.Phone, contact.Mobile)
					if ticket.Email == "" {
						ticket.Email = firstNonEmpty(contact.Email, contact.SecondaryEmail)
					}
					ticket.ContactEmail = firstNonEmpty(ticket.ContactEmail, ticket.Email, contact.Email, contact.SecondaryEmail)
					if ticket.ContactName == "" {
						ticket.ContactName = cleanContactName(
							contact.FullName,
							strings.TrimSpace(contact.FirstName+" "+contact.LastName),
							contact.FirstName,
							contact.LastName,
						)
					}
					if ticket.Phone == "" {
						phoneCandidate := firstNonEmpty(contact.FullName, contact.FirstName, contact.LastName)
						if looksLikePhone(phoneCandidate) {
							ticket.Phone = phoneCandidate
						}
					}
				}
				cResp.Body.Close()
			}
		}
	}

	if looksLikePhone(ticket.ContactName) {
		if ticket.Phone == "" {
			ticket.Phone = ticket.ContactName
		}
		ticket.ContactName = ""
	}

	if ticket.AssigneeID != "" {
		if agentInfo, err := s.GetAgent(ticket.AssigneeID); err == nil {
			ticket.AssigneeName = agentInfo.Name
			ticket.AssigneeEmail = agentInfo.Email
		} else {
			log.Printf("[ZohoService] Could not fetch agent %s: %v", ticket.AssigneeID, err)
		}
	}

	return &ticket, nil
}

// ZohoThread represents a message thread inside a Zoho Desk ticket (WhatsApp / email thread)
type ZohoThread struct {
	ID          string    `json:"id"`
	Channel     string    `json:"channel"`
	AuthorName  string    `json:"authorName"`
	AuthorType  string    `json:"authorType"` // agent, contact, system
	Summary     string    `json:"summary"`
	Content     string    `json:"content"`
	CreatedTime time.Time `json:"createdTime"`
}

// GetTicketThreads retrieves chronological messaging history / chat logs for a ticket
func (s *ZohoService) GetTicketThreads(ticketID string) ([]ZohoThread, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}

	orgID, err := s.getOrgID()
	if err != nil {
		return nil, err
	}

	urlStr := fmt.Sprintf("https://desk.zoho.com/api/v1/tickets/%s/threads?limit=100", ticketID)
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get threads failed with status %d", resp.StatusCode)
	}

	var threadsResponse struct {
		Data []ZohoThread `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&threadsResponse); err != nil {
		return nil, err
	}

	return threadsResponse.Data, nil
}

// ReplyTicket sends a response message into Zoho Desk which routes via the appropriate active channel (WhatsApp/Email)
func (s *ZohoService) ReplyTicket(ticketID string, content string, channel string) (*ZohoThread, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}

	orgID, err := s.getOrgID()
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"channel": channel,
		"content": content,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	urlStr := fmt.Sprintf("https://desk.zoho.com/api/v1/tickets/%s/threads", ticketID)
	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("reply ticket failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var thread ZohoThread
	if err := json.NewDecoder(resp.Body).Decode(&thread); err != nil {
		return nil, err
	}

	return &thread, nil
}

// 🚀 ======================================================================
// 🚀 METODO AGREGADO: ReplyWhatsAppLiveChat para h.zohoSvc.ReplyWhatsAppLiveChat
// 🚀 ======================================================================

// ReplyWhatsAppLiveChat envía una respuesta de WhatsApp real al cliente usando la API de Mensajería Instantánea de Zoho Desk.
// Esto evita que el mensaje se guarde como un simple comentario y mantiene el formato nativo de chat.
func (s *ZohoService) ReplyWhatsAppLiveChat(ticketID string, content string, agentEmail string, cannedMessageID string) (*ZohoThread, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}
	orgID, err := s.getOrgID()
	if err != nil {
		return nil, err
	}

	// 1. OBTENER EL CONVERSATION ID
	// La API pública de Zoho requiere este ID específico para mensajería
	convURL := fmt.Sprintf("https://desk.zoho.com/api/v1/tickets/%s", ticketID)
	req, _ := http.NewRequest("GET", convURL, nil)
	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	ticketBytes, _ := io.ReadAll(resp.Body)
	var ticketData map[string]interface{}
	if err := json.Unmarshal(ticketBytes, &ticketData); err != nil {
		return nil, fmt.Errorf("error parseando json del ticket: %w", err)
	}

	source, ok := ticketData["source"].(map[string]interface{})
	if !ok || source["extId"] == nil {
		return nil, fmt.Errorf("el ticket no contiene una sesion de mensajeria activa (source.extId)")
	}
	sessionID := source["extId"].(string)

	// 2. 🚀 PAYLOAD LIQUIDADO SIN 'displayMessage'
	payload := map[string]interface{}{
		"message": content,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error codificando json: %w", err)
	}

	// 3. 🎯 Endpoint oficial de Zoho Desk para IMSession
	urlStr := fmt.Sprintf("https://desk.zoho.com/api/v1/im/sessions/%s/messages", sessionID)

	req, err = http.NewRequest("POST", urlStr, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("error creando request: %w", err)
	}

	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error ejecutando request al endpoint de consola: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error leyendo respuesta: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		log.Printf("[ZohoService] Error usando endpoint de consola (Status %d): %s", resp.StatusCode, string(respBytes))
		return nil, fmt.Errorf("zoho retorno status %d: %s", resp.StatusCode, string(respBytes))
	}

	log.Printf("[ZohoService] ✅ Mensaje enviado exitosamente vía API pública.")

	var msgResp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(respBytes, &msgResp)
	msgID := msgResp.ID
	if msgID == "" {
		msgID = sessionID
	}

	return &ZohoThread{
		ID:          msgID,
		Channel:     "WhatsApp",
		Content:     content,
		AuthorType:  "agent",
		CreatedTime: time.Now(),
	}, nil
}

// UpdateTicketStatus changes status or metadata on a Zoho Desk Ticket
func (s *ZohoService) UpdateTicketStatus(ticketID string, stage string, status string, assigneeID string) error {
	token, err := s.GetAccessToken()
	if err != nil {
		return err
	}

	orgID, err := s.getOrgID()
	if err != nil {
		return err
	}

	payload := make(map[string]interface{})
	if stage != "" {
		payload["stage"] = stage
	}
	if status != "" {
		payload["status"] = status
	}
	if assigneeID != "" {
		payload["assigneeId"] = assigneeID
	}

	if len(payload) == 0 {
		return errors.New("no update fields provided")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	urlStr := fmt.Sprintf("https://desk.zoho.com/api/v1/tickets/%s", ticketID)
	req, err := http.NewRequest("PATCH", urlStr, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update ticket failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// ZohoThreadMessage represents a sub-message within a Zoho Desk ticket thread
type ZohoThreadMessage struct {
	ID          string    `json:"id"`
	Summary     string    `json:"summary"`
	Direction   string    `json:"direction"` // IN, OUT
	Type        string    `json:"type"`      // TEXT, INFO
	CreatedTime time.Time `json:"createdTime"`
	Author      *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"` // CONNECTION, AGENT
	} `json:"author"`
}

// GetThreadMessages retrieves individual messaging bubbles / chat logs inside a specific thread
func (s *ZohoService) GetThreadMessages(threadID string) ([]ZohoThreadMessage, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}

	orgID, err := s.getOrgID()
	if err != nil {
		return nil, err
	}

	urlStr := fmt.Sprintf("https://desk.zoho.com/api/v1/threads/%s/messages", threadID)
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get thread messages failed with status %d", resp.StatusCode)
	}

	var msgResponse struct {
		Data []ZohoThreadMessage `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&msgResponse); err != nil {
		return nil, err
	}

	return msgResponse.Data, nil
}

// GetAgent retrieves agent details (name, email) for a given Zoho agent ID.
func (s *ZohoService) GetAgent(agentID string) (AgentInfo, error) {
	s.mu.RLock()
	if info, ok := s.agentCache[agentID]; ok {
		s.mu.RUnlock()
		return info, nil
	}
	s.mu.RUnlock()

	token, err := s.GetAccessToken()
	if err != nil {
		return AgentInfo{}, err
	}

	orgID, err := s.getOrgID()
	if err != nil {
		return AgentInfo{}, err
	}

	urlStr := fmt.Sprintf("https://desk.zoho.com/api/v1/agents/%s", agentID)
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return AgentInfo{}, err
	}
	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return AgentInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return AgentInfo{}, fmt.Errorf("get agent failed with status %d", resp.StatusCode)
	}

	var agentResp struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		EmailID string `json:"emailId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&agentResp); err != nil {
		return AgentInfo{}, err
	}

	info := AgentInfo{
		Name:  agentResp.Name,
		Email: agentResp.EmailID,
	}

	s.mu.Lock()
	s.agentCache[agentID] = info
	s.mu.Unlock()

	return info, nil
}

// GetAgentByEmail looks up a Zoho Desk agent by email address and returns AgentInfo including the Zoho agent ID.
// Used during login to sync the web user's ZohoAgentID field.
func (s *ZohoService) GetAgentByEmail(email string) (string, AgentInfo, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return "", AgentInfo{}, err
	}

	orgID, err := s.getOrgID()
	if err != nil {
		return "", AgentInfo{}, err
	}

	urlStr := fmt.Sprintf("https://desk.zoho.com/api/v1/agents/email/%s", url.PathEscape(email))
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "", AgentInfo{}, err
	}
	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", AgentInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", AgentInfo{}, fmt.Errorf("get agent by email failed status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		EmailID string `json:"emailId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", AgentInfo{}, err
	}
	if result.ID == "" {
		return "", AgentInfo{}, fmt.Errorf("no agent ID returned for email %s", email)
	}

	info := AgentInfo{Name: result.Name, Email: result.EmailID}

	s.mu.Lock()
	s.agentCache[result.ID] = info
	s.mu.Unlock()

	return result.ID, info, nil
}

// ZohoAgent is a Zoho Desk agent (transfer target).
type ZohoAgent struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ListAgents returns the Zoho Desk agents of the org (active confirmed agents).
func (s *ZohoService) ListAgents() ([]ZohoAgent, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}
	orgID, err := s.getOrgID()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", "https://desk.zoho.com/api/v1/agents?limit=200&status=ACTIVE", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list agents failed status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			EmailID string `json:"emailId"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	agents := make([]ZohoAgent, 0, len(result.Data))
	for _, a := range result.Data {
		agents = append(agents, ZohoAgent{ID: a.ID, Name: a.Name, Email: a.EmailID})
		s.mu.Lock()
		s.agentCache[a.ID] = AgentInfo{Name: a.Name, Email: a.EmailID}
		s.mu.Unlock()
	}
	return agents, nil
}

// ListWhatsAppTickets returns WhatsApp tickets filtered by assigneeId (use "unassigned" for open queue)
// and status (e.g. "open"). Results are sorted by most recently modified.
func (s *ZohoService) ListWhatsAppTickets(assigneeID string, status string, modifiedTimeRange string) ([]ZohoTicket, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}

	orgID, err := s.getOrgID()
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("channel", "Oberstaff")
	params.Set("sortBy", "-modifiedTime")
	params.Set("limit", "50")
	params.Set("include", "contacts")
	if status != "" {
		params.Set("status", status)
	}
	if modifiedTimeRange != "" {
		params.Set("modifiedTimeRange", modifiedTimeRange)
	}

	urlStr := "https://desk.zoho.com/api/v1/tickets?" + params.Encode()
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list whatsapp tickets failed status %d: %s", resp.StatusCode, string(body))
	}

	var ticketsResp struct {
		Data []ZohoTicket `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ticketsResp); err != nil {
		return nil, err
	}

	// Warm up agent cache if empty to avoid multiple single GetAgent calls
	s.mu.RLock()
	cacheLen := len(s.agentCache)
	s.mu.RUnlock()
	if cacheLen == 0 {
		if _, err := s.ListAgents(); err != nil {
			log.Printf("[ZohoService] Failed to warm up agent cache: %v", err)
		}
	}

	for i := range ticketsResp.Data {
		t := &ticketsResp.Data[i]
		if t.AssigneeID != "" {
			if agentInfo, err := s.GetAgent(t.AssigneeID); err == nil {
				t.AssigneeName = agentInfo.Name
				t.AssigneeEmail = agentInfo.Email
			}
		}
	}

	if assigneeID == "unassigned" {
		var filtered []ZohoTicket
		for _, t := range ticketsResp.Data {
			if t.AssigneeID == "" {
				filtered = append(filtered, t)
			}
		}
		return filtered, nil
	} else if assigneeID != "" {
		var filtered []ZohoTicket
		for _, t := range ticketsResp.Data {
			if t.AssigneeID == assigneeID {
				filtered = append(filtered, t)
			}
		}
		return filtered, nil
	}

	return ticketsResp.Data, nil
}

// AssignTicket assigns a Zoho Desk ticket to the given agent ID.
func (s *ZohoService) AssignTicket(ticketID string, zohoAgentID string) error {
	return s.UpdateTicketStatus(ticketID, "", "", zohoAgentID)
}

type ZohoIMChannel struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	IntegrationService string `json:"integrationService"` // e.g. WHATSAPP
	PhoneNumber        string `json:"phoneNumber,omitempty"`
}

type ZohoTemplateMessage struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Message        string `json:"message"`
	DisplayMessage string `json:"displayMessage"`
	Status         string `json:"status"` // e.g. APPROVED
	Language       string `json:"language,omitempty"`
}

// ListIMChannels retrieves all Instant Messaging channels registered in Zoho Desk
func (s *ZohoService) ListIMChannels() ([]ZohoIMChannel, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}

	orgID, err := s.getOrgID()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", "https://desk.zoho.com/api/v1/im/channels", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list IM channels failed status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []ZohoIMChannel `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetWhatsAppChannelID attempts to locate the channel ID for the WhatsApp integration
func (s *ZohoService) GetWhatsAppChannelID() (string, error) {
	channels, err := s.ListIMChannels()
	if err != nil {
		return "", err
	}

	for _, ch := range channels {
		if strings.EqualFold(ch.IntegrationService, "WHATSAPP") || strings.Contains(strings.ToLower(ch.Name), "oberstaff") {
			return ch.ID, nil
		}
	}

	return "", fmt.Errorf("no WhatsApp channel found in Zoho Desk")
}

// ListDepartments retrieves the list of active department IDs in Zoho Desk
func (s *ZohoService) ListDepartments() ([]string, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}

	orgID, err := s.getOrgID()
	if err != nil {
		return nil, err
	}

	urlStr := "https://desk.zoho.com/api/v1/departments?isEnabled=true"
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list departments failed status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var ids []string
	for _, d := range result.Data {
		if d.ID != "" {
			ids = append(ids, d.ID)
		}
	}
	return ids, nil
}

// ListWhatsAppTemplates retrieves all approved template messages for a department.
// If departmentID is empty, it attempts to fetch them using active departments as a fallback.
func (s *ZohoService) ListWhatsAppTemplates(departmentID string) ([]ZohoTemplateMessage, error) {
	if departmentID == "" {
		deptIDs, err := s.ListDepartments()
		if err == nil && len(deptIDs) > 0 {
			// Try first department
			templates, err := s.listTemplatesForDept(deptIDs[0])
			if err == nil && len(templates) > 0 {
				return templates, nil
			}
			// Try other departments as fallback
			for i := 1; i < len(deptIDs); i++ {
				t, err := s.listTemplatesForDept(deptIDs[i])
				if err == nil && len(t) > 0 {
					return t, nil
				}
			}
		}
	}
	return s.listTemplatesForDept(departmentID)
}

func (s *ZohoService) listTemplatesForDept(departmentID string) ([]ZohoTemplateMessage, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}

	orgID, err := s.getOrgID()
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("type", "TEMPLATE")
	params.Set("limit", "100")
	if departmentID != "" {
		params.Set("departmentId", departmentID)
	}

	urlStr := "https://desk.zoho.com/api/v1/im/cannedMessages?" + params.Encode()
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list templates failed status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []ZohoTemplateMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// SendWhatsAppTemplate initiates a session with a template message
func (s *ZohoService) SendWhatsAppTemplate(channelID string, phone string, templateID string, message string, language string) (string, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return "", err
	}

	orgID, err := s.getOrgID()
	if err != nil {
		return "", err
	}

	if language == "" {
		language = "es" // Default to Spanish
	}

	payload := map[string]interface{}{
		"receiverId":      phone,
		"receiverType":    "PHONENUMBER",
		"cannedMessageId": templateID,
		"language":        language,
		"message":         message,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	urlStr := fmt.Sprintf("https://desk.zoho.com/api/v1/im/channels/%s/initiateSession", channelID)
	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("initiate session failed status %d: %s", resp.StatusCode, string(respBytes))
	}

	var msgResp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(respBytes, &msgResp)

	return msgResp.ID, nil
}

