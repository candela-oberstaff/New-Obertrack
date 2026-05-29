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
	// Contact details fetched separately
	ContactPhone  string `json:"-"`
	ContactEmail  string `json:"-"`
	// Assignee/owner details fetched separately
	AssigneeName  string `json:"-"`
	AssigneeEmail string `json:"-"`
}

// ListTickets retrieves the active tickets list from Zoho Desk API
func (s *ZohoService) ListTickets() ([]ZohoTicket, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}

	orgID, err := s.getOrgID()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", "https://desk.zoho.com/api/v1/tickets?sortBy=-modifiedTime&limit=50", nil)
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

	log.Printf("[ZohoService] GetTicketDetail Raw JSON: %s\n", string(body))

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
					FirstName string `json:"firstName"`
					LastName  string `json:"lastName"`
					FullName  string `json:"fullName"`
					Phone     string `json:"phone"`
					Email     string `json:"email"`
				}
				if json.NewDecoder(cResp.Body).Decode(&contact) == nil {
					if ticket.Phone == "" {
						ticket.Phone = contact.Phone
					}
					if ticket.Email == "" {
						ticket.Email = contact.Email
					}
					if ticket.ContactName == "" {
						if contact.FullName != "" {
							ticket.ContactName = contact.FullName
						} else {
							ticket.ContactName = contact.FirstName + " " + contact.LastName
						}
					}
				}
				cResp.Body.Close()
			}
		}
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

func (s *ZohoService) ReplyWhatsAppLiveChat(ticketID string, content string) (*ZohoThread, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}

	orgID, err := s.getOrgID()
	if err != nil {
		return nil, err
	}

	// 🚀 PAYLOAD CORREGIDO: Estructura exacta para /sendReply en canales de mensajería
	payload := map[string]interface{}{
		"channel":    "phone", // Mantén "phone" o cambia por "whatsapp" si tu canal es nativo avanzado
		"text":       content, // Zoho Desk para mensajería instantánea prefiere "text" o "content" según versión
		"content":    content, // Enviamos ambos para asegurar compatibilidad con tu DTO de respuesta
		"isPublic":   true,    // Obligatorio en Zoho para que se dispache hacia el cliente final
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	urlStr := fmt.Sprintf("https://desk.zoho.com/api/v1/tickets/%s/sendReply", ticketID)
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
		return nil, fmt.Errorf("reply whatsapp failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Al usar sendReply para mensajería, a veces Zoho devuelve un formato compacto.
	// Si el Unmarshal falla abajo, es porque la respuesta mapea directo a un Thread estructurado.
	var thread ZohoThread
	respBytes, _ := io.ReadAll(resp.Body)
	
	// Intentamos decodificar el JSON de la respuesta oficial de Zoho
	if err := json.Unmarshal(respBytes, &thread); err != nil {
		// Fallback por si la respuesta viene envuelta en un nodo "data"
		var wrapper struct {
			Data ZohoThread `json:"data"`
		}
		if json.Unmarshal(respBytes, &wrapper) == nil {
			return &wrapper.Data, nil
		}
		
		// Fallback de emergencia si el mensaje se envió con éxito pero cambió el formato de respuesta
		return &ZohoThread{
			ID:          "wh_" + fmt.Sprintf("%d", time.Now().Unix()),
			Channel:     "phone",
			Summary:     content,
			CreatedTime: time.Now(),
		}, nil
	}

	return &thread, nil
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