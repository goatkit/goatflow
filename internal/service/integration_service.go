package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// IntegrationService handles third-party integrations
type IntegrationService struct {
	integrationRepo repository.IntegrationRepository
	ticketRepo      repository.TicketRepository
	userRepo        repository.UserRepository
	httpClient      *http.Client
	mu              sync.RWMutex
	activeIntegrations map[string]*Integration
}

// Integration represents a third-party integration
type Integration struct {
	ID          int                    `json:"id"`
	Type        string                 `json:"type"` // slack, teams, discord, etc.
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Config      map[string]interface{} `json:"config"`
	Enabled     bool                   `json:"enabled"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	LastSync    *time.Time             `json:"last_sync,omitempty"`
	Status      string                 `json:"status"` // connected, error, pending
	Error       string                 `json:"error,omitempty"`
}

// SlackConfig represents Slack integration configuration
type SlackConfig struct {
	WorkspaceID   string            `json:"workspace_id"`
	TeamID        string            `json:"team_id"`
	BotToken      string            `json:"bot_token"`
	AppToken      string            `json:"app_token"`
	SigningSecret string            `json:"signing_secret"`
	Channels      []SlackChannel    `json:"channels"`
	UserMappings  map[string]int    `json:"user_mappings"` // Slack user ID -> GOTRS user ID
	Features      SlackFeatures     `json:"features"`
}

// SlackChannel represents a Slack channel configuration
type SlackChannel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	QueueID       int    `json:"queue_id"`
	NotifyOnNew   bool   `json:"notify_on_new"`
	NotifyOnUpdate bool  `json:"notify_on_update"`
	NotifyOnClose bool   `json:"notify_on_close"`
}

// SlackFeatures represents enabled Slack features
type SlackFeatures struct {
	CreateTickets    bool `json:"create_tickets"`
	UpdateTickets    bool `json:"update_tickets"`
	SearchTickets    bool `json:"search_tickets"`
	Notifications    bool `json:"notifications"`
	SlashCommands    bool `json:"slash_commands"`
	InteractiveMessages bool `json:"interactive_messages"`
}

// SlackMessage represents a Slack message
type SlackMessage struct {
	Channel     string            `json:"channel"`
	Text        string            `json:"text,omitempty"`
	Blocks      []SlackBlock      `json:"blocks,omitempty"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
	ThreadTS    string            `json:"thread_ts,omitempty"`
}

// SlackBlock represents a Slack block
type SlackBlock struct {
	Type     string                 `json:"type"`
	Text     *SlackText             `json:"text,omitempty"`
	Elements []SlackElement         `json:"elements,omitempty"`
	Fields   []SlackText            `json:"fields,omitempty"`
	Accessory *SlackElement         `json:"accessory,omitempty"`
}

// SlackText represents Slack text
type SlackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// SlackElement represents a Slack element
type SlackElement struct {
	Type     string       `json:"type"`
	Text     *SlackText   `json:"text,omitempty"`
	Value    string       `json:"value,omitempty"`
	ActionID string       `json:"action_id,omitempty"`
	Options  []SlackOption `json:"options,omitempty"`
}

// SlackOption represents a Slack option
type SlackOption struct {
	Text  SlackText `json:"text"`
	Value string    `json:"value"`
}

// SlackAttachment represents a Slack attachment
type SlackAttachment struct {
	Color      string       `json:"color,omitempty"`
	Title      string       `json:"title,omitempty"`
	TitleLink  string       `json:"title_link,omitempty"`
	Text       string       `json:"text,omitempty"`
	Fields     []SlackField `json:"fields,omitempty"`
	Footer     string       `json:"footer,omitempty"`
	FooterIcon string       `json:"footer_icon,omitempty"`
	Timestamp  int64        `json:"ts,omitempty"`
}

// SlackField represents a Slack field
type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// TeamsConfig represents Microsoft Teams integration configuration
type TeamsConfig struct {
	TenantID      string            `json:"tenant_id"`
	ClientID      string            `json:"client_id"`
	ClientSecret  string            `json:"client_secret"`
	WebhookURLs   map[string]string `json:"webhook_urls"` // Channel -> Webhook URL
	UserMappings  map[string]int    `json:"user_mappings"` // Teams user ID -> GOTRS user ID
	Features      TeamsFeatures     `json:"features"`
}

