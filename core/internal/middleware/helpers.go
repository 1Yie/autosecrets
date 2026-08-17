package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"autosecrets.dev/core/internal/database"
)

// ValidName reports whether value is a non-empty, trimmed name of at most
// max characters.
func ValidName(value string, max int) bool {
	value = strings.TrimSpace(value)
	return len(value) > 0 && len(value) <= max
}

// ModeString renders a POSIX file mode as a zero-padded octal string.
func ModeString(m int64) string { return fmt.Sprintf("%04o", m) }

// PageParams parses the shared cursor, limit, and 1-based page query parameters.
func PageParams(r *http.Request) (database.Cursor, int, int, error) {
	cursor, err := database.DecodeCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		return database.Cursor{}, 0, 0, err
	}
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 100 {
		limit = 100
	}
	page := 0
	if raw := r.URL.Query().Get("page"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			page = n
		}
	}
	return cursor, limit, page, nil
}

// ReasonCategories is the closed set of Operation Reason categories.
var ReasonCategories = map[string]bool{
	"maintenance": true, "incident_response": true, "access_change": true,
	"configuration_correction": true, "other": true,
}

// OperationReasonInput is the request shape for an Operation Reason.
type OperationReasonInput struct {
	Category    string `json:"category"`
	Explanation string `json:"explanation"`
	ExternalRef string `json:"external_ref"`
}

// OperationReason validates and normalizes an Operation Reason. The
// explanation is 10-500 characters; the external reference is optional and
// capped so Audit Events stay bounded.
func OperationReason(body *OperationReasonInput) (database.OperationReason, bool) {
	if body == nil {
		return database.OperationReason{}, false
	}
	category := strings.ToLower(strings.TrimSpace(body.Category))
	explanation := strings.TrimSpace(body.Explanation)
	if !ReasonCategories[category] {
		return database.OperationReason{}, false
	}
	runes := []rune(explanation)
	if len(runes) < 10 || len(runes) > 500 {
		return database.OperationReason{}, false
	}
	externalRef := strings.TrimSpace(body.ExternalRef)
	if len(externalRef) > 128 {
		return database.OperationReason{}, false
	}
	return database.OperationReason{Category: category, Explanation: explanation, ExternalRef: externalRef}, true
}

// OperationReasonOr returns the validated reason when supplied, or an
// 'other' default when omitted: Operation Reasons are an operator aid, never
// a hard gate on publishing (product decision 2026-08). A supplied but
// malformed reason is still rejected so typos are not silently ignored.
func OperationReasonOr(body *OperationReasonInput) (database.OperationReason, bool) {
	if body == nil {
		return database.OperationReason{Category: "other"}, true
	}
	return OperationReason(body)
}
