package organisation

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// HandleSwitchOrg handles POST /api/v1/session/org to switch the active organisation.
func HandleSwitchOrg(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			OrgID int64 `json:"org_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.OrgID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "org_id is required"})
			return
		}

		// Verify org exists and is active.
		org, err := repo.GetOrg(req.OrgID)
		if err != nil || org == nil || !org.IsActive() {
			c.JSON(http.StatusNotFound, gin.H{"error": "organisation not found"})
			return
		}

		// Verify user is a member of this org.
		userID, _ := c.Get("user_id")
		uid, ok := toInt(userID)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		userOrgs, err := repo.GetUserOrgs(uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
			return
		}

		isMember := false
		for _, o := range userOrgs {
			if o.ID == req.OrgID {
				isMember = true
				break
			}
		}
		if !isMember {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this organisation"})
			return
		}

		// Set the active org cookie (30 day expiry).
		c.SetCookie("active_org_id", strconv.FormatInt(req.OrgID, 10), 86400*30, "/", "", false, true)

		// Update context for current request.
		setOrgContext(c, req.OrgID)

		c.JSON(http.StatusOK, gin.H{
			"status": "switched",
			"org_id": req.OrgID,
			"org":    org.Name,
		})
	}
}

// HandleListUserOrgs handles GET /api/v1/session/orgs to list the current user's organisations.
func HandleListUserOrgs(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		uid, ok := toInt(userID)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		orgs, err := repo.GetUserOrgs(uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Prefer the gin context (set by organisation.Middleware when
		// installed), but fall back to the cookie directly. The middleware
		// isn't wired into the plugin route chain, so without the cookie
		// fallback the picker can't highlight the user's active org.
		activeOrgID := ActiveOrgFromGin(c)
		if activeOrgID == 0 {
			if cookie, err := c.Cookie("active_org_id"); err == nil && cookie != "" {
				var n int64
				if _, scanErr := fmt.Sscanf(cookie, "%d", &n); scanErr == nil {
					activeOrgID = n
				}
			}
		}

		type orgItem struct {
			ID     int64  `json:"id"`
			Name   string `json:"name"`
			Slug   string `json:"slug"`
			Active bool   `json:"active"`
		}

		items := make([]orgItem, len(orgs))
		for i, o := range orgs {
			items[i] = orgItem{
				ID:     o.ID,
				Name:   o.Name,
				Slug:   o.Slug,
				Active: o.ID == activeOrgID,
			}
		}

		c.JSON(http.StatusOK, gin.H{"organisations": items, "active_org_id": activeOrgID})
	}
}

// --- Admin Handlers ---

// HandleAdminListOrgs handles GET /admin/organisations.
func HandleAdminListOrgs(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		statusFilter := c.Query("status")
		orgs, err := repo.ListOrgs(statusFilter, false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"organisations": orgs})
	}
}

// HandleAdminCreateOrg handles POST /admin/api/organisations.
func HandleAdminCreateOrg(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name              string  `json:"name"`
			Slug              string  `json:"slug"`
			CustomerCompanyID *string `json:"customer_company_id,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Name == "" || req.Slug == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name and slug are required"})
			return
		}

		if !isValidSlug(req.Slug) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "slug must be lowercase alphanumeric with hyphens"})
			return
		}

		// Check slug uniqueness.
		existing, _ := repo.GetOrgBySlug(req.Slug)
		if existing != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "slug already in use"})
			return
		}

		org := &Organisation{
			Name:              req.Name,
			Slug:              req.Slug,
			Status:            StatusActive,
			CustomerCompanyID: req.CustomerCompanyID,
			ValidID:           1,
		}

		userID := getAdminUserID(c)
		id, err := repo.CreateOrg(org, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"id": id, "slug": req.Slug})
	}
}

