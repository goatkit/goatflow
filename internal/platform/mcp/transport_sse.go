package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	// ProtocolVersion202503 is the MCP Streamable HTTP protocol version.
	ProtocolVersion202503 = "2025-03-26"

	// SessionHeader is the HTTP header for MCP session identification.
	SessionHeader = "Mcp-Session-Id"

	// SSEHeartbeatInterval is the keepalive interval for SSE connections.
	SSEHeartbeatInterval = 30 * time.Second
)

// WriteSSEEvent writes a single SSE event to the writer and flushes.
func WriteSSEEvent(w http.ResponseWriter, flusher http.Flusher, eventType string, data []byte) {
	if eventType != "" {
		fmt.Fprintf(w, "event: %s\n", eventType)
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// WriteSSEMessage writes a JSON-RPC response as an SSE "message" event.
func WriteSSEMessage(w http.ResponseWriter, flusher http.Flusher, response []byte) {
	WriteSSEEvent(w, flusher, "message", response)
}

// WriteSSEHeartbeat sends a comment line to keep the connection alive.
func WriteSSEHeartbeat(w http.ResponseWriter, flusher http.Flusher) {
	fmt.Fprintf(w, ": keepalive\n\n")
	flusher.Flush()
}

// HandleStreamableHTTPPost handles POST requests for the MCP Streamable HTTP transport.
// For "initialize" requests: creates a session and returns the session ID in response headers.
// For other requests: processes via the session's server and returns JSON or SSE.
func HandleStreamableHTTPPost(
	w http.ResponseWriter, r *http.Request,
	sessions *SessionManager, bridge *APIBridge,
	userID int, userLogin, userRole string,
) {
	body := make([]byte, 0)
	if r.Body != nil {
		var err error
		body, err = readBody(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "Failed to read request body")
			return
		}
	}

	// Parse to check if this is an initialize request
	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON-RPC request")
		return
	}

	if req.Method == "initialize" {
		// Create a new session
		session := sessions.Create(userID, userLogin, userRole, bridge)

		response, err := session.Server.HandleMessage(r.Context(), body)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		w.Header().Set(SessionHeader, session.ID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response)
		return
	}

	// For all other methods, require a session
	sessionID := r.Header.Get(SessionHeader)
	if sessionID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing Mcp-Session-Id header")
		return
	}

	session := sessions.Get(sessionID)
	if session == nil {
		writeJSONError(w, http.StatusNotFound, "Session not found or expired")
		return
	}

	// Verify user matches session
	if session.UserID != userID {
		writeJSONError(w, http.StatusForbidden, "Session user mismatch")
		return
	}

	// Process the message
	response, err := session.Server.HandleMessage(r.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// No response for notifications
	if response == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

// HandleStreamableHTTPGet handles GET requests for the MCP SSE notification stream.
// Keeps the connection open and sends server-initiated notifications.
func HandleStreamableHTTPGet(
	w http.ResponseWriter, r *http.Request,
	sessions *SessionManager,
	userID int,
) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	sessionID := r.Header.Get(SessionHeader)
	if sessionID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing Mcp-Session-Id header")
		return
	}

	session := sessions.Get(sessionID)
	if session == nil {
		writeJSONError(w, http.StatusNotFound, "Session not found or expired")
		return
	}

	if session.UserID != userID {
		writeJSONError(w, http.StatusForbidden, "Session user mismatch")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Send initial connected event
	WriteSSEEvent(w, flusher, "open", []byte(`{"status":"ok"}`))

	heartbeat := time.NewTicker(SSEHeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-session.NotifyChan:
			if !ok {
				return
			}
			WriteSSEMessage(w, flusher, msg)
		case <-heartbeat.C:
			WriteSSEHeartbeat(w, flusher)
		}
	}
}

// HandleStreamableHTTPDelete handles DELETE requests to terminate an MCP session.
func HandleStreamableHTTPDelete(
	w http.ResponseWriter, r *http.Request,
	sessions *SessionManager,
	userID int,
) {
	sessionID := r.Header.Get(SessionHeader)
	if sessionID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing Mcp-Session-Id header")
		return
	}

	session := sessions.Get(sessionID)
	if session == nil {
		writeJSONError(w, http.StatusNotFound, "Session not found or expired")
		return
	}

	if session.UserID != userID {
		writeJSONError(w, http.StatusForbidden, "Session user mismatch")
		return
	}

	sessions.Delete(sessionID)
	w.WriteHeader(http.StatusNoContent)
}

func readBody(r *http.Request) ([]byte, error) {
	var body []byte
	buf := make([]byte, 4096)
	for {
		n, err := r.Body.Read(buf)
		if n > 0 {
			body = append(body, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	return body, nil
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
