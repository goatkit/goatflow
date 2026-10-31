package api

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/audit"
	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/platform/sysconfig"
)

// adminUsersDB returns the database handle, or answers 500 and reports false.
func adminUsersDB(c *gin.Context) (*sql.DB, bool) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("admin users: database unavailable: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return nil, false
	}
	return db, true
}

// adminUsersFail logs err and answers 500 with message.
func adminUsersFail(c *gin.Context, message string, err error) {
	log.Printf("admin users: %s: %v", message, err)
	c.JSON(http.StatusInternalServerError, gin.H{
		"success": false,
		"error":   message,
	})
}

// HandleAdminUsers renders the admin users management page.
func HandleAdminUsers(c *gin.Context) {
	fail := func(what string, err error) {
		log.Printf("admin users page: %s: %v", what, err)
		c.String(http.StatusInternalServerError, "Failed to load users")
	}

	renderer := shared.GetGlobalRenderer()
	if renderer == nil {
		fail("template renderer", errors.New("not initialised"))
		return
	}
	db, err := database.GetDB()
	if err != nil || db == nil {
		fail("database unavailable", err)
		return
	}

	type urow struct {
		id                   int
		login, title, fn, ln string
		valid                int
	}
	var list []urow
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, login, COALESCE(title,''), first_name, last_name, valid_id
		FROM users
		ORDER BY last_name, first_name, id`))
	if err != nil {
		fail("query users", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var r urow
		if err := rows.Scan(&r.id, &r.login, &r.title, &r.fn, &r.ln, &r.valid); err != nil {
			fail("scan user", err)
			return
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		fail("iterate users", err)
		return
	}

	// Group memberships for all users
	gm := map[int][]string{}
	gr, err := db.Query(database.ConvertPlaceholders(`
		SELECT DISTINCT gu.user_id, g.name
		FROM group_user gu
		JOIN groups g ON g.id = gu.group_id
		WHERE g.valid_id = 1`))
	if err != nil {
		fail("query group memberships", err)
		return
	}
	defer gr.Close()
	for gr.Next() {
		var uid int
		var gname string
		if err := gr.Scan(&uid, &gname); err != nil {
			fail("scan group membership", err)
			return
		}
		gm[uid] = append(gm[uid], gname)
	}
	if err := gr.Err(); err != nil {
		fail("iterate group memberships", err)
		return
	}

	// 2FA status for all users
	totp2fa := map[int]bool{}
	tr, err := db.Query(database.ConvertPlaceholders(`
		SELECT user_id FROM user_preferences
		WHERE preferences_key = 'UserTOTPEnabled'
		AND preferences_value = '1'`))
	if err != nil {
		fail("query 2FA status", err)
		return
	}
	defer tr.Close()
	for tr.Next() {
		var uid int
		if err := tr.Scan(&uid); err != nil {
			fail("scan 2FA status", err)
			return
		}
		totp2fa[uid] = true
	}
	if err := tr.Err(); err != nil {
		fail("iterate 2FA status", err)
		return
	}

	users := make([]gin.H, 0, len(list))
	for _, r := range list {
		users = append(users, gin.H{
			"ID":             r.id,
			"Login":          r.login,
			"Title":          r.title,
			"FirstName":      r.fn,
			"LastName":       r.ln,
			"ValidID":        r.valid,
			"Groups":         gm[r.id],
			"TOTP2FAEnabled": totp2fa[r.id],
		})
	}

	// All groups for filters and modal
	groups := make([]gin.H, 0)
	ar, err := db.Query(database.ConvertPlaceholders(`SELECT id, name FROM groups WHERE valid_id = 1 ORDER BY name`))
	if err != nil {
		fail("query groups", err)
		return
	}
	defer ar.Close()
	for ar.Next() {
		var id int
		var name string
		if err := ar.Scan(&id, &name); err != nil {
			fail("scan group", err)
			return
		}
		groups = append(groups, gin.H{"ID": id, "Name": name})
	}
	if err := ar.Err(); err != nil {
		fail("iterate groups", err)
		return
	}

	user := getUserMapForTemplate(c)
	isInAdminGroup := false
	if v, ok := user["IsInAdminGroup"].(bool); ok {
		isInAdminGroup = v
	}
	renderer.HTML(c, http.StatusOK, "pages/admin/users.pongo2", gin.H{
		"Title":          "Users",
		"Users":          users,
		"Groups":         groups,
		"User":           user,
		"IsInAdminGroup": isInAdminGroup,
		"ActivePage":     "admin",
	})
}

// HandleAdminUserGet handles GET /admin/users/:id.
func HandleAdminUserGet(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	db, ok := adminUsersDB(c)
	if !ok {
		return
	}

	// users.title is nullable (the seeded root@localhost agent has NULL).
	var (
		login, firstName, lastName string
		title                      sql.NullString
		validID                    int
	)
	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT login, title, first_name, last_name, valid_id
		FROM users
		WHERE id = ?`), id).Scan(&login, &title, &firstName, &lastName, &validID)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}
	if err != nil {
		adminUsersFail(c, "Failed to load user", err)
		return
	}

	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT DISTINCT g.id, g.name
		FROM groups g
		JOIN group_user gu ON g.id = gu.group_id
		WHERE gu.user_id = ? AND g.valid_id = 1`), id)
	if err != nil {
		adminUsersFail(c, "Failed to load user groups", err)
		return
	}
	defer rows.Close()
	groupNames := make([]string, 0)
	for rows.Next() {
		var gid int
		var gname string
		if err := rows.Scan(&gid, &gname); err != nil {
			adminUsersFail(c, "Failed to load user groups", err)
			return
		}
		groupNames = append(groupNames, gname)
	}
	if err := rows.Err(); err != nil {
		adminUsersFail(c, "Failed to load user groups", err)
		return
	}

	validXlat := "invalid"
	if validID == 1 {
		validXlat = "valid"
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":            id,
			"login":         login,
			"title":         title.String,
			"first_name":    firstName,
			"last_name":     lastName,
			"email":         login, // OTRS uses login as email
			"valid_id":      validID,
			"groups":        groupNames,
			"xlats":         gin.H{"valid_id": validXlat},
			"valid_id_xlat": validXlat,
		},
	})
}

// adminUserRequest is the body of the admin create/update user endpoints.
type adminUserRequest struct {
	Login     string `json:"login" form:"login"`
	Title     string `json:"title" form:"title"`
	FirstName string `json:"first_name" form:"first_name"`
	LastName  string `json:"last_name" form:"last_name"`
	Email     string `json:"email" form:"email"`
	Password  string `json:"password" form:"password"`
	// ConfirmPassword is checked when the client sends it (the admin form
	// always does); API clients may omit it.
	ConfirmPassword *string  `json:"confirm_password" form:"confirm_password"`
	ValidID         int      `json:"valid_id" form:"valid_id"`
	Groups          []string `json:"groups" form:"groups"`
}

// bindAdminUserRequest binds the body and reports whether the client
// submitted the groups field (an empty submitted selection clears memberships).
func bindAdminUserRequest(c *gin.Context) (adminUserRequest, bool, bool) {
	var req adminUserRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request data",
		})
		return req, false, false
	}
	// Some form encoders send groups[]; ShouldBind can miss repeated keys for urlencoded PUT.
	groupsSubmitted := req.Groups != nil
	if len(req.Groups) == 0 {
		if arr := c.PostFormArray("groups"); len(arr) > 0 {
			req.Groups = arr
			groupsSubmitted = true
		} else if arr := c.PostFormArray("groups[]"); len(arr) > 0 {
			req.Groups = arr
			groupsSubmitted = true
		}
	}
	if strings.TrimSpace(c.PostForm("groups_submitted")) == "1" {
		groupsSubmitted = true
	}
	return req, groupsSubmitted, true
}

// checkAdminSetPassword enforces the confirmation and the agent password
// policy (PreferencesGroups###Password, the policy agents' own password change
// applies) on a password an admin sets for an agent. It answers 400 and
// reports false when the password is rejected.
func checkAdminSetPassword(c *gin.Context, db *sql.DB, password string, confirm *string) bool {
	if confirm != nil && *confirm != password {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Passwords do not match",
			"code":    "mismatch",
		})
		return false
	}
	policy, err := sysconfig.LoadAgentPasswordPolicy(db)
	if err != nil {
		adminUsersFail(c, "Failed to load password policy", err)
		return false
	}
	if verr := policy.ValidatePassword(password); verr != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   passwordPolicyMessage(c, verr.Code, policy),
			"code":    verr.Code,
		})
		return false
	}
	return true
}

// passwordPolicyMessage renders a policy violation in the request language.
func passwordPolicyMessage(c *gin.Context, code string, policy sysconfig.PasswordPolicy) string {
	msg := middleware.T(c, sysconfig.PasswordRequirementKey(code))
	return strings.ReplaceAll(msg, "{n}", strconv.Itoa(policy.PasswordMinSize))
}

// errUnknownGroup marks a group token that names no valid group.
var errUnknownGroup = errors.New("unknown group")

// resolveAgentGroupIDs maps group IDs or names to valid group IDs.
func resolveAgentGroupIDs(db *sql.DB, tokens []string) ([]int, error) {
	ids := make([]int, 0, len(tokens))
	seen := map[int]bool{}
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		var groupID int
		var err error
		if n, convErr := strconv.Atoi(token); convErr == nil {
			err = db.QueryRow(database.ConvertPlaceholders(
				"SELECT id FROM groups WHERE id = ? AND valid_id = 1"), n).Scan(&groupID)
		} else {
			err = db.QueryRow(database.ConvertPlaceholders(
				"SELECT id FROM groups WHERE name = ? AND valid_id = 1"), token).Scan(&groupID)
		}
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s", errUnknownGroup, token)
		}
		if err != nil {
			return nil, err
		}
		if !seen[groupID] {
			seen[groupID] = true
			ids = append(ids, groupID)
		}
	}
	return ids, nil
}

// resolveAgentGroupsOrFail resolves the submitted groups, answering 400 for an
// unknown group and 500 for a database error.
func resolveAgentGroupsOrFail(c *gin.Context, db *sql.DB, tokens []string) ([]int, bool) {
	ids, err := resolveAgentGroupIDs(db, tokens)
	if errors.Is(err, errUnknownGroup) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return nil, false
	}
	if err != nil {
		adminUsersFail(c, "Failed to resolve groups", err)
		return nil, false
	}
	return ids, true
}

// HandleAdminUserCreate handles POST /admin/users.
func HandleAdminUserCreate(c *gin.Context) {
	req, _, ok := bindAdminUserRequest(c)
	if !ok {
		return
	}

	if req.Login == "" || req.FirstName == "" || req.LastName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Login, first name, and last name are required",
		})
		return
	}

	actorID, ok := auditUserID(c)
	if !ok {
		return
	}
	db, ok := adminUsersDB(c)
	if !ok {
		return
	}

	var exists bool
	if err := db.QueryRow(database.ConvertPlaceholders(
		"SELECT EXISTS(SELECT 1 FROM users WHERE login = ?)"), req.Login).Scan(&exists); err != nil {
		adminUsersFail(c, "Failed to check existing user", err)
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "User already exists"})
		return
	}

	var hashedPassword string
	if req.Password != "" {
		if !checkAdminSetPassword(c, db, req.Password, req.ConfirmPassword) {
			return
		}
		var err error
		hashedPassword, err = auth.NewPasswordHasher().HashPassword(req.Password)
		if err != nil {
			c.JSON(passwordHashErrorStatus(err), gin.H{
				"success": false,
				"error":   "Failed to hash password: " + err.Error(),
			})
			return
		}
	}

	groupIDs, ok := resolveAgentGroupsOrFail(c, db, req.Groups)
	if !ok {
		return
	}

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		adminUsersFail(c, "Failed to begin user create transaction", err)
		return
	}
	defer func() { _ = tx.Rollback() }()

	userID64, err := database.GetAdapter().InsertWithReturningTx(tx, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, title, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, NOW(), ?, NOW(), ?)
		RETURNING id`),
		req.Login, hashedPassword, req.Title, req.FirstName, req.LastName, req.ValidID, actorID, actorID)
	if err != nil {
		adminUsersFail(c, "Failed to create user", err)
		return
	}
	userID := int(userID64)

	for _, groupID := range groupIDs {
		if _, err := tx.Exec(database.ConvertPlaceholders(`
			INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'rw', NOW(), ?, NOW(), ?)`),
			userID, groupID, actorID, actorID); err != nil {
			adminUsersFail(c, "Failed to assign group", err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		adminUsersFail(c, "Failed to commit user create", err)
		return
	}

	sortedGroups := append([]int(nil), groupIDs...)
	sort.Ints(sortedGroups)
	fields := map[string]any{
		"login":      req.Login,
		"title":      req.Title,
		"first_name": req.FirstName,
		"last_name":  req.LastName,
		"valid_id":   req.ValidID,
		"groups":     sortedGroups,
	}
	if req.Password != "" {
		fields["password"] = "(set)"
	}
	audit.RecordRequest(c, audit.Event{
		Category:    audit.CategoryAdmin,
		Action:      "admin.user.create",
		Outcome:     audit.OutcomeSuccess,
		TargetType:  "user",
		TargetID:    audit.ID(userID64),
		TargetLabel: req.Login,
		Details:     map[string]any{"fields": audit.Snapshot(fields)},
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User created successfully",
		"user_id": userID,
	})
}

// HandleAdminUserUpdate handles PUT /admin/users/:id.
func HandleAdminUserUpdate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	req, groupsSubmitted, ok := bindAdminUserRequest(c)
	if !ok {
		return
	}

	actorID, ok := auditUserID(c)
	if !ok {
		return
	}
	db, ok := adminUsersDB(c)
	if !ok {
		return
	}

	before, err := loadAgentAuditFields(db, id)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}
	if err != nil {
		adminUsersFail(c, "Failed to load user", err)
		return
	}

	var loginTaken bool
	if err := db.QueryRow(database.ConvertPlaceholders(
		"SELECT EXISTS(SELECT 1 FROM users WHERE login = ? AND id <> ?)"), req.Login, id).Scan(&loginTaken); err != nil {
		adminUsersFail(c, "Failed to check existing user", err)
		return
	}
	if loginTaken {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "User already exists"})
		return
	}

	var hash string
	if req.Password != "" {
		if !checkAdminSetPassword(c, db, req.Password, req.ConfirmPassword) {
			return
		}
		hash, err = auth.NewPasswordHasher().HashPassword(req.Password)
		if err != nil {
			c.JSON(passwordHashErrorStatus(err), gin.H{
				"success": false,
				"error":   "Failed to hash password: " + err.Error(),
			})
			return
		}
	}

	// When the form explicitly submitted the groups field the selection is
	// authoritative (the empty set clears memberships). Otherwise memberships
	// change only when non-empty groups are provided, so serializers that omit
	// multi-selects don't wipe them.
	cleaned := make([]string, 0, len(req.Groups))
	for _, g := range req.Groups {
		if g = strings.TrimSpace(g); g != "" {
			cleaned = append(cleaned, g)
		}
	}
	updateGroups := groupsSubmitted || len(cleaned) > 0
	var groupIDs []int
	if updateGroups {
		if groupIDs, ok = resolveAgentGroupsOrFail(c, db, cleaned); !ok {
			return
		}
	}

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		adminUsersFail(c, "Failed to begin user update transaction", err)
		return
	}
	defer func() { _ = tx.Rollback() }()

	if hash != "" {
		_, err = tx.Exec(database.ConvertPlaceholders(`
			UPDATE users
			SET login = ?, pw = ?, title = ?, first_name = ?, last_name = ?,
				valid_id = ?, change_time = NOW(), change_by = ?
			WHERE id = ?`),
			req.Login, hash, req.Title, req.FirstName, req.LastName, req.ValidID, actorID, id)
	} else {
		_, err = tx.Exec(database.ConvertPlaceholders(`
			UPDATE users
			SET login = ?, title = ?, first_name = ?, last_name = ?,
				valid_id = ?, change_time = NOW(), change_by = ?
			WHERE id = ?`),
			req.Login, req.Title, req.FirstName, req.LastName, req.ValidID, actorID, id)
	}
	if err != nil {
		adminUsersFail(c, "Failed to update user", err)
		return
	}

	if updateGroups {
		if _, err := tx.Exec(database.ConvertPlaceholders(
			"DELETE FROM group_user WHERE user_id = ? AND permission_key = 'rw'"), id); err != nil {
			adminUsersFail(c, "Failed to clear existing groups", err)
			return
		}
		for _, groupID := range groupIDs {
			if _, err := tx.Exec(database.ConvertPlaceholders(`
				INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
				VALUES (?, ?, 'rw', NOW(), ?, NOW(), ?)`),
				id, groupID, actorID, actorID); err != nil {
				adminUsersFail(c, "Failed to assign group", err)
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		adminUsersFail(c, "Failed to commit user update", err)
		return
	}

	after := map[string]any{
		"login":      req.Login,
		"title":      req.Title,
		"first_name": req.FirstName,
		"last_name":  req.LastName,
		"valid_id":   req.ValidID,
		"groups":     before["groups"],
	}
	if updateGroups {
		sortedGroups := append([]int(nil), groupIDs...)
		sort.Ints(sortedGroups)
		after["groups"] = sortedGroups
	}
	if hash != "" {
		before["password"] = ""
		after["password"] = "(new)"
	}
	audit.RecordRequest(c, audit.Event{
		Category:    audit.CategoryAdmin,
		Action:      "admin.user.update",
		Outcome:     audit.OutcomeSuccess,
		TargetType:  "user",
		TargetID:    audit.ID(int64(id)),
		TargetLabel: req.Login,
		Details:     map[string]any{"changes": audit.Changes(before, after)},
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User updated successfully",
	})
}

// loadAgentAuditFields returns the agent fields the admin user endpoints
// write, with the sorted ids of the agent's rw groups, for audit changes.
func loadAgentAuditFields(db *sql.DB, id int) (map[string]any, error) {
	var login, firstName, lastName string
	var title sql.NullString
	var validID int
	if err := db.QueryRow(database.ConvertPlaceholders(
		"SELECT login, title, first_name, last_name, valid_id FROM users WHERE id = ?"), id).
		Scan(&login, &title, &firstName, &lastName, &validID); err != nil {
		return nil, err
	}
	fields := map[string]any{
		"login":      login,
		"title":      title.String,
		"first_name": firstName,
		"last_name":  lastName,
		"valid_id":   validID,
	}
	rows, err := db.Query(database.ConvertPlaceholders(
		"SELECT group_id FROM group_user WHERE user_id = ? AND permission_key = 'rw' ORDER BY group_id"), id)
	if err != nil {
		return fields, nil
	}
	defer rows.Close()
	groups := make([]int, 0)
	for rows.Next() {
		var gid int
		if rows.Scan(&gid) == nil {
			groups = append(groups, gid)
		}
	}
	if rows.Err() == nil {
		fields["groups"] = groups
	}
	return fields, nil
}

// agentLoginAndValid returns an agent's login and valid_id.
func agentLoginAndValid(db *sql.DB, id int) (string, int, error) {
	var login string
	var validID int
	err := db.QueryRow(database.ConvertPlaceholders("SELECT login, valid_id FROM users WHERE id = ?"), id).
		Scan(&login, &validID)
	return login, validID, err
}

// recordAgentValidChange records an agent valid_id change made by the admin
// delete and status endpoints; before is 0 when it could not be read.
func recordAgentValidChange(c *gin.Context, action string, id int, login string, before, after int) {
	var from any
	if before != 0 {
		from = before
	}
	recordAdminAccessEvent(c, action, "user", audit.ID(int64(id)), login, map[string]any{"changes": audit.Changes(
		map[string]any{"valid_id": from}, map[string]any{"valid_id": after})})
}

// recordAdminAccessEvent records a successful admin change to agents,
// groups, roles, queues or their permissions.
func recordAdminAccessEvent(c *gin.Context, action, targetType, targetID, label string, details map[string]any) {
	audit.RecordRequest(c, audit.Event{
		Category:    audit.CategoryAdmin,
		Action:      action,
		Outcome:     audit.OutcomeSuccess,
		TargetType:  targetType,
		TargetID:    targetID,
		TargetLabel: label,
		Details:     details,
	})
}

// setAgentValidID sets users.valid_id, answering 404 when the agent does not exist.
func setAgentValidID(c *gin.Context, db *sql.DB, id, validID int) bool {
	actorID, ok := auditUserID(c)
	if !ok {
		return false
	}
	res, err := db.Exec(database.ConvertPlaceholders(
		"UPDATE users SET valid_id = ?, change_time = NOW(), change_by = ? WHERE id = ?"), validID, actorID, id)
	if err != nil {
		adminUsersFail(c, "Failed to update user status", err)
		return false
	}
	n, err := res.RowsAffected()
	if err != nil {
		adminUsersFail(c, "Failed to update user status", err)
		return false
	}
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return false
	}
	return true
}

