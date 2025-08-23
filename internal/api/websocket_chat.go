package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in development
		// TODO: Restrict in production
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// ChatMessage represents a message in the chat
type ChatMessage struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"` // user, claude, system
	Message   string      `json:"message"`
	Timestamp time.Time   `json:"timestamp"`
	Context   interface{} `json:"context,omitempty"`
	UserID    string      `json:"userId,omitempty"`
	SessionID string      `json:"sessionId,omitempty"`
}

// ChatClient represents a connected chat client
type ChatClient struct {
	conn      *websocket.Conn
	send      chan ChatMessage
	sessionID string
	userID    string
	page      string
}

// ChatHub manages all active chat connections
type ChatHub struct {
	clients    map[string]*ChatClient
	broadcast  chan ChatMessage
	register   chan *ChatClient
	unregister chan *ChatClient
	messages   []ChatMessage // In-memory message history
	mu         sync.RWMutex
}

// Global chat hub
var chatHub = &ChatHub{
	clients:    make(map[string]*ChatClient),
	broadcast:  make(chan ChatMessage),
	register:   make(chan *ChatClient),
	unregister: make(chan *ChatClient),
	messages:   make([]ChatMessage, 0),
}

// Start the chat hub
func init() {
	go chatHub.run()
}

// run handles the chat hub operations
func (h *ChatHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.sessionID] = client
			h.mu.Unlock()
			
			log.Printf("Chat client connected: session=%s, page=%s", client.sessionID, client.page)
			
			// Send recent message history
			h.sendHistory(client)
			
			// Send welcome message
			welcome := ChatMessage{
				ID:        generateMessageID(),
				Type:      "system",
				Message:   "Connected to Claude Code real-time chat. I can see you're on: " + client.page,
				Timestamp: time.Now(),
			}
			client.send <- welcome

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.sessionID]; ok {
				delete(h.clients, client.sessionID)
				close(client.send)
				h.mu.Unlock()
				log.Printf("Chat client disconnected: session=%s", client.sessionID)
			} else {
				h.mu.Unlock()
			}

		case message := <-h.broadcast:
			h.mu.Lock()
			// Store message in history
			h.messages = append(h.messages, message)
			// Keep only last 100 messages
			if len(h.messages) > 100 {
				h.messages = h.messages[len(h.messages)-100:]
			}
			
			// Broadcast to all connected clients
			for _, client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client's send channel is full, close it
					delete(h.clients, client.sessionID)
					close(client.send)
				}
			}
			h.mu.Unlock()
			
			// Process the message and generate response if needed
			if message.Type == "user" {
				go h.processUserMessage(message)
			}
		}
	}
}

// processUserMessage handles incoming user messages and generates responses
func (h *ChatHub) processUserMessage(msg ChatMessage) {
	log.Printf("Processing user message: %s", msg.Message)
	
	// Simulate Claude thinking (in production, this would call Claude API)
	time.Sleep(500 * time.Millisecond)
	
	// Generate a contextual response
	response := ChatMessage{
		ID:        generateMessageID(),
		Type:      "claude",
		Timestamp: time.Now(),
	}
	
	// Simple response logic (in production, use Claude API)
	switch {
	case contains(msg.Message, "hello", "hi", "hey"):
		response.Message = "Hello! I'm Claude Code, here to help in real-time. What can I assist you with?"
	
	case contains(msg.Message, "broken", "error", "bug", "issue"):
		response.Message = "I see you're reporting an issue. Can you click 'Select Element' and point to the specific problem? I'll analyze it immediately."
	
	case contains(msg.Message, "dropdown", "select", "option"):
		response.Message = "Dropdown issues are common! If it's showing IDs instead of names, that usually means we need to add a lookup table join. I can fix that for you."
	
	case contains(msg.Message, "slow", "performance", "loading"):
		response.Message = "Performance issue noted. Let me check if there are any N+1 queries or missing indexes. What specific action is slow?"
	
	case contains(msg.Message, "color", "style", "css", "design"):
		response.Message = "I can help with styling! Would you like me to update the colors to match the GOTRS theme, or do you have specific colors in mind?"
	
	default:
		response.Message = "I understand. I'm analyzing the context of your message along with the page state. In production, I would provide more intelligent responses through the Claude API."
	}
	
	// Broadcast the response
	h.broadcast <- response
}

// sendHistory sends recent message history to a newly connected client
func (h *ChatHub) sendHistory(client *ChatClient) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	// Send last 20 messages
	start := len(h.messages) - 20
	if start < 0 {
		start = 0
	}
	
	for i := start; i < len(h.messages); i++ {
		select {
		case client.send <- h.messages[i]:
		default:
			// Channel full, stop sending history
			return
		}
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *ChatClient) readPump() {
	defer func() {
		chatHub.unregister <- c
		c.conn.Close()
	}()
	
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	
	for {
		var msg ChatMessage
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		
		// Add metadata to message
		msg.ID = generateMessageID()
		msg.Timestamp = time.Now()
		msg.SessionID = c.sessionID
		msg.UserID = c.userID
		
		log.Printf("Received message from %s: %s", c.sessionID, msg.Message)
		
		// Broadcast the message
		chatHub.broadcast <- msg
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *ChatClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			
			if err := c.conn.WriteJSON(message); err != nil {
				return
			}
			
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleWebSocketChat handles WebSocket connections for real-time chat
func handleWebSocketChat(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	
	// Get session info
	sessionID := c.Query("session")
	if sessionID == "" {
		sessionID = generateMessageID()
	}
	
	page := c.Query("page")
	userID := ""
	
	// Try to get user ID from context
	if user := getUserFromContext(c); user != nil {
		userID = fmt.Sprintf("%d", user.ID)
	}
	
	// Create new client
	client := &ChatClient{
		conn:      conn,
		send:      make(chan ChatMessage, 256),
		sessionID: sessionID,
		userID:    userID,
		page:      page,
	}
	
	// Register the client
	chatHub.register <- client
	
	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// Helper functions
func generateMessageID() string {
	return time.Now().Format("20060102150405") + "-" + generateRandomString(6)
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

func contains(text string, words ...string) bool {
	lower := strings.ToLower(text)
	for _, word := range words {
		if strings.Contains(lower, strings.ToLower(word)) {
			return true
		}
	}
	return false
}