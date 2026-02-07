package api

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/shared"
	"github.com/goatkit/goatflow/internal/ticketutil"
)

// queryInt safely parses an integer query parameter with a default value.
// Returns the parsed value or defaultVal if parsing fails.
func queryInt(c *gin.Context, param string, defaultVal int) int {
	if val, err := strconv.Atoi(c.DefaultQuery(param, strconv.Itoa(defaultVal))); err == nil {
		return val
	}
	return defaultVal
}

// TicketDisplay represents ticket data for display purposes.
type TicketDisplay struct {
	models.Ticket
	QueueName    string
	PriorityName string
	StateName    string
	OwnerName    string
	CustomerName string
}

const pendingAutoStateTypeID = 5
const autoCloseNoTimeLabel = "No auto-close time scheduled"
const pendingReminderStateTypeID = 4
const pendingReminderNoTimeLabel = "No reminder time scheduled"

// Kept for backwards compatibility - new code should use shared.GetGlobalRenderer() directly.
func getPongo2Renderer() *shared.TemplateRenderer {
	return shared.GetGlobalRenderer()
}

type permissionDefinition struct {
	Key         string
	Label       string
	Description string
}

type groupPermissionsGroup struct {
	ID         uint
	Name       string
	Comments   string
	ValidID    int
	QueueCount int
}

type groupPermissionMember struct {
	ID          uint
	Login       string
	FirstName   string
	LastName    string
	Permissions map[string]bool
}

type groupPermissionsQueue struct {
	ID        uint
	Name      string
	Comment   string
	ValidID   int
	UserCount int
}

type groupPermissionsData struct {
	Group   groupPermissionsGroup
	Members []groupPermissionMember
	Queues  []groupPermissionsQueue
}

var groupPermissionDefinitions = []permissionDefinition{
	{Key: string(repository.PermissionRO), Label: "RO", Description: "Read tickets"},
	{Key: string(repository.PermissionMoveInto), Label: "Move", Description: "Move tickets into queue"},
	{Key: string(repository.PermissionCreate), Label: "Create", Description: "Create tickets"},
	{Key: string(repository.PermissionNote), Label: "Note", Description: "Add notes"},
	{Key: string(repository.PermissionOwner), Label: "Owner", Description: "Take ownership"},
	{Key: string(repository.PermissionPriority), Label: "Priority", Description: "Update priority"},
	{Key: string(repository.PermissionRW), Label: "RW", Description: "Full access"},
}

var groupPermissionKeys = []string{
	string(repository.PermissionRO),
	string(repository.PermissionMoveInto),
	string(repository.PermissionCreate),
	string(repository.PermissionNote),
	string(repository.PermissionOwner),
	string(repository.PermissionPriority),
	string(repository.PermissionRW),
}

func humanizeDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d == 0 {
		return "0s"
	}
	if d < 0 {
		d = -d
	}
	hours := int(d / time.Hour)
	minutes := int(d%time.Hour) / int(time.Minute)
	seconds := int(d%time.Minute) / int(time.Second)
	parts := make([]string, 0, 3)
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 && hours == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}
	return strings.Join(parts, " ")
}

func defaultGroupPermissionMap() map[string]bool {
	perms := make(map[string]bool, len(groupPermissionKeys))
	for _, key := range groupPermissionKeys {
		perms[key] = false
	}
	return perms
}

func normalizeGroupPermissionMap(input map[string]bool) map[string]bool {
	perms := defaultGroupPermissionMap()
	for key, value := range input {
		if _, ok := perms[key]; ok {
			perms[key] = value
		}
	}
	return perms
}

func wantsJSONResponse(c *gin.Context) bool {
	if strings.EqualFold(c.GetHeader("HX-Request"), "true") {
		return true
	}
	if strings.EqualFold(c.GetHeader("X-Requested-With"), "XMLHttpRequest") {
		return true
	}
	accept := strings.ToLower(c.GetHeader("Accept"))
	return strings.Contains(accept, "application/json")
}

