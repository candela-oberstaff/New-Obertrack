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
		// Small startup jitter so several backend instances (or a fast restart
		// loop) don't all hit WAHA at the same instant on boot.
		time.Sleep(startupJitter())
		s.Sync()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.Sync()
		}
	}()
}

// startupJitter returns a short, deterministic-per-process delay (0–15s) derived
// from the current time, avoiding a hard dependency on math/rand.
func startupJitter() time.Duration {
	return time.Duration(time.Now().UnixNano()%15000) * time.Millisecond
}

// Sync updates any local contacts that still have a generic "WA User" name or a
// missing wa_id, resolving their real name/phone from WAHA.
//
// It queries the local candidates FIRST and skips the (potentially large) WAHA
// contacts fetch entirely when there is nothing to enrich — which is the common
// steady state once the initial backfill is done, so most ticks make zero HTTP
// calls to WAHA.
func (s *ContactSyncService) Sync() {
	// Find all contacts that need updating (generic name or empty wa_id).
	var contacts []models.Contact
	s.db.Where("name LIKE ? OR wa_id = '' OR wa_id IS NULL", "WA User %").Find(&contacts)
	if len(contacts) == 0 {
		return // nothing to enrich — don't touch WAHA
	}

	session := s.wahaSvc.GetSession()
	log.Printf("[ContactSync] %d contact(s) pending enrichment; fetching WAHA contacts for session %s", len(contacts), session)

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