// HandleAdminUserDelete handles DELETE /admin/users/:id.
func HandleAdminUserDelete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	db, ok := adminUsersDB(c)
	if !ok {
		return
	}

	login, prevValid, _ := agentLoginAndValid(db, id)

	// Soft delete - set valid_id = 2
	if !setAgentValidID(c, db, id, 2) {
		return
	}
	recordAgentValidChange(c, "admin.user.delete", id, login, prevValid, 2)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User deleted successfully",
	})
}

// HandleAdminUserGroups handles GET /admin/users/:id/groups.
func HandleAdminUserGroups(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	db, ok := adminUsersDB(c)
	if !ok {
		return
	}

	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT DISTINCT g.id, g.name
		FROM groups g
		JOIN group_user gu ON g.id = gu.group_id
		WHERE gu.user_id = ? AND g.valid_id = 1
		ORDER BY g.name`), id)
	if err != nil {
		adminUsersFail(c, "Failed to fetch user groups", err)
		return
	}
	defer rows.Close()

	groups := make([]gin.H, 0)
	for rows.Next() {
		var gid int
		var gname string
		if err := rows.Scan(&gid, &gname); err != nil {
			adminUsersFail(c, "Failed to fetch user groups", err)
			return
		}
		groups = append(groups, gin.H{
			"id":   gid,
			"name": gname,
		})
	}
	if err := rows.Err(); err != nil {
		adminUsersFail(c, "Error iterating user groups", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"groups":  groups,
	})
}

// HandleAdminUsersStatus handles PUT /admin/users/:id/status to toggle user valid status.
func HandleAdminUsersStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	var req struct {
		ValidID int `json:"valid_id" form:"valid_id"`
	}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request data",
		})
		return
	}

	db, ok := adminUsersDB(c)
	if !ok {
		return
	}

	login, currentValid, err := agentLoginAndValid(db, id)

	// Toggle between valid (1) and invalid (2)
	if req.ValidID == 0 {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "User not found",
			})
			return
		}
		if err != nil {
			adminUsersFail(c, "Failed to load user", err)
			return
		}
		if currentValid == 1 {
			req.ValidID = 2
		} else {
			req.ValidID = 1
		}
	}

	if !setAgentValidID(c, db, id, req.ValidID) {
		return
	}
	recordAgentValidChange(c, "admin.user.status", id, login, currentValid, req.ValidID)

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "User status updated successfully",
		"valid_id": req.ValidID,
	})
}

// HandlePasswordPolicy returns the agent password policy
// (PreferencesGroups###Password) that admin-set and self-chosen agent
// passwords are validated against.
func HandlePasswordPolicy(c *gin.Context) {
	db, ok := adminUsersDB(c)
	if !ok {
		return
	}
	policy, err := sysconfig.LoadAgentPasswordPolicy(db)
	if err != nil {
		adminUsersFail(c, "Failed to load password policy", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"policy":  policy,
	})
}
