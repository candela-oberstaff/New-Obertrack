// Package apperrors defines sentinel errors shared across layers so handlers can
// map domain failures to HTTP status codes with errors.Is instead of fragile
// string comparisons.
package apperrors

import "errors"

var (
	// ErrNotFound indicates a requested resource does not exist.
	ErrNotFound = errors.New("not found")
	// ErrAccessDenied indicates the caller is not allowed to act on the resource.
	ErrAccessDenied = errors.New("access denied")
	// ErrInvalidInput indicates the request payload failed validation.
	ErrInvalidInput = errors.New("invalid input")
	// ErrExternalSend indicates an outbound integration (WAHA/Brevo) failed.
	ErrExternalSend = errors.New("failed to send external message")
	// ErrRateLimited indicates an outbound send was throttled by the anti-ban
	// rate limiter and should be retried later (maps to HTTP 429).
	ErrRateLimited = errors.New("outbound rate limit exceeded")
	// ErrColdOutreach indicates a send was blocked because the contact never
	// messaged first — cold outreach is the highest WhatsApp-ban risk (maps to 403).
	ErrColdOutreach = errors.New("cannot message a contact that has not written first")
)
