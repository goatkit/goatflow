package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/goatkit/goatflow/internal/platform/i18n"
	"regexp"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	pluginmgr "github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/repository"
)

// SysconfigSetupCompleted is the sysconfig key that records whether the first-run
// setup wizard has been completed. Seeded to "false" by migration 000026.
const SysconfigSetupCompleted = "setup.assistant.completed"

// SystemSnapshot is the "recce" result: live counts of each core entity plus
// whether first-run setup has been marked complete. Drives both wizard gating
// (Mode 1) and the assistant dashboard (Mode 2).
type SystemSnapshot struct {
	Groups         int  `json:"groups"`
	Queues         int  `json:"queues"`
	Agents         int  `json:"agents"`
	Customers      int  `json:"customers"`
	SLAs           int  `json:"slas"`
	Roles          int  `json:"roles"`
	SetupCompleted bool `json:"setup_completed"`
}

// GroupInput defines a group to create.
type GroupInput struct {
	Name     string `json:"name" form:"name"`
	Comments string `json:"comments,omitempty" form:"comments"`
}

// QueueInput defines a queue to create. GroupIDs are 1-based indices into
// WizardRequest.Groups when submitted through the first-run wizard (the groups
// do not have database IDs yet); ExecuteWizard resolves them to real IDs. When
// CreateQueue is called directly (task handlers / API on an existing system),
// GroupIDs must be real database group IDs.
type QueueInput struct {
	Name     string `json:"name" form:"name"`
	GroupIDs []int  `json:"group_ids,omitempty" form:"group_ids"`
	Comments string `json:"comments,omitempty" form:"comments"`
}

// AgentInput defines an agent (system user) to create. GroupIDs follow the same
// resolution rule as QueueInput.GroupIDs.
type AgentInput struct {
	Login     string `json:"login" form:"login"`
	FirstName string `json:"first_name" form:"first_name"`
	LastName  string `json:"last_name" form:"last_name"`
	Email     string `json:"email,omitempty" form:"email"`
	GroupIDs  []int  `json:"group_ids,omitempty" form:"group_ids"`
}

// CustomerInput defines a customer company to create.
type CustomerInput struct {
	CustomerID string            `json:"customer_id" form:"customer_id"`
	Name       string            `json:"name" form:"name"`
	Fields     map[string]string `json:"fields,omitempty" form:"fields"`
}

// SLAInput defines an SLA to create. Times are in minutes.
type SLAInput struct {
	Name              string `json:"name" form:"name"`
	FirstResponseTime int    `json:"first_response_time" form:"first_response_time"`
	SolutionTime      int    `json:"solution_time" form:"solution_time"`
}

// ResponseTemplateInput defines a canned response template for customer support.
type ResponseTemplateInput struct {
	Name        string   `json:"name" form:"name"`                           // Display name (required)
	Shortcut    string   `json:"shortcut,omitempty" form:"shortcut"`         // Quick access code
	Category    string   `json:"category,omitempty" form:"category"`         // For organization
	Content     string   `json:"content" form:"content"`                     // Response body (required)
	ContentType string   `json:"content_type,omitempty" form:"content_type"` // text/plain or text/html
	Tags        []string `json:"tags,omitempty" form:"tags"`                 // Search tags
}

// BusinessHoursInput defines working hours configuration for SLA calculations.
type BusinessHoursInput struct {
	Name        string `json:"name" form:"name"`                   // Configuration name
	Timezone    string `json:"timezone,omitempty" form:"timezone"` // e.g., "America/New_York"
	Description string `json:"description,omitempty" form:"description"`
}

// EmailTransportInput configures outbound SMTP for the system.
type EmailTransportInput struct {
	Host     string `json:"host" form:"host"`                     // SMTP server host (required)
	Port     int    `json:"port" form:"port"`                     // 25, 465, 587 etc.
	User     string `json:"user,omitempty" form:"user"`           // Username for auth
	Password string `json:"password,omitempty" form:"password"`   // Password (stored securely)
	AuthType string `json:"auth_type,omitempty" form:"auth_type"` // plain, login, crammd5
	TLS      bool   `json:"tls,omitempty" form:"tls"`             // Enable TLS
	TLSMode  string `json:"tls_mode,omitempty" form:"tls_mode"`   // none, starttls, smtps
	From     string `json:"from,omitempty" form:"from"`           // Default from address
	FromName string `json:"from_name,omitempty" form:"from_name"` // Display name for sender
}

// WizardRequest is the single structured payload the wizard (UI or LLM) submits.
// Submitting everything at once makes the flow idempotent and API-friendly.
type WizardRequest struct {
	OrgType            string                  `json:"org_type" form:"org_type"`
	Groups             []GroupInput            `json:"groups" form:"groups"`
	Queues             []QueueInput            `json:"queues" form:"queues"`
	Agents             []AgentInput            `json:"agents" form:"agents"`
	CustomerCompanies  []CustomerInput         `json:"customer_companies" form:"customer_companies"`
	SLAs               []SLAInput              `json:"slas" form:"slas"`
	ResponseTemplates  []ResponseTemplateInput `json:"response_templates,omitempty" form:"response_templates"`
	BusinessHours      []BusinessHoursInput    `json:"business_hours,omitempty" form:"business_hours"`
	MailTransport      *EmailTransportInput    `json:"mail_transport,omitempty" form:"mail_transport"`
	ExistingCustomerID string                  `json:"existing_customer_id,omitempty" form:"existing_customer_id"`
	CreateBy           int                     `json:"-" form:"-"`
}

// CreatedEntity records one entity ExecuteWizard successfully created, so a
// partial failure still reports what succeeded.
type CreatedEntity struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	ID   int    `json:"id,omitempty"`
}