// TeamsFeatures represents enabled Teams features
type TeamsFeatures struct {
	CreateTickets bool `json:"create_tickets"`
	UpdateTickets bool `json:"update_tickets"`
	Notifications bool `json:"notifications"`
	AdaptiveCards bool `json:"adaptive_cards"`
}

// TeamsMessage represents a Teams message
type TeamsMessage struct {
	Type        string            `json:"@type"`
	Context     string            `json:"@context"`
	Summary     string            `json:"summary,omitempty"`
	Title       string            `json:"title,omitempty"`
	Text        string            `json:"text,omitempty"`
	Sections    []TeamsSection    `json:"sections,omitempty"`
	Actions     []TeamsAction     `json:"potentialAction,omitempty"`
}

// TeamsSection represents a Teams section
type TeamsSection struct {
	Title      string       `json:"title,omitempty"`
	Text       string       `json:"text,omitempty"`
	ActivityTitle string    `json:"activityTitle,omitempty"`
	ActivitySubtitle string `json:"activitySubtitle,omitempty"`
	ActivityImage string    `json:"activityImage,omitempty"`
	Facts      []TeamsFact  `json:"facts,omitempty"`
}

// TeamsFact represents a Teams fact
type TeamsFact struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// TeamsAction represents a Teams action
type TeamsAction struct {
	Type    string        `json:"@type"`
	Name    string        `json:"name"`
	Targets []TeamsTarget `json:"targets,omitempty"`
}

// TeamsTarget represents a Teams target
type TeamsTarget struct {
	OS  string `json:"os"`
	URI string `json:"uri"`
}

// NewIntegrationService creates a new integration service
func NewIntegrationService(integrationRepo repository.IntegrationRepository, ticketRepo repository.TicketRepository, userRepo repository.UserRepository) *IntegrationService {
	is := &IntegrationService{
		integrationRepo: integrationRepo,
		ticketRepo:      ticketRepo,
		userRepo:        userRepo,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		activeIntegrations: make(map[string]*Integration),
	}
	
	// Load active integrations
	is.loadActiveIntegrations()
	
	return is
}