func stubGroupPermissionsData(groupID uint) *groupPermissionsData {
	return &groupPermissionsData{
		Group: groupPermissionsGroup{
			ID:       groupID,
			Name:     fmt.Sprintf("Group %d", groupID),
			Comments: "",
		},
		Members: []groupPermissionMember{},
		Queues:  []groupPermissionsQueue{},
	}
}

func fetchGroupPermissionsData(db *sql.DB, groupID uint) (*groupPermissionsData, error) {
	groupRepo := repository.NewGroupRepository(db)
	group, err := groupRepo.GetByID(groupID)
	if err != nil {
		return nil, err
	}

	queueRepo := repository.NewQueueRepository(db)
	queueList, err := queueRepo.List()
	if err != nil {
		return nil, err
	}

	queues := make([]groupPermissionsQueue, 0)
	for _, q := range queueList {
		if q == nil {
			continue
		}
		if q.GroupID != groupID {
			continue
		}
		queues = append(queues, groupPermissionsQueue{
			ID:      q.ID,
			Name:    q.Name,
			Comment: q.Comment,
			ValidID: q.ValidID,
		})
	}

	permRepo := repository.NewPermissionRepository(db)
	permissionMap, err := permRepo.GetGroupPermissions(groupID)
	if err != nil {
		return nil, err
	}

	userRepo := repository.NewUserRepository(db)
	members := make([]groupPermissionMember, 0, len(permissionMap))
	for userID, permKeys := range permissionMap {
		user, err := userRepo.GetByID(userID)
		if err != nil {
			continue
		}
		if user.ValidID != 1 {
			continue
		}
		perms := defaultGroupPermissionMap()
		for _, key := range permKeys {
			if _, ok := perms[key]; ok {
				perms[key] = true
			}
		}
		members = append(members, groupPermissionMember{
			ID:          user.ID,
			Login:       user.Login,
			FirstName:   user.FirstName,
			LastName:    user.LastName,
			Permissions: perms,
		})
	}

	sort.Slice(members, func(i, j int) bool {
		il := strings.ToLower(members[i].Login)
		jl := strings.ToLower(members[j].Login)
		if il == jl {
			return members[i].ID < members[j].ID
		}
		return il < jl
	})

	for idx := range queues {
		queues[idx].UserCount = len(members)
	}

	meta := groupPermissionsGroup{
		ID:         groupID,
		Name:       group.Name,
		Comments:   group.Comments,
		ValidID:    group.ValidID,
		QueueCount: len(queues),
	}

	return &groupPermissionsData{
		Group:   meta,
		Members: members,
		Queues:  queues,
	}, nil
}

func respondWithGroupPermissionsJSON(c *gin.Context, data *groupPermissionsData) {
	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"group":           data.Group,
		"members":         data.Members,
		"queues":          data.Queues,
		"permission_keys": groupPermissionDefinitions,
	})
}

func htmxHandlerSkipDB() bool {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("HTMX_HANDLER_TEST_MODE")))
	if mode == "1" || mode == "true" || mode == "yes" {
		return true
	}
	if strings.TrimSpace(os.Getenv("SKIP_DB_WAIT")) == "1" {
		return true
	}
	return false
}

func isPendingAutoState(stateName string, stateTypeID int) bool {
	if stateTypeID == pendingAutoStateTypeID {
		return true
	}
	normalized := strings.ReplaceAll(strings.ToLower(stateName), "-", " ")
	return strings.Contains(normalized, "pending auto")
}

func isPendingReminderState(stateName string, stateTypeID int) bool {
	if stateTypeID == pendingReminderStateTypeID {
		return true
	}
	normalized := strings.ReplaceAll(strings.ToLower(stateName), "-", " ")
	return strings.Contains(normalized, "pending reminder")
}

type autoCloseMeta struct {
	pending  bool
	at       string
	atISO    string
	relative string
	overdue  bool
}