// WizardResult is the outcome of ExecuteWizard.
type WizardResult struct {
	Success bool            `json:"success"`
	Created []CreatedEntity `json:"created,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// PluginSetupTask is one entry in the setup task catalog. Core tasks use
// Plugin == "setup-assistant"; plugin-contributed tasks carry the plugin name.
type PluginSetupTask struct {
	Plugin      string `json:"plugin"`
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Category    string `json:"category"`
	Handler     string `json:"handler"`
}

// SetupAssistantService is the business-logic core shared by the HTML and JSON
// handlers. All entity creation goes through existing repositories; no SQL is
// built in handlers or templates.
type SetupAssistantService struct {
	db        *sql.DB
	groupRepo *repository.GroupSQLRepository
	queueRepo *repository.QueueRepository
	userRepo  *repository.UserRepository
	emailRepo *repository.EmailAccountRepository
	pluginMgr *pluginmgr.Manager
}

// NewSetupAssistantService wires the service with its repositories and the
// plugin manager (nil mgr is allowed — plugin task discovery simply returns none).
func NewSetupAssistantService(db *sql.DB, mgr *pluginmgr.Manager) *SetupAssistantService {
	return &SetupAssistantService{
		db:        db,
		groupRepo: repository.NewGroupRepository(db),
		queueRepo: repository.NewQueueRepository(db),
		userRepo:  repository.NewUserRepository(db),
		emailRepo: repository.NewEmailAccountRepository(db),
		pluginMgr: mgr,
	}
}

// Recce returns the current system snapshot. Each count is independent and
// defaults to 0 on error (missing table / query failure) so a partial schema
// never breaks the wizard gating — mirrors handleAdminDashboard's pattern.
func (s *SetupAssistantService) Recce(ctx context.Context) (*SystemSnapshot, error) {
	snap := &SystemSnapshot{}
	if s.db == nil {
		return snap, errors.New("database unavailable")
	}

	count := func(query string) int {
		var n int
		if err := s.db.QueryRowContext(ctx, database.ConvertPlaceholders(query)).Scan(&n); err != nil {
			return 0
		}
		return n
	}

	snap.Groups = count("SELECT COUNT(*) FROM groups WHERE valid_id = 1")
	snap.Queues = count("SELECT COUNT(*) FROM queue WHERE valid_id = 1")
	snap.Agents = count("SELECT COUNT(*) FROM users WHERE valid_id = 1")
	snap.Customers = count("SELECT COUNT(*) FROM customer_company WHERE valid_id = 1")
	snap.SLAs = count("SELECT COUNT(*) FROM sla WHERE valid_id = 1")
	snap.Roles = count("SELECT COUNT(*) FROM roles WHERE valid_id = 1")
	snap.SetupCompleted = s.IsSetupComplete(ctx)

	return snap, nil
}

// IsSetupComplete reads the setup.assistant.completed sysconfig flag.
func (s *SetupAssistantService) IsSetupComplete(ctx context.Context) bool {
	val, ok := s.readSysconfig(ctx, SysconfigSetupCompleted)
	if !ok {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(strings.Trim(val, "\"'")))
	return v == "true" || v == "1"
}

// MarkSetupComplete sets the setup.assistant.completed flag to "true". Uses an
// UPDATE-then-INSERT sequence that works on both MySQL and PostgreSQL without
// depending on a unique constraint.
func (s *SetupAssistantService) MarkSetupComplete(ctx context.Context) error {
	return s.writeSysconfig(ctx, SysconfigSetupCompleted, "true")
}

// CreateGroup creates a group and returns its new ID.
func (s *SetupAssistantService) CreateGroup(ctx context.Context, name, comments string, createBy int) (int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, errors.New("group name is required")
	}
	if len(name) > 255 {
		return 0, errors.New("group name too long (max 255 characters)")
	}
	if createBy <= 0 {
		createBy = 1
	}
	group := &models.Group{
		Name:     name,
		Comments: strings.TrimSpace(comments),
		ValidID:  1,
		CreateBy: createBy,
		ChangeBy: createBy,
	}
	if err := s.groupRepo.Create(group); err != nil {
		if isDuplicateErr(err) {
			return 0, fmt.Errorf("group %q already exists", name)
		}
		return 0, fmt.Errorf("create group: %w", err)
	}
	return toIntID(group.ID), nil
}

// CreateQueue creates a queue and assigns it to the given groups. The first
// group becomes the queue's primary group_id (NOT NULL); all groups additionally
// receive queue_group access rows. groupIDs must be real database group IDs.
func (s *SetupAssistantService) CreateQueue(ctx context.Context, name string, groupIDs []int, comments string, createBy int) (int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, errors.New("queue name is required")
	}
	if len(name) > 200 {
		return 0, errors.New("queue name too long (max 200 characters)")
	}
	if len(groupIDs) == 0 {
		return 0, errors.New("queue requires at least one group")
	}
	if createBy <= 0 {
		createBy = 1
	}

	queue := &models.Queue{
		Name:     name,
		GroupID:  uint(groupIDs[0]),
		Comment:  strings.TrimSpace(comments),
		ValidID:  1,
		CreateBy: uint(createBy),
		ChangeBy: uint(createBy),
	}
	// The queue table's system_address_id / salutation_id / signature_id columns
	// are NOT NULL with no default; default them to the standard seed row (1),
	// which the minimal-data migration guarantees exists.
	if queue.SystemAddressID == 0 {
		queue.SystemAddressID = 1
	}
	if queue.SalutationID == 0 {
		queue.SalutationID = 1
	}
	if queue.SignatureID == 0 {
		queue.SignatureID = 1
	}
	if queue.FollowUpID == 0 {
		queue.FollowUpID = 1
	}
	if err := s.queueRepo.Create(queue); err != nil {
		if isDuplicateErr(err) {
			return 0, fmt.Errorf("queue %q already exists", name)
		}
		return 0, fmt.Errorf("create queue: %w", err)
	}
	queueID := int(queue.ID)

	// Additional group access via the queue_group auxiliary table. Best-effort:
	// a minimal schema without queue_group still leaves the primary group_id set.
	for _, gid := range groupIDs {
		if gid <= 0 {
			continue
		}
		_, _ = s.db.ExecContext(ctx,
			database.ConvertPlaceholders("INSERT INTO queue_group (queue_id, group_id) VALUES (?, ?)"),
			queueID, gid) //nolint:errcheck // auxiliary table; absence is non-fatal
	}
	return queueID, nil
}

// CreateAgent creates a system user and grants rw access to the given groups
// via group_user. groupIDs must be real database group IDs.
func (s *SetupAssistantService) CreateAgent(ctx context.Context, login, firstName, lastName, email string, groupIDs []int, createBy int) (int, error) {
	login = strings.TrimSpace(login)
	if login == "" {
		return 0, errors.New("agent login is required")
	}
	if len(login) > 200 {
		return 0, errors.New("agent login too long (max 200 characters)")
	}
	if createBy <= 0 {
		createBy = 1
	}

	// Default password; admins reset via the users page. Hashed by the repo path
	// only if SetPassword is used — here we store a placeholder that cannot be
	// used to log in until reset.
	user := &models.User{
		Login:      login,
		Password:   "", // no usable credential until an admin sets one
		Title:      "Agent",
		FirstName:  strings.TrimSpace(firstName),
		LastName:   strings.TrimSpace(lastName),
		ValidID:    1,
		CreateTime: time.Now(),
		CreateBy:   createBy,
		ChangeTime: time.Now(),
		ChangeBy:   createBy,
	}
	if email != "" {
		// The users table has no email column; login doubles as the contact handle.
		if user.Login == "" {
			user.Login = email
		}
	}
	if err := s.userRepo.Create(user); err != nil {
		if isDuplicateErr(err) {
			return 0, fmt.Errorf("agent login %q already exists", login)
		}
		return 0, fmt.Errorf("create agent: %w", err)
	}
	userID := int(user.ID)

	// Grant rw group membership (the access the agent UI keys on).
	for _, gid := range groupIDs {
		if gid <= 0 {
			continue
		}
		if _, err := s.db.ExecContext(ctx,
			database.ConvertPlaceholders(`INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
				VALUES (?, ?, 'rw', NOW(), ?, NOW(), ?)`),
			userID, gid, createBy, createBy); err != nil {
			return userID, fmt.Errorf("agent created but group assignment failed for group %d: %w", gid, err)
		}
	}
	return userID, nil
}

// AssignAgentToGroups grants an existing agent (user) rw membership in the
// given teams. Idempotent per (user, group): an existing 'rw' row is left
// untouched. Returns an error if the agent does not exist or is inactive.
func (s *SetupAssistantService) AssignAgentToGroups(ctx context.Context, userID int, groupIDs []int, createBy int) error {
	if userID <= 0 {
		return errors.New("agent is required")
	}
	if len(groupIDs) == 0 {
		return errors.New("at least one team is required")
	}
	if createBy <= 0 {
		createBy = 1
	}

	var exists int
	if err := s.db.QueryRowContext(ctx, database.ConvertPlaceholders(
		"SELECT 1 FROM users WHERE id = ? AND valid_id = 1"), userID).Scan(&exists); err != nil {
		return errors.New("agent not found")
	}

	for _, gid := range groupIDs {
		if gid <= 0 {
			continue
		}
		var mapped int
		err := s.db.QueryRowContext(ctx, database.ConvertPlaceholders(
			"SELECT 1 FROM group_user WHERE user_id = ? AND group_id = ? AND permission_key = 'rw'"), userID, gid).Scan(&mapped)
		if err == nil {
			continue // already a member with rw
		}
		if _, err := s.db.ExecContext(ctx, database.ConvertPlaceholders(`
			INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'rw', NOW(), ?, NOW(), ?)`), userID, gid, createBy, createBy); err != nil {
			return fmt.Errorf("assign agent to team %d: %w", gid, err)
		}
	}
	return nil
}

// AssignQueueToGroup grants an existing team ownership/access of an existing
// queue. In the canonical GoatFlow schema a queue is owned by a single team via
// queue.group_id (users gain queue access by belonging to that team), so the
// first team supplied becomes the queue's owner. Idempotent.
func (s *SetupAssistantService) AssignQueueToGroup(ctx context.Context, queueID int, groupID int, createBy int) error {
	if queueID <= 0 {
		return errors.New("queue is required")
	}
	if groupID <= 0 {
		return errors.New("team is required")
	}
	if createBy <= 0 {
		createBy = 1
	}

	var exists int
	if err := s.db.QueryRowContext(ctx, database.ConvertPlaceholders(
		"SELECT 1 FROM queue WHERE id = ? AND valid_id = 1"), queueID).Scan(&exists); err != nil {
		return errors.New("queue not found")
	}
	_, err := s.db.ExecContext(ctx, database.ConvertPlaceholders(
		"UPDATE queue SET group_id = ?, change_time = NOW(), change_by = ? WHERE id = ?"),
		groupID, createBy, queueID)
	if err != nil {
		return fmt.Errorf("grant team %d queue access: %w", groupID, err)
	}
	return nil
}

// CreateCustomerCompany inserts a customer_company row. Fields carries optional
// columns (street, zip, city, country, url, comments).
func (s *SetupAssistantService) CreateCustomerCompany(ctx context.Context, customerID, name string, fields map[string]string, createBy int) error {
	customerID = strings.TrimSpace(customerID)
	name = strings.TrimSpace(name)
	if customerID == "" {
		return errors.New("customer_id is required")
	}
	if name == "" {
		return errors.New("customer name is required")
	}
	if len(customerID) > 200 {
		return errors.New("customer_id too long (max 200 characters)")
	}
	if createBy <= 0 {
		createBy = 1
	}

	field := func(key string) string {
		if v, ok := fields[key]; ok {
			return v
		}
		return ""
	}
	_, err := s.db.ExecContext(ctx, database.ConvertPlaceholders(`
		INSERT INTO customer_company (
			customer_id, name, street, zip, city, country, url, comments,
			valid_id, create_time, create_by, change_time, change_by
		) VALUES (
			?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''),
			NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''),
			1, NOW(), ?, NOW(), ?
		)`),
		customerID, name,
		field("street"), field("zip"), field("city"),
		field("country"), field("url"), field("comments"),
		createBy, createBy)
	if err != nil {
		if isDuplicateErr(err) {
			return fmt.Errorf("customer %q already exists", customerID)
		}
		return fmt.Errorf("create customer company: %w", err)
	}
	return nil
}

// CustomerUserInput defines one portal user to provision during onboarding.
type CustomerUserInput struct {
	Login     string `json:"login" form:"login"`
	Email     string `json:"email" form:"email"`
	FirstName string `json:"first_name" form:"first_name"`
	LastName  string `json:"last_name" form:"last_name"`
	Title     string `json:"title,omitempty" form:"title"`
	Phone     string `json:"phone,omitempty" form:"phone"`
	Mobile    string `json:"mobile,omitempty" form:"mobile"`
}

// MailAccountInput configures an inbound mailbox (OTRS mail_account) whose
// fetched messages land in a queue — used to wire up a customer's support
// email address during onboarding.
type MailAccountInput struct {
	Login              string `json:"login" form:"login"`                                           // mailbox username / email
	Password           string `json:"password" form:"password"`                                     // mailbox password (stored as-is, matching existing admin UI)
	Host               string `json:"host" form:"host"`                                             // IMAP/POP3 host
	AccountType        string `json:"account_type" form:"account_type"`                             // IMAP, IMAPTLS, IMAPS, POP3, POP3S
	QueueID            int    `json:"queue_id" form:"queue_id"`                                     // existing queue fetched mail lands in
	CreateQueueName    string `json:"create_queue_name,omitempty" form:"create_queue_name"`         // if set + QueueID==0, create a new queue
	CreateQueueGroupID int    `json:"create_queue_group_id,omitempty" form:"create_queue_group_id"` // owning team for a created queue
	// CreateQueueUseNewTeam: own the created queue with the team created earlier
	// in this onboarding wizard (no id yet at request time).
	CreateQueueUseNewTeam bool   `json:"create_queue_use_new_team,omitempty" form:"create_queue_use_new_team"`
	IMAPFolder            string `json:"imap_folder,omitempty" form:"imap_folder"`
	Trusted               bool   `json:"trusted,omitempty" form:"trusted"`
	Comments              string `json:"comments,omitempty" form:"comments"`
}

// OnboardCustomerRequest is the full payload for the customer onboarding wizard:
// company identity + address + the initial portal users to provision.
type OnboardCustomerRequest struct {
	CustomerID string              `json:"customer_id" form:"customer_id"`
	Name       string              `json:"name" form:"name"`
	Street     string              `json:"street,omitempty" form:"street"`
	City       string              `json:"city,omitempty" form:"city"`
	ZIP        string              `json:"zip,omitempty" form:"zip"`
	Country    string              `json:"country,omitempty" form:"country"`
	URL        string              `json:"url,omitempty" form:"url"`
	Comments   string              `json:"comments,omitempty" form:"comments"`
	Users      []CustomerUserInput `json:"users,omitempty" form:"users"`
	// ManagingGroupIDs: agent teams granted read-write access to this company
	// (OTRS group_customer) — i.e. which teams manage this customer.
	ManagingGroupIDs []int `json:"managing_group_ids,omitempty" form:"managing_group_ids"`
	// CreateManagingGroupName: if set, create a new team with this name (e.g.
	// derived from the customer id) and add it to the managing teams.
	CreateManagingGroupName string `json:"create_managing_group_name,omitempty" form:"create_managing_group_name"`
	// Optional Service/SLA mapping (OTRS service → service_sla → service_customer_user).
	ServiceName string `json:"service_name,omitempty" form:"service_name"`
	SLAID       int    `json:"sla_id,omitempty" form:"sla_id"`
	// Optional inbound mailbox (OTRS mail_account) wired to a destination queue.
	MailAccount *MailAccountInput `json:"mail_account,omitempty" form:"mail_account"`
}

// OnboardedUser records one provisioned portal user and its one-time password.
type OnboardedUser struct {
	Login    string `json:"login"`
	Email    string `json:"email"`
	Password string `json:"password,omitempty"`
}

// OnboardCustomerResult is the outcome of OnboardCustomer.
type OnboardCustomerResult struct {
	Success          bool            `json:"success"`
	CustomerID       string          `json:"customer_id"`
	CompanyName      string          `json:"company_name"`
	UsersCreated     []OnboardedUser `json:"users_created"`
	ManagingGroupIDs []int           `json:"managing_group_ids,omitempty"`
	ServiceID        int             `json:"service_id,omitempty"`
	ServiceName      string          `json:"service_name,omitempty"`
	SLALinked        bool            `json:"sla_linked,omitempty"`
	CreatedGroupID   int             `json:"created_group_id,omitempty"`
	CreatedGroupName string          `json:"created_group_name,omitempty"`
	MailAccountID    int             `json:"mail_account_id,omitempty"`
	CreatedQueueID   int             `json:"created_queue_id,omitempty"`
	Error            string          `json:"error,omitempty"`
}

// OnboardCustomer provisions a full customer setup in one shot: creates the
// company, then each initial portal user (with a generated temporary password
// the admin distributes). On any failure it reports what succeeded and stops.
// If CustomerID is empty it is derived from Name via SuggestCustomerID.
func (s *SetupAssistantService) OnboardCustomer(ctx context.Context, req OnboardCustomerRequest) *OnboardCustomerResult {
	res := &OnboardCustomerResult{UsersCreated: []OnboardedUser{}}
	if s.db == nil {
		res.Error = "database unavailable"
		return res
	}
	// abort records a failure and signals the caller to stop the chain.
	abort := func(err error) bool {
		if err != nil {
			res.Error = err.Error()
			return true
		}
		return false
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		res.Error = "company name is required"
		return res
	}
	cid := strings.TrimSpace(req.CustomerID)
	if cid == "" {
		cid = SuggestCustomerID(name)
	}
	if cid == "" {
		res.Error = "could not derive a customer id from the company name"
		return res
	}
	if len(cid) > 200 {
		res.Error = "customer_id too long (max 200 characters)"
		return res
	}

	const createBy = 1

	// 1. Company record.
	if abort(s.CreateCustomerCompany(ctx, cid, name, map[string]string{
		"street": req.Street, "zip": req.ZIP, "city": req.City,
		"country": req.Country, "url": req.URL, "comments": req.Comments,
	}, createBy)) {
		return res
	}
	res.CustomerID = cid
	res.CompanyName = name

	// 2. Managing agent teams (OTRS group_customer) — read-write access to the
	// company. Optionally create a new team first (e.g. one named after the customer).
	groupIDs := append([]int(nil), req.ManagingGroupIDs...)
	if newTeam := strings.TrimSpace(req.CreateManagingGroupName); newTeam != "" {
		gid, err := s.CreateGroup(ctx, newTeam, "", createBy)
		if abort(err) {
			return res
		}
		groupIDs = append(groupIDs, gid)
		res.CreatedGroupID = gid
		res.CreatedGroupName = newTeam
	}
	if len(groupIDs) > 0 {
		if abort(s.AssignGroupsToCustomer(ctx, cid, groupIDs, createBy)) {
			return res
		}
		res.ManagingGroupIDs = groupIDs
	}

	// 3. Portal users, each with a generated temporary password.
	if abort(s.provisionCustomerUsers(ctx, cid, req.Users, createBy, res)) {
		return res
	}

	// 4. Optional Service/SLA mapping (OTRS service → service_sla → service_customer_user).
	if req.SLAID > 0 {
		if abort(s.provisionCustomerSLA(ctx, name, req.ServiceName, req.SLAID, createBy, res)) {
			return res
		}
	}

	// 5. Optional inbound mailbox (OTRS mail_account) wired to a destination queue.
	if req.MailAccount != nil {
		if abort(s.provisionMailAccount(ctx, req.MailAccount, req.ManagingGroupIDs, createBy, res)) {
			return res
		}
	}

	res.Success = true
	return res
}

// provisionCustomerUsers creates each portal user with a generated temp
// password, appending to res.UsersCreated. Skips rows with an empty login.
func (s *SetupAssistantService) provisionCustomerUsers(ctx context.Context, customerID string, users []CustomerUserInput, createBy int, res *OnboardCustomerResult) error {
	hasher := auth.NewPasswordHasher()
	for _, u := range users {
		login := strings.TrimSpace(u.Login)
		if login == "" {
			continue
		}
		email := strings.TrimSpace(u.Email)
		if email == "" {
			email = login
		}
		pw, err := generateTempPassword()
		if err != nil {
			return fmt.Errorf("generate password: %w", err)
		}
		hash, err := hasher.HashPassword(pw)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		if err := s.createCustomerUser(ctx, login, email, customerID, hash, u, createBy); err != nil {
			return fmt.Errorf("user %q: %w", login, err)
		}
		res.UsersCreated = append(res.UsersCreated, OnboardedUser{Login: login, Email: email, Password: pw})
	}
	return nil
}

// provisionCustomerSLA creates a service, links the SLA to it, and grants the
// service to every already-provisioned user. Sets res.ServiceID/Name/SLALinked.
func (s *SetupAssistantService) provisionCustomerSLA(ctx context.Context, companyName, serviceName string, slaID, createBy int, res *OnboardCustomerResult) error {
	svcName := strings.TrimSpace(serviceName)
	if svcName == "" {
		svcName = companyName + " Service"
	}
	serviceID, err := s.CreateService(ctx, svcName, createBy)
	if err != nil {
		return err
	}
	res.ServiceID = serviceID
	res.ServiceName = svcName
	if err := s.LinkServiceSLA(ctx, serviceID, slaID); err != nil {
		return err
	}
	res.SLALinked = true
	for _, u := range res.UsersCreated {
		if err := s.AssignServiceToCustomerUser(ctx, u.Login, serviceID, createBy); err != nil {
			return fmt.Errorf("service assignment for %q: %w", u.Login, err)
		}
	}
	return nil
}

// provisionMailAccount resolves the destination queue (an existing one, or a
// newly-created queue owned by a managing team) and then creates the inbound
// mailbox wired to it. Sets res.MailAccountID and res.CreatedQueueID.
func (s *SetupAssistantService) provisionMailAccount(ctx context.Context, m *MailAccountInput, fallbackGroupIDs []int, createBy int, res *OnboardCustomerResult) error {
	queueID, created, err := s.resolveMailQueue(ctx, m, fallbackGroupIDs, res.CreatedGroupID, createBy)
	if err != nil {
		return err
	}
	m.QueueID = queueID
	id, err := s.CreateMailAccount(ctx, *m, createBy)
	if err != nil {
		return err
	}
	res.MailAccountID = id
	res.CreatedQueueID = created
	return nil
}

// resolveMailQueue returns the destination queue id for a mailbox: an explicit
// existing QueueID, or a freshly-created queue (owned by a managing team) when
// CreateQueueName is set. Returns (queueID, createdQueueID, error).
func (s *SetupAssistantService) resolveMailQueue(ctx context.Context, m *MailAccountInput, fallbackGroupIDs []int, newTeamID, createBy int) (int, int, error) {
	if m.QueueID > 0 {
		return m.QueueID, 0, nil
	}
	name := strings.TrimSpace(m.CreateQueueName)
	if name == "" {
		return 0, 0, errors.New("a destination queue is required for the mailbox (pick one or create one)")
	}
	// Owning team: explicit choice, then the team created earlier in this wizard
	// (if requested), then the first managing team as a fallback.
	gid := m.CreateQueueGroupID
	if gid <= 0 && m.CreateQueueUseNewTeam && newTeamID > 0 {
		gid = newTeamID
	}
	if gid <= 0 && len(fallbackGroupIDs) > 0 {
		gid = fallbackGroupIDs[0]
	}
	if gid <= 0 {
		return 0, 0, errors.New("creating a queue requires an owning team")
	}
	id, err := s.CreateQueue(ctx, name, []int{gid}, "", createBy)
	if err != nil {
		return 0, 0, fmt.Errorf("create queue for mailbox: %w", err)
	}
	return id, id, nil
}

// CreateMailAccount provisions an inbound mailbox via the existing
// EmailAccountRepository (no raw SQL in the service). Fetched messages land in
// the chosen queue. The password is stored as provided, matching the existing
// admin UI and the inbound poller (which reads `pw` directly).
func (s *SetupAssistantService) CreateMailAccount(ctx context.Context, m MailAccountInput, createBy int) (int, error) {
	login := strings.TrimSpace(m.Login)
	host := strings.TrimSpace(m.Host)
	if login == "" {
		return 0, errors.New("mailbox login is required")
	}
	if host == "" {
		return 0, errors.New("mailbox host is required")
	}
	if m.Password == "" {
		return 0, errors.New("mailbox password is required")
	}
	if m.QueueID <= 0 {
		return 0, errors.New("a destination queue is required for the mailbox")
	}
	acctType := strings.TrimSpace(m.AccountType)
	if acctType == "" {
		acctType = "IMAP"
	}
	var folderPtr *string
	if folder := strings.TrimSpace(m.IMAPFolder); folder != "" {
		folderPtr = &folder
	}
	return s.emailRepo.Create(&models.EmailAccount{
		Login:             login,
		PasswordEncrypted: m.Password,
		Host:              host,
		AccountType:       acctType,
		QueueID:           m.QueueID,
		Trusted:           m.Trusted,
		IMAPFolder:        folderPtr,
		ValidID:           1,
		CreatedBy:         createBy,
		UpdatedBy:         createBy,
	})
}

// createCustomerUser inserts a customer_user row linked to a company.
func (s *SetupAssistantService) createCustomerUser(ctx context.Context, login, email, customerID, hashedPW string, u CustomerUserInput, createBy int) error {
	_, err := s.db.ExecContext(ctx, database.ConvertPlaceholders(`
		INSERT INTO customer_user (
			login, email, customer_id, pw, title, first_name, last_name,
			phone, fax, mobile, street, zip, city, country, comments,
			valid_id, create_time, change_time, create_by, change_by
		) VALUES (
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?,
			1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?, ?
		)`),
		login, email, customerID, hashedPW, u.Title, u.FirstName, u.LastName,
		u.Phone, "", u.Mobile, "", "", "", "", "",
		createBy, createBy)
	if err != nil {
		if isDuplicateErr(err) {
			return fmt.Errorf("login %q already exists", login)
		}
		return fmt.Errorf("insert customer user: %w", err)
	}
	return nil
}

// CreateService inserts an OTRS `service` row and returns its new id. Used by
// the onboarding flow to give a customer SLA coverage the OTRS way (service →
// service_sla → service_customer_user).
func (s *SetupAssistantService) CreateService(ctx context.Context, name string, createBy int) (int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, errors.New("service name is required")
	}
	if len(name) > 200 {
		return 0, errors.New("service name too long (max 200 characters)")
	}
	if createBy <= 0 {
		createBy = 1
	}
	query := `
		INSERT INTO service (name, valid_id, comments, create_time, create_by, change_time, change_by)
		VALUES (?, 1, '', NOW(), ?, NOW(), ?)
		RETURNING id`
	execQuery, useLastInsert := database.ConvertReturning(query)
	execQuery = database.ConvertPlaceholders(execQuery)
	if useLastInsert && database.IsMySQL() {
		res, err := s.db.ExecContext(ctx, execQuery, name, createBy, createBy)
		if err != nil {
			if isDuplicateErr(err) {
				return 0, fmt.Errorf("service %q already exists", name)
			}
			return 0, fmt.Errorf("create service: %w", err)
		}
		last, err := res.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("create service: %w", err)
		}
		return int(last), nil
	}
	var id int
	if err := s.db.QueryRowContext(ctx, execQuery, name, createBy, createBy).Scan(&id); err != nil {
		if isDuplicateErr(err) {
			return 0, fmt.Errorf("service %q already exists", name)
		}
		return 0, fmt.Errorf("create service: %w", err)
	}
	return id, nil
}

// LinkServiceSLA links an SLA to a service via the OTRS `service_sla` table.
// Idempotent: a duplicate link is not an error (INSERT IGNORE / ON CONFLICT).
func (s *SetupAssistantService) LinkServiceSLA(ctx context.Context, serviceID, slaID int) error {
	if serviceID <= 0 || slaID <= 0 {
		return errors.New("service_id and sla_id are required")
	}
	var err error
	if database.IsMySQL() {
		_, err = s.db.ExecContext(ctx,
			database.ConvertPlaceholders("INSERT IGNORE INTO service_sla (service_id, sla_id) VALUES (?, ?)"),
			serviceID, slaID)
	} else {
		_, err = s.db.ExecContext(ctx,
			database.ConvertPlaceholders("INSERT INTO service_sla (service_id, sla_id) VALUES (?, ?) ON CONFLICT (service_id, sla_id) DO NOTHING"),
			serviceID, slaID)
	}
	if err != nil && !isDuplicateErr(err) {
		return fmt.Errorf("link service/SLA: %w", err)
	}
	return nil
}

// CreateCannedResponses creates multiple canned response templates for customer onboarding.
func (s *SetupAssistantService) CreateCannedResponses(ctx context.Context, templates []ResponseTemplateInput, createBy int) error {
	if len(templates) == 0 {
		return nil // skippable - nothing to do
	}

	for _, t := range templates {
		cr := &models.CannedResponse{
			Name: strings.TrimSpace(t.Name), Shortcut: strings.TrimSpace(t.Shortcut), Category: strings.TrimSpace(t.Category),
			Content: t.Content, ContentType: "text/html", Tags: t.Tags, IsActive: true,
			OwnerID: uint(createBy), CreatedBy: uint(createBy), UpdatedBy: uint(createBy),
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if cr.Name == "" {
			return fmt.Errorf("response name is required")
		}
		if cr.Content == "" {
			return fmt.Errorf("response content is required for %q", cr.Name)
		}

		query := database.ConvertPlaceholders(`INSERT INTO canned_response (name, category, content, content_type, tags, scope, owner_id, valid_id, create_time, create_by, change_time, change_by) VALUES (?, ?, ?, ?, ?, 'personal', ?, 1, NOW(), ?, NOW(), ?)`)
		tagsJSON, _ := json.Marshal(cr.Tags)
		_, err := s.db.ExecContext(ctx, query, cr.Name, cr.Category, cr.Content, cr.ContentType, string(tagsJSON), cr.OwnerID, createBy, createBy)
		if err != nil {
			return fmt.Errorf("creating response %q: %w", cr.Name, err)
		}
	}
	return nil
}

// CreateEmailTransport configures outbound SMTP settings via sysconfig.
func (s *SetupAssistantService) CreateEmailTransport(ctx context.Context, cfg EmailTransportInput, createBy int) error {
	if strings.TrimSpace(cfg.Host) == "" {
		return errors.New("SMTP host is required")
	}

	sysconfigKey := "Email.SMTP.Config"
	configJSON := fmt.Sprintf(`{"host":"%s","port":%d,"user":"%s","auth_type":"%s","tls":%v,"tls_mode":"%s","from":"%s","from_name":"%s"}`, cfg.Host, cfg.Port, strings.TrimSpace(cfg.User), strings.TrimSpace(cfg.AuthType), cfg.TLS, cfg.TLSMode, strings.TrimSpace(cfg.From), strings.TrimSpace(cfg.FromName))

	return s.writeSysconfig(ctx, sysconfigKey, configJSON)
}

// CreateBusinessHours creates a business hours calendar for SLA calculations.
func (s *SetupAssistantService) CreateBusinessHours(ctx context.Context, cfg BusinessHoursInput, createBy int) error {
	if strings.TrimSpace(cfg.Name) == "" {
		return errors.New("business hours name is required")
	}

	query := `INSERT INTO calendar (group_id, name, salt_string, color, ticket_appointments, valid_id, create_time, create_by, change_time, change_by) VALUES (?, ?, UUID(), '#3498db', '{}', 1, NOW(), ?, NOW(), ?)`
	_, err := s.db.ExecContext(ctx, database.ConvertPlaceholders(query), createBy, cfg.Name, createBy, createBy)
	if err != nil {
		return fmt.Errorf("creating business hours calendar %q: %w", cfg.Name, err)
	}

	sysconfigKey := "BusinessHours." + cfg.Name + ".Timezone"
	return s.writeSysconfig(ctx, sysconfigKey, cfg.Timezone)
}

// AssignServiceToCustomerUser grants a service to a customer user by login via
// the OTRS `service_customer_user` table.
func (s *SetupAssistantService) AssignServiceToCustomerUser(ctx context.Context, login string, serviceID, createBy int) error {
	login = strings.TrimSpace(login)
	if login == "" || serviceID <= 0 {
		return errors.New("login and service_id are required")
	}
	if createBy <= 0 {
		createBy = 1
	}
	_, err := s.db.ExecContext(ctx, database.ConvertPlaceholders(`
		INSERT INTO service_customer_user (customer_user_login, service_id, create_time, create_by)
		VALUES (?, ?, NOW(), ?)`),
		login, serviceID, createBy)
	if err != nil && !isDuplicateErr(err) {
		return fmt.Errorf("assign service to customer user: %w", err)
	}
	return nil
}

// LoadExistingCustomer loads an existing customer company and its associated configuration
// for review and modification in the wizard.
func (s *SetupAssistantService) LoadExistingCustomer(ctx context.Context, customerID string) (*CustomerConfiguration, error) {
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return nil, errors.New("customer_id is required")
	}

	config := &CustomerConfiguration{}

	// Load customer company details
	row := s.db.QueryRowContext(ctx,
		"SELECT name, street, zip, city, country, url, comments FROM customer_company WHERE customer_id = ?",
		customerID)

	var customer CustomerCompany
	var street, zip, city, country, urlVal, comments sql.NullString
	err := row.Scan(&customer.Name, &street, &zip, &city, &country, &urlVal, &comments)
	if err == nil {
		customer.Street = street.String
		customer.Zip = zip.String
		customer.City = city.String
		customer.Country = country.String
		customer.Url = urlVal.String
		customer.Comments = comments.String
	}
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("customer %q not found", customerID)
		}
		return nil, fmt.Errorf("load customer: %w", err)
	}

	config.Customer = &customer

	// Load associated groups
	groups, err := s.loadCustomerGroups(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("load groups: %w", err)
	}
	config.Groups = groups

	// Load associated queues
	queues, err := s.loadCustomerQueues(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("load queues: %w", err)
	}
	config.Queues = queues

	// Load associated agents/users
	agents, err := s.loadCustomerAgents(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("load agents: %w", err)
	}
	config.Agents = agents

	// Load mail accounts
	mailAccounts, err := s.loadCustomerMailAccounts(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("load mail accounts: %w", err)
	}
	config.MailAccounts = mailAccounts

	// Load services and SLAs
	services, slas, err := s.loadCustomerServicesAndSLAs(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("load services/SLAs: %w", err)
	}
	config.Services = services
	config.SLAs = slas

	// Load canned responses (simplified - just names and counts)
	responses, err := s.loadCustomerResponses(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("load responses: %w", err)
	}
	config.CannedResponses = responses

	// Load business hours
	hours, err := s.loadCustomerBusinessHours(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("load business hours: %w", err)
	}
	config.BusinessHours = hours

	// Load email transport config
	transport, err := s.loadEmailTransportConfig(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("load email transport: %w", err)
	}
	config.EmailTransport = transport

	// Load portal users (customer_user table)
	portalUsers, err := s.loadCustomerPortalUsers(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("load portal users: %w", err)
	}
	config.PortalUsers = portalUsers

	return config, nil
}

// CustomerConfiguration holds all configuration for an existing customer
type CustomerConfiguration struct {
	Customer        *CustomerCompany         `json:"customer"`
	Groups          []GroupWithID            `json:"groups"`
	Queues          []QueueWithGroupIDs      `json:"queues"`
	Agents          []AgentInputWithGroupIDs `json:"agents"`
	MailAccounts    []MailAccountInput       `json:"mail_accounts"`
	Services        []string                 `json:"services"`
	SLAs            []SLAInput               `json:"slas"`
	PortalUsers     []PortalUserInfo         `json:"portal_users"`
	CannedResponses []string                 `json:"canned_responses"`
	BusinessHours   []BusinessHoursInput     `json:"business_hours"`
	EmailTransport  *EmailTransportInput     `json:"mail_transport"`
}

// PortalUserInfo holds a customer portal user record for the config response.
type PortalUserInfo struct {
	Login     string `json:"login"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone,omitempty"`
	Title     string `json:"title,omitempty"`
}