// ConfigureSlack configures Slack integration
func (s *IntegrationService) ConfigureSlack(config *SlackConfig) error {
	integration := &Integration{
		Type:        "slack",
		Name:        "Slack Integration",
		Description: fmt.Sprintf("Slack workspace: %s", config.WorkspaceID),
		Config:      s.slackConfigToMap(config),
		Enabled:     true,
		Status:      "pending",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	// Test connection
	if err := s.testSlackConnection(config); err != nil {
		integration.Status = "error"
		integration.Error = err.Error()
	} else {
		integration.Status = "connected"
		now := time.Now()
		integration.LastSync = &now
	}
	
	// Save integration
	if err := s.integrationRepo.Create(integration); err != nil {
		return err
	}
	
	// Add to active integrations
	if integration.Enabled && integration.Status == "connected" {
		s.mu.Lock()
		s.activeIntegrations["slack"] = integration
		s.mu.Unlock()
	}
	
	return nil
}

// ConfigureTeams configures Microsoft Teams integration
func (s *IntegrationService) ConfigureTeams(config *TeamsConfig) error {
	integration := &Integration{
		Type:        "teams",
		Name:        "Microsoft Teams Integration",
		Description: fmt.Sprintf("Tenant: %s", config.TenantID),
		Config:      s.teamsConfigToMap(config),
		Enabled:     true,
		Status:      "pending",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	// Test connection
	if err := s.testTeamsConnection(config); err != nil {
		integration.Status = "error"
		integration.Error = err.Error()
	} else {
		integration.Status = "connected"
		now := time.Now()
		integration.LastSync = &now
	}
	
	// Save integration
	if err := s.integrationRepo.Create(integration); err != nil {
		return err
	}
	
	// Add to active integrations
	if integration.Enabled && integration.Status == "connected" {
		s.mu.Lock()
		s.activeIntegrations["teams"] = integration
		s.mu.Unlock()
	}
	
	return nil
}

// SendSlackNotification sends a notification to Slack
func (s *IntegrationService) SendSlackNotification(event string, data interface{}) error {
	s.mu.RLock()
	integration, exists := s.activeIntegrations["slack"]
	s.mu.RUnlock()
	
	if !exists || !integration.Enabled {
		return nil // Slack not configured
	}
	
	config := s.mapToSlackConfig(integration.Config)
	
	// Build message based on event
	message := s.buildSlackMessage(event, data, config)
	
	// Send to configured channels
	for _, channel := range config.Channels {
		if s.shouldNotifyChannel(channel, event) {
			message.Channel = channel.ID
			if err := s.sendSlackMessage(config.BotToken, message); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// SendTeamsNotification sends a notification to Teams
func (s *IntegrationService) SendTeamsNotification(event string, data interface{}) error {
	s.mu.RLock()
	integration, exists := s.activeIntegrations["teams"]
	s.mu.RUnlock()
	
	if !exists || !integration.Enabled {
		return nil // Teams not configured
	}
	
	config := s.mapToTeamsConfig(integration.Config)
	
	// Build message based on event
	message := s.buildTeamsMessage(event, data)
	
	// Send to configured channels
	for channel, webhookURL := range config.WebhookURLs {
		if err := s.sendTeamsMessage(webhookURL, message); err != nil {
			return fmt.Errorf("failed to send to channel %s: %w", channel, err)
		}
	}
	
	return nil
}

// HandleSlackCommand handles Slack slash commands
func (s *IntegrationService) HandleSlackCommand(command, text, userID, channelID string) (string, error) {
	switch command {
	case "/ticket":
		return s.handleSlackTicketCommand(text, userID, channelID)
	case "/search":
		return s.handleSlackSearchCommand(text, userID)
	case "/status":
		return s.handleSlackStatusCommand(userID)
	default:
		return "Unknown command", nil
	}
}

// handleSlackTicketCommand handles /ticket command
func (s *IntegrationService) handleSlackTicketCommand(text, userID, channelID string) (string, error) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "Usage: /ticket create <subject> | /ticket view <id> | /ticket list", nil
	}
	
	action := parts[0]
	switch action {
	case "create":
		if len(parts) < 2 {
			return "Please provide a ticket subject", nil
		}
		subject := strings.Join(parts[1:], " ")
		return s.createTicketFromSlack(subject, userID, channelID)
		
	case "view":
		if len(parts) < 2 {
			return "Please provide a ticket ID", nil
		}
		return s.viewTicketFromSlack(parts[1], userID)
		
	case "list":
		return s.listTicketsFromSlack(userID)
		
	default:
		return "Unknown action. Use: create, view, or list", nil
	}
}

// handleSlackSearchCommand handles /search command
func (s *IntegrationService) handleSlackSearchCommand(query, userID string) (string, error) {
	if query == "" {
		return "Please provide a search query", nil
	}
	
	// Search tickets
	// This would integrate with search service
	return fmt.Sprintf("Searching for: %s", query), nil
}

// handleSlackStatusCommand handles /status command
func (s *IntegrationService) handleSlackStatusCommand(userID string) (string, error) {
	// Get user's tickets and stats
	// This would integrate with ticket service
	return "Your ticket status summary", nil
}

// Helper methods

// testSlackConnection tests Slack connection
func (s *IntegrationService) testSlackConnection(config *SlackConfig) error {
	// Test API connection
	req, err := http.NewRequest("GET", "https://slack.com/api/auth.test", nil)
	if err != nil {
		return err
	}
	
	req.Header.Set("Authorization", "Bearer "+config.BotToken)
	
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	
	if ok, _ := result["ok"].(bool); !ok {
		if errorMsg, exists := result["error"].(string); exists {
			return fmt.Errorf("Slack API error: %s", errorMsg)
		}
		return fmt.Errorf("Slack authentication failed")
	}
	
	return nil
}

// testTeamsConnection tests Teams connection
func (s *IntegrationService) testTeamsConnection(config *TeamsConfig) error {
	// Test webhook URLs
	for channel, webhookURL := range config.WebhookURLs {
		testMessage := &TeamsMessage{
			Type:    "MessageCard",
			Context: "https://schema.org/extensions",
			Summary: "GOTRS Integration Test",
			Title:   "Integration Test",
			Text:    "Testing GOTRS integration with Microsoft Teams",
		}
		
		if err := s.sendTeamsMessage(webhookURL, testMessage); err != nil {
			return fmt.Errorf("failed to test channel %s: %w", channel, err)
		}
	}
	
	return nil
}

// sendSlackMessage sends a message to Slack
func (s *IntegrationService) sendSlackMessage(token string, message *SlackMessage) error {
	url := "https://slack.com/api/chat.postMessage"
	
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	
	if ok, _ := result["ok"].(bool); !ok {
		if errorMsg, exists := result["error"].(string); exists {
			return fmt.Errorf("Slack API error: %s", errorMsg)
		}
		return fmt.Errorf("failed to send message")
	}
	
	return nil
}

// sendTeamsMessage sends a message to Teams
func (s *IntegrationService) sendTeamsMessage(webhookURL string, message *TeamsMessage) error {
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	
	resp, err := s.httpClient.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Teams webhook error: %s", string(body))
	}
	
	return nil
}

// buildSlackMessage builds a Slack message for an event
func (s *IntegrationService) buildSlackMessage(event string, data interface{}, config *SlackConfig) *SlackMessage {
	message := &SlackMessage{}
	
	switch event {
	case "ticket.created":
		if ticket, ok := data.(*models.Ticket); ok {
			message.Blocks = []SlackBlock{
				{
					Type: "header",
					Text: &SlackText{
						Type: "plain_text",
						Text: fmt.Sprintf("New Ticket #%d", ticket.ID),
					},
				},
				{
					Type: "section",
					Text: &SlackText{
						Type: "mrkdwn",
						Text: fmt.Sprintf("*Subject:* %s\n*Priority:* %s\n*Status:* %s", 
							ticket.Subject, s.getPriorityName(ticket.TicketPriorityID), s.getStatusName(ticket.TicketStateID)),
					},
				},
				{
					Type: "actions",
					Elements: []SlackElement{
						{
							Type: "button",
							Text: &SlackText{
								Type: "plain_text",
								Text: "View Ticket",
							},
							Value:    fmt.Sprintf("view_%d", ticket.ID),
							ActionID: "view_ticket",
						},
						{
							Type: "button",
							Text: &SlackText{
								Type: "plain_text",
								Text: "Assign to Me",
							},
							Value:    fmt.Sprintf("assign_%d", ticket.ID),
							ActionID: "assign_ticket",
						},
					},
				},
			}
		}
		
	case "ticket.updated":
		if ticket, ok := data.(*models.Ticket); ok {
			message.Text = fmt.Sprintf("Ticket #%d has been updated", ticket.ID)
		}
		
	case "ticket.closed":
		if ticket, ok := data.(*models.Ticket); ok {
			message.Text = fmt.Sprintf("Ticket #%d has been closed", ticket.ID)
		}
	}
	
	return message
}

// buildTeamsMessage builds a Teams message for an event
func (s *IntegrationService) buildTeamsMessage(event string, data interface{}) *TeamsMessage {
	message := &TeamsMessage{
		Type:    "MessageCard",
		Context: "https://schema.org/extensions",
	}
	
	switch event {
	case "ticket.created":
		if ticket, ok := data.(*models.Ticket); ok {
			message.Summary = fmt.Sprintf("New Ticket #%d", ticket.ID)
			message.Title = fmt.Sprintf("New Ticket #%d: %s", ticket.ID, ticket.Subject)
			message.Sections = []TeamsSection{
				{
					Facts: []TeamsFact{
						{Name: "Priority", Value: s.getPriorityName(ticket.TicketPriorityID)},
						{Name: "Status", Value: s.getStatusName(ticket.TicketStateID)},
						{Name: "Queue", Value: s.getQueueName(ticket.QueueID)},
					},
				},
			}
			message.Actions = []TeamsAction{
				{
					Type: "OpenUri",
					Name: "View Ticket",
					Targets: []TeamsTarget{
						{
							OS:  "default",
							URI: fmt.Sprintf("https://gotrs.example.com/tickets/%d", ticket.ID),
						},
					},
				},
			}
		}
		
	case "ticket.updated":
		if ticket, ok := data.(*models.Ticket); ok {
			message.Summary = fmt.Sprintf("Ticket #%d Updated", ticket.ID)
			message.Title = fmt.Sprintf("Ticket #%d has been updated", ticket.ID)
			message.Text = ticket.Subject
		}
		
	case "ticket.closed":
		if ticket, ok := data.(*models.Ticket); ok {
			message.Summary = fmt.Sprintf("Ticket #%d Closed", ticket.ID)
			message.Title = fmt.Sprintf("Ticket #%d has been closed", ticket.ID)
			message.Text = ticket.Subject
		}
	}
	
	return message
}

// shouldNotifyChannel checks if a channel should be notified for an event
func (s *IntegrationService) shouldNotifyChannel(channel SlackChannel, event string) bool {
	switch event {
	case "ticket.created":
		return channel.NotifyOnNew
	case "ticket.updated":
		return channel.NotifyOnUpdate
	case "ticket.closed":
		return channel.NotifyOnClose
	default:
		return false
	}
}

// createTicketFromSlack creates a ticket from Slack
func (s *IntegrationService) createTicketFromSlack(subject, slackUserID, channelID string) (string, error) {
	// Map Slack user to GOTRS user
	// Create ticket
	// This would integrate with ticket service
	return fmt.Sprintf("Ticket created with subject: %s", subject), nil
}

// viewTicketFromSlack views a ticket from Slack
func (s *IntegrationService) viewTicketFromSlack(ticketID, slackUserID string) (string, error) {
	// Get ticket details
	// Format for Slack
	// This would integrate with ticket service
	return fmt.Sprintf("Ticket #%s details", ticketID), nil
}

// listTicketsFromSlack lists tickets from Slack
func (s *IntegrationService) listTicketsFromSlack(slackUserID string) (string, error) {
	// Get user's tickets
	// Format for Slack
	// This would integrate with ticket service
	return "Your open tickets list", nil
}

// Utility methods

func (s *IntegrationService) getPriorityName(priorityID int) string {
	priorities := map[int]string{
		1: "Low",
		2: "Normal", 
		3: "High",
		4: "Urgent",
		5: "Critical",
	}
	return priorities[priorityID]
}

func (s *IntegrationService) getStatusName(stateID int) string {
	states := map[int]string{
		1: "New",
		2: "Open",
		3: "Pending",
		4: "Resolved",
		5: "Closed",
	}
	return states[stateID]
}

func (s *IntegrationService) getQueueName(queueID int) string {
	// This would fetch from queue repository
	return fmt.Sprintf("Queue %d", queueID)
}

// Configuration conversion methods

func (s *IntegrationService) slackConfigToMap(config *SlackConfig) map[string]interface{} {
	data, _ := json.Marshal(config)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result
}

func (s *IntegrationService) mapToSlackConfig(data map[string]interface{}) *SlackConfig {
	jsonData, _ := json.Marshal(data)
	var config SlackConfig
	json.Unmarshal(jsonData, &config)
	return &config
}

func (s *IntegrationService) teamsConfigToMap(config *TeamsConfig) map[string]interface{} {
	data, _ := json.Marshal(config)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result
}

func (s *IntegrationService) mapToTeamsConfig(data map[string]interface{}) *TeamsConfig {
	jsonData, _ := json.Marshal(data)
	var config TeamsConfig
	json.Unmarshal(jsonData, &config)
	return &config
}

// loadActiveIntegrations loads active integrations from repository
func (s *IntegrationService) loadActiveIntegrations() {
	integrations, err := s.integrationRepo.GetActiveIntegrations()
	if err != nil {
		return
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for _, integration := range integrations {
		s.activeIntegrations[integration.Type] = integration
	}
}