package models

import "time"

// Session represents an active user session.
// The OTRS sessions table uses a key-value store format with columns:
// session_id, data_key, data_value, serialized
type Session struct {
	SessionID    string    `json:"session_id"`
	UserID       int       `json:"user_id"`
	UserLogin    string    `json:"user_login"`
	UserType     string    `json:"user_type"` // "User" (agent) or "Customer"
	UserTitle    string    `json:"user_title"`
	UserFullName string    `json:"user_full_name"`
	CreateTime   time.Time `json:"create_time"`
	LastRequest  time.Time `json:"last_request"`
	RemoteAddr   string    `json:"remote_addr"`
	UserAgent    string    `json:"user_agent"`
}

// SessionData represents a key-value pair in the sessions table.
type SessionData struct {
	SessionID  string `json:"session_id"`
	DataKey    string `json:"data_key"`
	DataValue  string `json:"data_value"`
	Serialized int    `json:"serialized"` // 0=plain text, 1=serialized
}

// Session data keys (matching OTRS conventions).
const (
	SessionKeyUserID          = "UserID"
	SessionKeyUserLogin       = "UserLogin"
	SessionKeyUserType        = "UserType"
	SessionKeyUserTitle       = "UserTitle"
	SessionKeyUserFullname    = "UserFullname" // OTRS uses lowercase 'n'
	SessionKeyCreateTime      = "CreateTime"
	SessionKeyLastRequest     = "LastRequest"
	SessionKeyUserRemoteAddr  = "UserRemoteAddr"
	SessionKeyUserRemoteAgent = "UserRemoteUserAgent"
)

// User type constants.
const (
	UserTypeAgent    = "User"
	UserTypeCustomer = "Customer"
)