// CustomerCompany holds customer company details
type CustomerCompany struct {
	Name     string `json:"name"`
	Street   string `json:"street"`
	Zip      string `json:"zip"`
	City     string `json:"city"`
	Country  string `json:"country"`
	Url      string `json:"url"`
	Comments string `json:"comments"`
}

// GroupWithID holds group name with ID from DB
type GroupWithID struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

// QueueWithGroupIDs holds queue with group assignments
type QueueWithGroupIDs struct {
	Name     string `json:"name"`
	GroupIDs []int  `json:"group_ids"`
}

// AgentInputWithGroupIDs extends AgentInput with ID and group assignments
type AgentInputWithGroupIDs struct {
	Login     string `json:"login"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	GroupIDs  []int  `json:"group_ids"`
}

// Helper methods to load customer associations
func (s *SetupAssistantService) loadCustomerGroups(ctx context.Context, customerID string) ([]GroupWithID, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT g.id, g.name FROM group_customer gc JOIN groups g ON gc.group_id = g.id WHERE gc.customer_id = ?",
		customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []GroupWithID
	for rows.Next() {
		var g GroupWithID
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func (s *SetupAssistantService) loadCustomerQueues(ctx context.Context, customerID string) ([]QueueWithGroupIDs, error) {
	// Queues are linked to customers through groups: queue.group_id → group_customer.group_id → customer_id
	rows, err := s.db.QueryContext(ctx,
		"SELECT DISTINCT q.id, q.name FROM queue q "+
			"JOIN group_customer gc ON q.group_id = gc.group_id "+
			"WHERE gc.customer_id = ? AND q.valid_id = 1",
		customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queues []QueueWithGroupIDs
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		queues = append(queues, QueueWithGroupIDs{Name: name})
	}
	return queues, rows.Err()
}

func (s *SetupAssistantService) loadCustomerAgents(ctx context.Context, customerID string) ([]AgentInputWithGroupIDs, error) {
	// Agents are not directly linked to customers via a simple join.
	// Return empty list - agent association is through queues/groups.
	return nil, nil
}

func (s *SetupAssistantService) loadCustomerMailAccounts(ctx context.Context, customerID string) ([]MailAccountInput, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT ma.login, ma.pw, ma.host, ma.account_type, ma.queue_id, ma.trusted, ma.imap_folder, ma.comments "+
			"FROM mail_account ma "+
			"JOIN queue q ON ma.queue_id = q.id JOIN group_customer gc ON q.group_id = gc.group_id "+
			"WHERE gc.customer_id = ?",
		customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []MailAccountInput
	for rows.Next() {
		var account MailAccountInput
		var pw, comments, imapFolder sql.NullString
		var trusted int
		if err := rows.Scan(
			&account.Login, &pw, &account.Host, &account.AccountType,
			&account.QueueID, &trusted, &imapFolder, &comments,
		); err != nil {
			return nil, err
		}
		account.Password = pw.String
		account.Trusted = trusted != 0
		account.IMAPFolder = imapFolder.String
		account.Comments = comments.String
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func (s *SetupAssistantService) loadCustomerServicesAndSLAs(ctx context.Context, customerID string) ([]string, []SLAInput, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT s.name FROM service s JOIN service_customer_user scu ON s.id = scu.service_id JOIN customer_user cu ON scu.customer_user_login = cu.login WHERE cu.customer_id = ?",
		customerID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var services []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, nil, err
		}
		services = append(services, name)
	}

	return services, []SLAInput{}, nil
}

func (s *SetupAssistantService) loadCustomerResponses(ctx context.Context, customerID string) ([]string, error) {
	// canned_response is not customer-scoped in the current schema.
	// Return empty list — responses are global in OTRS model.
	return nil, nil

}

func (s *SetupAssistantService) loadCustomerBusinessHours(ctx context.Context, customerID string) ([]BusinessHoursInput, error) {
	// business_hours table doesn't exist in current schema.
	// Return empty list — calendars are global, not customer-scoped.
	return nil, nil

}

func (s *SetupAssistantService) loadEmailTransportConfig(ctx context.Context, customerID string) (*EmailTransportInput, error) {
	// This would query sysconfig or a dedicated email_config table
	// For now, return nil (no config loaded)
	return nil, nil
}

// AssignGroupsToCustomer grants agent teams (groups) read-write access to a
// customer company via the OTRS `group_customer` table — i.e. which teams can
// see and manage this customer's tickets. Idempotent per (customer, group).
func (s *SetupAssistantService) AssignGroupsToCustomer(ctx context.Context, customerID string, groupIDs []int, createBy int) error {
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return errors.New("customer_id is required")
	}
	if createBy <= 0 {
		createBy = 1
	}
	for _, gid := range groupIDs {
		if gid <= 0 {
			continue
		}
		_, err := s.db.ExecContext(ctx, database.ConvertPlaceholders(`
			INSERT INTO group_customer (customer_id, group_id, permission_key, permission_value, permission_context, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'rw', 1, 'Ticket', NOW(), ?, NOW(), ?)`),
			customerID, gid, createBy, createBy)
		if err != nil && !isDuplicateErr(err) {
			return fmt.Errorf("assign group %d to customer: %w", gid, err)
		}
	}
	return nil
}

// loadCustomerPortalUsers loads customer_user records for a given customer.
func (s *SetupAssistantService) loadCustomerPortalUsers(ctx context.Context, customerID string) ([]PortalUserInfo, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT login, email, first_name, last_name, phone, title FROM customer_user WHERE customer_id = ? AND valid_id = 1 ORDER BY login",
		customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]PortalUserInfo, 0)
	for rows.Next() {
		var u PortalUserInfo
		var phone, title sql.NullString
		if err := rows.Scan(&u.Login, &u.Email, &u.FirstName, &u.LastName, &phone, &title); err != nil {
			return nil, err
		}
		u.Phone = phone.String
		u.Title = title.String
		users = append(users, u)
	}
	return users, rows.Err()
}

// suggestIDRe collapses runs of separators in a derived customer id.
var suggestIDRe = regexp.MustCompile(`-{2,}`)

// SuggestCustomerID derives a customer_id slug from a company name: uppercased,
// alphanumerics only, spaces/underscores/slashes collapsed to single hyphens,
// trimmed and capped at 50 chars. The admin may refine it before submitting.
func SuggestCustomerID(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	upper := strings.ToUpper(name)
	var b strings.Builder
	for _, r := range upper {
		switch {
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '-' || r == '/':
			b.WriteRune('-')
		}
	}
	s := strings.Trim(suggestIDRe.ReplaceAllString(b.String(), "-"), "-")
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}

// generateTempPassword returns a random 12-char URL-safe password. Stored only
// as a hash; the plaintext is returned once for the admin to distribute.
func generateTempPassword() (string, error) {
	b := make([]byte, 9)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// CreateSLA inserts an SLA row and returns its new ID. Times are in minutes.
func (s *SetupAssistantService) CreateSLA(ctx context.Context, name string, firstResponseTime, solutionTime int, createBy int) (int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, errors.New("sla name is required")
	}
	if len(name) > 255 {
		return 0, errors.New("sla name too long (max 255 characters)")
	}
	if firstResponseTime < 0 || solutionTime < 0 {
		return 0, errors.New("sla times must be non-negative")
	}
	if createBy <= 0 {
		createBy = 1
	}

	query := database.ConvertPlaceholders(`
		INSERT INTO sla (
			name, calendar_name, first_response_time, first_response_notify,
			update_time, update_notify, solution_time, solution_notify,
			valid_id, comments, create_time, create_by, change_time, change_by
		) VALUES (?, 'Default', ?, 0, 0, 0, ?, 0, 1, '', NOW(), ?, NOW(), ?)
		RETURNING id`)

	// MySQL has no RETURNING — strip it and use LastInsertId when on MySQL.
	execQuery, useLastInsert := database.ConvertReturning(query)
	var id int
	if useLastInsert && database.IsMySQL() {
		res, err := s.db.ExecContext(ctx, execQuery, name, firstResponseTime, solutionTime, createBy, createBy)
		if err != nil {
			if isDuplicateErr(err) {
				return 0, fmt.Errorf("sla %q already exists", name)
			}
			return 0, fmt.Errorf("create sla: %w", err)
		}
		last, err := res.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("create sla: %w", err)
		}
		id = int(last)
	} else {
		if err := s.db.QueryRowContext(ctx, execQuery, name, firstResponseTime, solutionTime, createBy, createBy).Scan(&id); err != nil {
			if isDuplicateErr(err) {
				return 0, fmt.Errorf("sla %q already exists", name)
			}
			return 0, fmt.Errorf("create sla: %w", err)
		}
	}
	return id, nil
}

// ExecuteWizard creates every entity in WizardRequest in dependency order:
// groups → queues (queue_group) → agents (group_user) → customers → SLAs. On
// any failure it returns a WizardResult describing what succeeded and what
// failed. It is not wrapped in a single transaction (entity creation spans
// several repositories); on a fresh system partial state is re-runnable.
func (s *SetupAssistantService) ExecuteWizard(ctx context.Context, req WizardRequest) *WizardResult {
	res := &WizardResult{Created: []CreatedEntity{}}
	if s.db == nil {
		res.Error = "database unavailable"
		return res
	}
	createBy := req.CreateBy
	if createBy <= 0 {
		createBy = 1
	}

	// 1. Groups. Track created IDs in submission order so queues/agents can
	// reference them by 1-based index.
	createdGroupIDs := make([]int, 0, len(req.Groups))
	for _, g := range req.Groups {
		id, err := s.CreateGroup(ctx, g.Name, g.Comments, createBy)
		if err != nil {
			res.Error = fmt.Sprintf("group %q: %v", g.Name, err)
			return res
		}
		createdGroupIDs = append(createdGroupIDs, id)
		res.Created = append(res.Created, CreatedEntity{Kind: "group", Name: g.Name, ID: id})
	}
	// resolveGroups maps wizard group references to real DB IDs. A reference in
	// [1, len(createdGroupIDs)] is a 1-based index into the just-created groups;
	// any other positive value is treated as a literal existing DB group ID.
	resolveGroups := func(refs []int) []int {
		out := make([]int, 0, len(refs))
		for _, r := range refs {
			if r >= 1 && r <= len(createdGroupIDs) {
				out = append(out, createdGroupIDs[r-1])
			} else if r > 0 {
				out = append(out, r)
			}
		}
		return out
	}

	// 2. Queues (+ queue_group assignments).
	for _, q := range req.Queues {
		id, err := s.CreateQueue(ctx, q.Name, resolveGroups(q.GroupIDs), q.Comments, createBy)
		if err != nil {
			res.Error = fmt.Sprintf("queue %q: %v", q.Name, err)
			return res
		}
		res.Created = append(res.Created, CreatedEntity{Kind: "queue", Name: q.Name, ID: id})
	}

	// 3. Agents (+ group_user assignments).
	for _, a := range req.Agents {
		id, err := s.CreateAgent(ctx, a.Login, a.FirstName, a.LastName, a.Email, resolveGroups(a.GroupIDs), createBy)
		if err != nil {
			res.Error = fmt.Sprintf("agent %q: %v", a.Login, err)
			return res
		}
		res.Created = append(res.Created, CreatedEntity{Kind: "agent", Name: a.Login, ID: id})
	}

	// 4. Customer companies.
	for _, c := range req.CustomerCompanies {
		if err := s.CreateCustomerCompany(ctx, c.CustomerID, c.Name, c.Fields, createBy); err != nil {
			res.Error = fmt.Sprintf("customer %q: %v", c.CustomerID, err)
			return res
		}
		res.Created = append(res.Created, CreatedEntity{Kind: "customer", Name: c.CustomerID})
	}

	// 5. SLAs.
	for _, sl := range req.SLAs {
		id, err := s.CreateSLA(ctx, sl.Name, sl.FirstResponseTime, sl.SolutionTime, createBy)
		if err != nil {
			res.Error = fmt.Sprintf("sla %q: %v", sl.Name, err)
			return res
		}
		res.Created = append(res.Created, CreatedEntity{Kind: "sla", Name: sl.Name, ID: id})
	}

	// 6. Canned response templates.
	if len(req.ResponseTemplates) > 0 {
		if err := s.CreateCannedResponses(ctx, req.ResponseTemplates, createBy); err != nil {
			res.Error = fmt.Sprintf("canned responses: %v", err)
			return res
		}
		res.Created = append(res.Created, CreatedEntity{Kind: "canned_response", Name: "templates"})
	}
	// 7. Business hours.
	for _, bh := range req.BusinessHours {
		if err := s.CreateBusinessHours(ctx, bh, createBy); err != nil {
			res.Error = fmt.Sprintf("business hours %q: %v", bh.Name, err)
			return res
		}
		res.Created = append(res.Created, CreatedEntity{Kind: "business_hours", Name: bh.Name})
	}
	// 8. Email transport.
	if req.MailTransport != nil {
		if err := s.CreateEmailTransport(ctx, *req.MailTransport, createBy); err != nil {
			res.Error = fmt.Sprintf("email transport: %v", err)
			return res
		}
		res.Created = append(res.Created, CreatedEntity{Kind: "mail_transport", Name: req.MailTransport.Host})
	}

	res.Success = true
	return res
}

// GetCoreTasks returns the built-in setup task catalog. These dispatch to the
// core HTML/API task handlers, not to plugins.
func (s *SetupAssistantService) GetCoreTasks() []PluginSetupTask {
	return []PluginSetupTask{
		{Plugin: "setup-assistant", ID: "create_group", Title: "Create a team", Category: "teams", Handler: "create_group", Icon: "fa-users", Description: "Add a team or department that owns queues and tickets."},
		{Plugin: "setup-assistant", ID: "create_queue", Title: "Create a queue", Category: "queues", Handler: "create_queue", Icon: "fa-inbox", Description: "Add a ticket queue and grant teams access to it."},
		{Plugin: "setup-assistant", ID: "assign_queue_group", Title: "Grant queue access", Category: "queues", Handler: "assign_queue_group", Icon: "fa-lock", Description: "Give an existing team access to an existing queue."},
		{Plugin: "setup-assistant", ID: "create_agent", Title: "Add an agent", Category: "teams", Handler: "create_agent", Icon: "fa-user-plus", Description: "Create a system user and assign them to teams."},
		{Plugin: "setup-assistant", ID: "assign_agent_group", Title: "Assign agent to team", Category: "teams", Handler: "assign_agent_group", Icon: "fa-user-tag", Description: "Add an existing agent to an existing team."},
		{Plugin: "setup-assistant", ID: "create_customer", Title: "Onboard a customer", Category: "customers", Handler: "create_customer", Icon: "fa-building", Description: "Create a customer company, provision portal users, and generate temporary passwords in one go."},
		{Plugin: "setup-assistant", ID: "create_sla", Title: "Configure an SLA", Category: "sla", Handler: "create_sla", Icon: "fa-clock", Description: "Define first-response and solution time targets."},
		{Plugin: "setup-assistant", ID: "create_response_template", Title: i18n.GetInstance().T("", "setup_assistant_tasks.create_response_template.title"), Description: i18n.GetInstance().T("", "setup_assistant_tasks.create_response_template.description")},
		{Plugin: "setup-assistant", ID: "configure_business_hours", Title: i18n.GetInstance().T("", "setup_assistant_tasks.configure_business_hours.title"), Description: i18n.GetInstance().T("", "setup_assistant_tasks.configure_business_hours.description")},
		{Plugin: "setup-assistant", ID: "configure_email_transport", Title: i18n.GetInstance().T("", "setup_assistant_tasks.configure_email_transport.title"), Description: i18n.GetInstance().T("", "setup_assistant_tasks.configure_email_transport.description")},
		{Plugin: "setup-assistant", ID: "mark_complete", Title: "Mark setup complete", Category: "system", Handler: "mark_complete", Icon: "fa-check", Description: "Dismiss the first-run wizard and stop the dashboard redirect."},
	}
}

// GetPluginTasks returns setup tasks contributed by registered plugins.
func (s *SetupAssistantService) GetPluginTasks() []PluginSetupTask {
	out := make([]PluginSetupTask, 0)
	if s.pluginMgr == nil {
		return out
	}
	for _, reg := range s.pluginMgr.List() {
		for _, t := range reg.SetupTasks {
			out = append(out, PluginSetupTask{
				Plugin:      reg.Name,
				ID:          t.ID,
				Title:       t.Title,
				Description: t.Description,
				Icon:        t.Icon,
				Category:    t.Category,
				Handler:     t.Handler,
			})
		}
	}
	return out
}

// GetAllTasks returns the combined core + plugin task catalog, grouped in
// catalog order (core first, then plugins).
func (s *SetupAssistantService) GetAllTasks() []PluginSetupTask {
	core := s.GetCoreTasks()
	plugins := s.GetPluginTasks()
	all := make([]PluginSetupTask, 0, len(core)+len(plugins))
	all = append(all, core...)
	all = append(all, plugins...)
	return all
}

// CallPluginTask dispatches a plugin setup task by invoking its handler via the
// plugin manager. Returns the handler's raw JSON response.
func (s *SetupAssistantService) CallPluginTask(ctx context.Context, pluginName, taskID string, args []byte) ([]byte, error) {
	if s.pluginMgr == nil {
		return nil, errors.New("plugin manager unavailable")
	}
	handler, err := s.pluginTaskHandler(pluginName, taskID)
	if err != nil {
		return nil, err
	}
	return s.pluginMgr.Call(ctx, pluginName, handler, args)
}

// pluginTaskHandler resolves the handler function name for a plugin task.
func (s *SetupAssistantService) pluginTaskHandler(pluginName, taskID string) (string, error) {
	for _, reg := range s.pluginMgr.List() {
		if reg.Name != pluginName {
			continue
		}
		for _, t := range reg.SetupTasks {
			if t.ID == taskID {
				return t.Handler, nil
			}
		}
	}
	return "", fmt.Errorf("plugin %q has no setup task %q", pluginName, taskID)
}

// --- sysconfig helpers (the service cannot reach api.sysconfigValue, so it
// reads/writes sysconfig directly via parameterised SQL). ---

func (s *SetupAssistantService) readSysconfig(ctx context.Context, name string) (string, bool) {
	if s.db == nil || strings.TrimSpace(name) == "" {
		return "", false
	}
	var value sql.NullString

	err := s.db.QueryRowContext(ctx, database.ConvertPlaceholders(`
        SELECT effective_value FROM sysconfig_modified
        WHERE name = ? AND is_valid = 1
        ORDER BY change_time DESC LIMIT 1`), name).Scan(&value)
	if err == nil && value.Valid {
		return value.String, true
	}

	err = s.db.QueryRowContext(ctx, database.ConvertPlaceholders(`
        SELECT effective_value FROM sysconfig_default WHERE name = ?`), name).Scan(&value)
	if err != nil || !value.Valid {
		return "", false
	}
	return value.String, true
}

func (s *SetupAssistantService) writeSysconfig(ctx context.Context, name, value string) error {
	if s.db == nil {
		return errors.New("database unavailable")
	}
	// Try UPDATE first (works when the sysconfig_default row already exists).
	res, err := s.db.ExecContext(ctx,
		database.ConvertPlaceholders("UPDATE sysconfig_default SET effective_value = ? WHERE name = ?"),
		value, name)
	if err != nil {
		return fmt.Errorf("update sysconfig: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	// INSERT with all NOT NULL columns required by the schema.
	// On duplicate key, fall back to UPDATE (race between concurrent callers).
	_, err = s.db.ExecContext(ctx,
		database.ConvertPlaceholders(`INSERT INTO sysconfig_default
			(name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
			 has_configlevel, user_modification_possible, user_modification_active,
			 effective_value, xml_content_raw, xml_content_parsed, xml_filename,
			 is_dirty, exclusive_lock_guid,
			 create_time, create_by, change_time, change_by)
			VALUES (?, '', '', 1, 1, 0, 1, 0, 0, 0, ?, '', '', '', 0, '', NOW(), ?, NOW(), ?)`),
		name, value, 1, 1)
	if err != nil {
		if isDuplicateErr(err) {
			// Row was inserted by a concurrent caller — update it instead.
			_, err = s.db.ExecContext(ctx,
				database.ConvertPlaceholders("UPDATE sysconfig_default SET effective_value = ? WHERE name = ?"),
				value, name)
			if err != nil {
				return fmt.Errorf("update sysconfig after duplicate: %w", err)
			}
			return nil
		}
		return fmt.Errorf("insert sysconfig: %w", err)
	}
	return nil
}

// --- shared helpers ---

// isDuplicateErr recognises a unique-constraint violation across drivers and
// repository error wrapping, matching the heuristic in admin_groups_handlers.
func isDuplicateErr(err error) bool {
	if err == nil {
		return false
	}
	low := strings.ToLower(err.Error())
	return strings.Contains(low, "duplicate") ||
		strings.Contains(low, "exists") ||
		strings.Contains(low, "unique") ||
		strings.Contains(low, "23505") // postgres unique_violation
}

// toIntID coerces a model identifier (interface{} on Group.ID) to int.
func toIntID(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case uint:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}