func computeAutoCloseMeta(ticket *models.Ticket, stateName string, stateTypeID int, now time.Time) autoCloseMeta {
	meta := autoCloseMeta{}
	meta.pending = isPendingAutoState(stateName, stateTypeID)
	if ticket == nil {
		return meta
	}

	// Use the effective pending time - this handles legacy/migrated data with no date set
	// by defaulting to now + 24 hours
	if meta.pending || ticket.UntilTime > 0 {
		autoCloseAt := ticketutil.GetEffectivePendingTime(ticket.UntilTime)
		diff := autoCloseAt.Sub(now)
		meta.overdue = diff < 0
		if meta.overdue {
			meta.relative = humanizeDuration(-diff)
		} else {
			meta.relative = humanizeDuration(diff)
		}
		meta.at = autoCloseAt.Format("2006-01-02 15:04:05 UTC")
		meta.atISO = autoCloseAt.Format(time.RFC3339)
	}
	return meta
}

type pendingReminderMeta struct {
	pending  bool
	at       string
	atISO    string
	relative string
	overdue  bool
	hasTime  bool
	message  string
}

func computePendingReminderMeta(ticket *models.Ticket, stateName string, stateTypeID int, now time.Time) pendingReminderMeta {
	meta := pendingReminderMeta{}
	if ticket == nil {
		return meta
	}
	meta.pending = isPendingReminderState(stateName, stateTypeID)

	// Use the effective pending time - this handles legacy/migrated data with no date set
	// by defaulting to now + 24 hours
	if meta.pending || ticket.UntilTime > 0 {
		reminderAt := ticketutil.GetEffectivePendingTime(ticket.UntilTime)
		diff := reminderAt.Sub(now)
		meta.overdue = diff < 0
		meta.hasTime = ticket.UntilTime > 0 // Track if time was explicitly set
		if meta.overdue {
			meta.relative = humanizeDuration(-diff)
		} else {
			meta.relative = humanizeDuration(diff)
		}
		meta.at = reminderAt.Format("2006-01-02 15:04:05 UTC")
		meta.atISO = reminderAt.Format(time.RFC3339)
		if !meta.hasTime {
			meta.message = "Default reminder time (24h from now)"
		}
	}
	return meta
}

func hashPasswordSHA256(password, salt string) string { //nolint:unused
	h := sha256.New()
	h.Write([]byte(password + salt))
	return hex.EncodeToString(h.Sum(nil))
}

func generateSalt() string { //nolint:unused
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return time.Now().Format(time.RFC3339Nano)
	}
	return hex.EncodeToString(salt)
}

func verifyPassword(password, storedPassword string) bool { //nolint:unused
	if strings.HasPrefix(storedPassword, "$2a$") || strings.HasPrefix(storedPassword, "$2b$") || strings.HasPrefix(storedPassword, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password)) == nil
	}
	if strings.HasPrefix(storedPassword, "sha256$") {
		parts := strings.SplitN(storedPassword, "$", 3)
		if len(parts) == 3 {
			salt := parts[1]
			storedHash := parts[2]
			computedHash := hashPasswordSHA256(password, salt)
			return storedHash == computedHash
		}
	}
	return password == storedPassword
}

func isMarkdownContent(content string) bool {
	lines := strings.Split(content, "\n")

	if strings.Contains(content, "#") {
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") {
				i := 0
				for i < len(line) && line[i] == '#' {
					i++
				}
				if i < len(line) && line[i] == ' ' {
					return true
				}
			}
		}
	}

	if strings.Contains(content, "|") {
		tableLines := 0
		separatorFound := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "|") {
				tableLines++
				if strings.Contains(line, "|") && strings.Contains(line, "-") {
					separatorFound = true
				}
			}
		}
		if tableLines >= 2 && separatorFound {
			return true
		}
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 1 {
			if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
				return true
			}
			if len(line) > 2 && line[0] >= '0' && line[0] <= '9' && line[1] == '.' && line[2] == ' ' {
				return true
			}
		}
	}

	if strings.Contains(content, "](") && strings.Contains(content, ")") {
		return true
	}

	if strings.Contains(content, "**") || (strings.Contains(content, "*") && strings.Count(content, "*") >= 2) {
		return true
	}

	if strings.Contains(content, "`") {
		return true
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, ">") {
			return true
		}
	}

	return strings.Contains(content, "```")
}

var titleCaser = cases.Title(language.English)

