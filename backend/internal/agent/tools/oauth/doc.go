// Package oauth provides the shared OAuth + Apps Script bridge infrastructure
// used by gmail-agent, google-calendar-agent, and outlook-agent skills.
//
// BridgeClient is a thin HTTP wrapper around the Google Apps Script
// deployment URL used by Gmail and Calendar. It does NOT speak Google
// OAuth — Google access lives on the Apps Script side, executed as the
// deploying admin.
//
// OutlookOAuth implements the Microsoft OAuth 2.0 authorization-code +
// refresh-token flow. Refresh-token storage is encrypted with the existing
// pkg/utils.EncryptionManager (AES-GCM); see TokenStore.
//
// state.go provides HMAC-signed state-parameter (CSRF + replay defense).
//
// All three skills are single-user-mode only.
package oauth
