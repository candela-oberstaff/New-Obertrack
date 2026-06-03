package service

import (
	"log"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/models"
)

// ContactSyncService periodically fetches all contacts from WAHA and updates
// the local contacts table with real names and phone numbers.
type ContactSyncService struct {
	db      *gorm.DB
	wahaSvc *WahaService
}

func NewContactSyncService(db *gorm.DB, wahaSvc *WahaService) *ContactSyncService {
	return &ContactSyncService{db: db, wahaSvc: wahaSvc}
}

// Start launches a background goroutine that runs a sync every `interval`.
func (s *ContactSyncService) Start(interval time.Duration) {
	go func() {
		// Run immediately on startup, then on each tick
		s.Sync()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.Sync()
		}
	}()
}

// Sync fetches all WAHA contacts and updates any local contacts that still
// have a generic "WA User" name or a missing phone number.
func (s *ContactSyncService) Sync() {
	session := s.wahaSvc.GetSession()
	log.Printf("[ContactSync] Starting sync for session: %s", session)

	waContacts, err := s.wahaSvc.GetAllContacts(session)
	if err != nil {
		log.Printf("[ContactSync] Could not fetch contacts from WAHA: %v", err)
		return
	}

	// Build a lookup map: waID -> WahaContactResponse
	byWaID := make(map[string]*WahaContactResponse, len(waContacts))
	// Also map numeric part -> WahaContactResponse for @lid contacts
	byNumber := make(map[string]*WahaContactResponse, len(waContacts))
	for i := range waContacts {
		c := &waContacts[i]
		if c.ID != "" {
			byWaID[c.ID] = c
		}
		// Also index by the numeric part of the ID
		numPart := strings.Split(c.ID, "@")[0]
		if numPart != "" {
			if _, exists := byNumber[numPart]; !exists {
				byNumber[numPart] = c
			}
		}
	}

	// Find all contacts that need updating (generic name or empty wa_id)
	var contacts []models.Contact
	s.db.Where("name LIKE ? OR wa_id = '' OR wa_id IS NULL", "WA User %").Find(&contacts)

	updated := 0
	for _, contact := range contacts {
		var match *WahaContactResponse

		// Try exact wa_id match first
		if contact.WaID != "" {
			match = byWaID[contact.WaID]
		}
		// Try numeric phone match
		if match == nil && contact.Phone != "" {
			match = byNumber[contact.Phone]
		}

		if match == nil {
			continue
		}

		changed := false
		updates := map[string]interface{}{}

		// Update name if it's generic or empty
		displayName := match.GetDisplayName()
		if displayName != "" && (contact.Name == "" || strings.HasPrefix(contact.Name, "WA User ")) {
			updates["name"] = displayName
			changed = true
		}

		// Update phone if it's just the numeric WA ID (not a real formatted number)
		realPhone := match.GetPhone()
		if realPhone != "" && realPhone != contact.Phone {
			updates["phone"] = realPhone
			changed = true
		}

		// Backfill wa_id
		if contact.WaID == "" && match.ID != "" {
			updates["wa_id"] = match.ID
			changed = true
		}

		if changed {
			s.db.Model(&contact).Updates(updates)
			updated++
		}
	}

	log.Printf("[ContactSync] Done. Updated %d/%d contacts from %d WAHA contacts",
		updated, len(contacts), len(waContacts))
}