func normalizeRoleString(role interface{}) string {
	raw := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", role)))
	switch raw {
	case "":
		return ""
	case "admin":
		return "Admin"
	case "agent":
		return "Agent"
	case "customer", "customer_user", "customer-user", "customeruser":
		return "Customer"
	case "guest":
		return "Guest"
	default:
		return titleCaser.String(raw)
	}
}

func isAdminRole(role string) bool {
	return strings.EqualFold(role, "Admin")
}

func toUintID(v interface{}) uint {
	return shared.ToUint(v, 0)
}

func isUserInAdminGroup(db *sql.DB, userID uint) bool {
	if db == nil || userID == 0 {
		return false
	}
	var cnt int
	err := db.QueryRow(database.ConvertPlaceholders(`
		SELECT COUNT(*)
		FROM group_user ug
		JOIN groups g ON ug.group_id = g.id
		WHERE ug.user_id = ? AND LOWER(g.name) = 'admin'`), userID).Scan(&cnt)
	return err == nil && cnt > 0
}

type userDetails struct {
	Login     string
	FirstName string
	LastName  string
	Title     string
}

func getUserDetailsFromDB(db *sql.DB, userID uint) userDetails {
	details := userDetails{}
	if db == nil || userID == 0 {
		return details
	}
	var dbLogin, dbFirst, dbLast, dbTitle sql.NullString
	err := db.QueryRow(database.ConvertPlaceholders(`
		SELECT login, first_name, last_name, title FROM users WHERE id = ?`), userID).Scan(&dbLogin, &dbFirst, &dbLast, &dbTitle)
	if err != nil {
		return details
	}
	if dbLogin.Valid {
		details.Login = dbLogin.String
	}
	if dbFirst.Valid {
		details.FirstName = dbFirst.String
	}
	if dbLast.Valid {
		details.LastName = dbLast.String
	}
	if dbTitle.Valid {
		details.Title = dbTitle.String
	}
	return details
}

func normalizeUserMapFields(m gin.H) gin.H {
	if roleVal, ok := m["Role"].(string); ok {
		nr := normalizeRoleString(roleVal)
		if nr == "" {
			nr = "Agent"
		}
		m["Role"] = nr
		if _, exists := m["IsAdmin"]; !exists {
			m["IsAdmin"] = isAdminRole(nr)
		}
		if _, exists := m["IsInAdminGroup"]; !exists && isAdminRole(nr) {
			m["IsInAdminGroup"] = true
		}
	}
	return m
}

func guestUserMap() gin.H {
	return gin.H{
		"ID":        0,
		"Login":     "guest",
		"FirstName": "",
		"LastName":  "",
		"Email":     "",
		"IsActive":  false,
		"IsAdmin":   false,
		"Role":      "Guest",
	}
}

// GetUserMapForTemplate exposes the internal user-context builder for reuse
// across packages without duplicating logic.
func GetUserMapForTemplate(c *gin.Context) gin.H {
	return getUserMapForTemplate(c)
}

func getUserMapForTemplate(c *gin.Context) gin.H {
	userCtx, ok := c.Get("user")
	if ok {
		if user, isModel := userCtx.(*models.User); isModel {
			return buildUserMapFromModel(user)
		}
		if userH, isGinH := userCtx.(gin.H); isGinH {
			normalized := gin.H{}
			for k, v := range userH {
				normalized[k] = v
			}
			return normalizeUserMapFields(normalized)
		}
		if userMap, isMap := userCtx.(map[string]any); isMap {
			normalized := gin.H{}
			for k, v := range userMap {
				normalized[k] = v
			}
			return normalizeUserMapFields(normalized)
		}
	}

	userID, ok := c.Get("user_id")
	if ok {
		return buildUserMapFromContext(c, userID)
	}

	return guestUserMap()
}

// computeInitials returns the initials from first and last name (e.g., "JD" for John Doe).
func computeInitials(firstName, lastName string) string {
	var initials string
	if len(firstName) > 0 {
		initials += strings.ToUpper(string([]rune(firstName)[0]))
	}
	if len(lastName) > 0 {
		initials += strings.ToUpper(string([]rune(lastName)[0]))
	}
	if initials == "" {
		return "?"
	}
	return initials
}