// HandleAdminUpdateOrg handles PUT /admin/api/organisations/:id.
func HandleAdminUpdateOrg(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org ID"})
			return
		}

		org, err := repo.GetOrg(id)
		if err != nil || org == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "organisation not found"})
			return
		}

		var req struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Name != "" {
			org.Name = req.Name
		}
		if req.Status != "" {
			if !IsValidStatus(req.Status) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
				return
			}
			org.Status = req.Status
		}

		userID := getAdminUserID(c)
		if err := repo.UpdateOrg(org, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "updated"})
	}
}

// HandleAdminDeleteOrg handles DELETE /admin/api/organisations/:id.
func HandleAdminDeleteOrg(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org ID"})
			return
		}

		org, _ := repo.GetOrg(id)
		if org == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "organisation not found"})
			return
		}

		if err := repo.DeleteOrg(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	}
}

// HandleAdminListMembers handles GET /admin/api/organisations/:id/members.
func HandleAdminListMembers(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org ID"})
			return
		}

		members, err := repo.ListMembers(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"members": members})
	}
}

// HandleAdminAddMember handles POST /admin/api/organisations/:id/members.
func HandleAdminAddMember(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org ID"})
			return
		}

		var req struct {
			UserID        *int    `json:"user_id,omitempty"`
			CustomerLogin *string `json:"customer_login,omitempty"`
			Role          string  `json:"role"`
			IsDefault     bool    `json:"is_default"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.UserID == nil && req.CustomerLogin == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id or customer_login is required"})
			return
		}

		role := req.Role
		if role == "" {
			role = RoleMember
		}
		if !IsValidRole(role) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
			return
		}

		member := &UserOrganisation{
			OrgID:         orgID,
			UserID:        req.UserID,
			CustomerLogin: req.CustomerLogin,
			Role:          role,
			IsDefault:     req.IsDefault,
		}

		userID := getAdminUserID(c)
		id, err := repo.AddMember(member, userID)
		if err != nil {
			if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "duplicate") {
				c.JSON(http.StatusConflict, gin.H{"error": "user is already a member"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"id": id})
	}
}

// HandleAdminRemoveMember handles DELETE /admin/api/organisations/:id/members/:member_id.
func HandleAdminRemoveMember(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, err := strconv.ParseInt(c.Param("member_id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid member ID"})
			return
		}

		if err := repo.RemoveMember(memberID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "removed"})
	}
}

// --- Per-Org Config Handlers ---

// HandleAdminListOrgConfigs handles GET /admin/api/organisations/:id/config.
func HandleAdminListOrgConfigs(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org ID"})
			return
		}

		configs, err := repo.ListOrgConfigs(orgID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Convert to name → value map for easier consumption.
		result := make(map[string]string, len(configs))
		for _, cfg := range configs {
			result[cfg.Name] = string(cfg.EffectiveValue)
		}

		c.JSON(http.StatusOK, gin.H{"org_id": orgID, "config": result})
	}
}

// HandleAdminSetOrgConfig handles PUT /admin/api/organisations/:id/config.
func HandleAdminSetOrgConfig(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org ID"})
			return
		}

		var req struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name and value are required"})
			return
		}

		userID := getAdminUserID(c)
		if err := repo.SetOrgConfig(orgID, req.Name, []byte(req.Value), userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "saved"})
	}
}

// HandleAdminDeleteOrgConfig handles DELETE /admin/api/organisations/:id/config/:name.
func HandleAdminDeleteOrgConfig(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org ID"})
			return
		}

		name := c.Param("name")
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "config name is required"})
			return
		}

		if err := repo.DeleteOrgConfig(orgID, name); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	}
}

// --- Helpers ---

var slugRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func isValidSlug(s string) bool {
	return slugRegex.MatchString(s) && !strings.HasPrefix(s, "-") && !strings.HasSuffix(s, "-")
}

func getAdminUserID(c *gin.Context) int {
	if id, exists := c.Get("user_id"); exists {
		if uid, ok := toInt(id); ok {
			return uid
		}
	}
	return 1
}
