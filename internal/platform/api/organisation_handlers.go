package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/organisation"
	"github.com/goatkit/goatflow/internal/platform/routing"
)

func init() {
	routing.RegisterHandler("handleSwitchOrg", handleSwitchOrg)
	routing.RegisterHandler("handleListUserOrgs", handleListUserOrgs)
	routing.RegisterHandler("handleAPIListOrgs", handleAPIListOrgs)
	routing.RegisterHandler("handleAPICreateOrg", handleAPICreateOrg)
	routing.RegisterHandler("handleAPIUpdateOrg", handleAPIUpdateOrg)
	routing.RegisterHandler("handleAPIDeleteOrg", handleAPIDeleteOrg)
	routing.RegisterHandler("handleAPIListMembers", handleAPIListMembers)
	routing.RegisterHandler("handleAPIAddMember", handleAPIAddMember)
	routing.RegisterHandler("handleAPIRemoveMember", handleAPIRemoveMember)
	routing.RegisterHandler("handleAPIListOrgConfigs", handleAPIListOrgConfigs)
	routing.RegisterHandler("handleAPISetOrgConfig", handleAPISetOrgConfig)
	routing.RegisterHandler("handleAPIDeleteOrgConfig", handleAPIDeleteOrgConfig)
}

// orgRepo returns a lazily-initialised organisation repository.
// Returns nil + 503 response if DB is unavailable.
func orgRepo(c *gin.Context) *organisation.Repository {
	repo, err := organisation.NewRepository()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return nil
	}
	return repo
}

func handleSwitchOrg(c *gin.Context) {
	repo := orgRepo(c)
	if repo == nil {
		return
	}
	organisation.HandleSwitchOrg(repo)(c)
}

func handleListUserOrgs(c *gin.Context) {
	repo := orgRepo(c)
	if repo == nil {
		return
	}
	organisation.HandleListUserOrgs(repo)(c)
}

func handleAPIListOrgs(c *gin.Context) {
	repo := orgRepo(c)
	if repo == nil {
		return
	}
	organisation.HandleAdminListOrgs(repo)(c)
}

func handleAPICreateOrg(c *gin.Context) {
	repo := orgRepo(c)
	if repo == nil {
		return
	}
	organisation.HandleAdminCreateOrg(repo)(c)
}

func handleAPIUpdateOrg(c *gin.Context) {
	repo := orgRepo(c)
	if repo == nil {
		return
	}
	organisation.HandleAdminUpdateOrg(repo)(c)
}

func handleAPIDeleteOrg(c *gin.Context) {
	repo := orgRepo(c)
	if repo == nil {
		return
	}
	organisation.HandleAdminDeleteOrg(repo)(c)
}

func handleAPIListMembers(c *gin.Context) {
	repo := orgRepo(c)
	if repo == nil {
		return
	}
	organisation.HandleAdminListMembers(repo)(c)
}

func handleAPIAddMember(c *gin.Context) {
	repo := orgRepo(c)
	if repo == nil {
		return
	}
	organisation.HandleAdminAddMember(repo)(c)
}

func handleAPIRemoveMember(c *gin.Context) {
	repo := orgRepo(c)
	if repo == nil {
		return
	}
	organisation.HandleAdminRemoveMember(repo)(c)
}

func handleAPIListOrgConfigs(c *gin.Context) {
	repo := orgRepo(c)
	if repo == nil {
		return
	}
	organisation.HandleAdminListOrgConfigs(repo)(c)
}

func handleAPISetOrgConfig(c *gin.Context) {
	repo := orgRepo(c)
	if repo == nil {
		return
	}
	organisation.HandleAdminSetOrgConfig(repo)(c)
}

func handleAPIDeleteOrgConfig(c *gin.Context) {
	repo := orgRepo(c)
	if repo == nil {
		return
	}
	organisation.HandleAdminDeleteOrgConfig(repo)(c)
}