func buildUserMapFromModel(user *models.User) gin.H {
	isAdmin := user.ID == 1 || strings.Contains(strings.ToLower(user.Login), "admin")
	isInAdminGroup := false
	if db, err := database.GetDB(); err == nil && db != nil {
		isInAdminGroup = isUserInAdminGroup(db, user.ID)
	}
	return gin.H{
		"ID":             user.ID,
		"Login":          user.Login,
		"FirstName":      user.FirstName,
		"LastName":       user.LastName,
		"Initials":       computeInitials(user.FirstName, user.LastName),
		"Title":          user.Title,
		"Email":          user.Email,
		"IsActive":       user.ValidID == 1,
		"IsAdmin":        isAdmin,
		"IsInAdminGroup": isInAdminGroup,
		"Role":           map[bool]string{true: "Admin", false: "Agent"}[isAdmin],
	}
}

func buildUserMapFromContext(c *gin.Context, userID interface{}) gin.H {
	userEmail, _ := c.Get("user_email")
	userRole, _ := c.Get("user_role")
	normalizedRole := normalizeRoleString(userRole)
	if normalizedRole == "" {
		normalizedRole = "Agent"
	}

	login := fmt.Sprintf("%v", userEmail)
	firstName, lastName, title := "", "", ""
	isInAdminGroup := false
	userIDVal := toUintID(userID)

	if db, err := database.GetDB(); err == nil && db != nil {
		details := getUserDetailsFromDB(db, userIDVal)
		if details.Login != "" {
			login = details.Login
		}
		firstName = details.FirstName
		lastName = details.LastName
		title = details.Title
		isInAdminGroup = isUserInAdminGroup(db, userIDVal)
	}

	if firstName == "" && lastName == "" {
		userName, _ := c.Get("user_name")
		nameParts := strings.Fields(fmt.Sprintf("%v", userName))
		if len(nameParts) > 0 {
			firstName = nameParts[0]
		}
		if len(nameParts) > 1 {
			lastName = strings.Join(nameParts[1:], " ")
		}
	}

	return gin.H{
		"ID":             userID,
		"Login":          login,
		"FirstName":      firstName,
		"LastName":       lastName,
		"Initials":       computeInitials(firstName, lastName),
		"Title":          title,
		"Email":          login,
		"IsActive":       true,
		"IsAdmin":        isAdminRole(normalizedRole),
		"IsInAdminGroup": isInAdminGroup,
		"Role":           normalizedRole,
	}
}

func sendErrorResponse(c *gin.Context, statusCode int, message string) {
	log.Printf("sendErrorResponse: status=%d message=%s path=%s", statusCode, message, c.FullPath())
	if strings.Contains(c.GetHeader("Accept"), "application/json") ||
		strings.HasPrefix(c.Request.URL.Path, "/api/") ||
		c.GetHeader("HX-Request") == "true" {
		c.JSON(statusCode, gin.H{
			"success": false,
			"error":   message,
		})
		return
	}

	if getPongo2Renderer() != nil {
		getPongo2Renderer().HTML(c, statusCode, "pages/error.pongo2", pongo2.Context{
			"StatusCode": statusCode,
			"Message":    message,
			"User":       getUserMapForTemplate(c),
		})
	} else {
		c.String(statusCode, "Error: %s", message)
	}
}

func checkAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := getUserMapForTemplate(c)

		if userID, ok := user["ID"].(uint); ok {
			if userID == 1 || userID == 2 {
				c.Next()
				return
			}

			db, err := database.GetDB()
			if err == nil {
				var count int
				err = db.QueryRow(database.ConvertPlaceholders(`
					SELECT COUNT(*)
					FROM group_user ug
					JOIN groups g ON ug.group_id = g.id
					WHERE ug.user_id = ? AND LOWER(g.name) = 'admin'`),
					userID).Scan(&count)
				if err == nil && count > 0 {
					c.Next()
					return
				}
			}
		}

		if login, ok := user["Login"].(string); ok {
			if strings.Contains(strings.ToLower(login), "admin") || login == "root@localhost" {
				c.Next()
				return
			}
		}

		sendErrorResponse(c, http.StatusForbidden, "Access denied. Admin privileges required.")
		c.Abort()
	}
}
