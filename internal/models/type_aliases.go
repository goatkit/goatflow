package models

import platformmodels "github.com/goatkit/goatflow/internal/platform/models"

// ---- Type aliases (preserve identity) ----

type APITokenUserType = platformmodels.APITokenUserType

type APIToken = platformmodels.APIToken
type APITokenCreateRequest = platformmodels.APITokenCreateRequest
type APITokenCreateResponse = platformmodels.APITokenCreateResponse
type APITokenListItem = platformmodels.APITokenListItem

type DBRole = platformmodels.DBRole
type DBRoleUser = platformmodels.DBRoleUser
type DBGroupRole = platformmodels.DBGroupRole

type Group = platformmodels.Group
type GroupMembership = platformmodels.GroupMembership

type LookupItem = platformmodels.LookupItem

type Role = platformmodels.Role
type Permission = platformmodels.Permission

type ScopeDefinition = platformmodels.ScopeDefinition
type ScopeRegistry = platformmodels.ScopeRegistry

type Session = platformmodels.Session
type SessionData = platformmodels.SessionData

type User = platformmodels.User
type UserRole = platformmodels.UserRole
type LoginRequest = platformmodels.LoginRequest
type LoginResponse = platformmodels.LoginResponse
type ChangePasswordRequest = platformmodels.ChangePasswordRequest

// ---- Const aliases ----

// User/UserRole consts (typed UserRole)
const (
	RoleAdmin    = platformmodels.RoleAdmin
	RoleAgent    = platformmodels.RoleAgent
	RoleCustomer = platformmodels.RoleCustomer
)

// APITokenUserType consts (typed APITokenUserType)
const (
	APITokenUserAgent    = platformmodels.APITokenUserAgent
	APITokenUserCustomer = platformmodels.APITokenUserCustomer
)

// APIToken consts
const (
	TokenPrefix       = platformmodels.TokenPrefix
	TokenRandomLength = platformmodels.TokenRandomLength
	TokenPrefixLength = platformmodels.TokenPrefixLength
	DefaultRateLimit  = platformmodels.DefaultRateLimit
)

// Role consts
const (
	RoleUser                  = platformmodels.RoleUser
	RoleGuest                 = platformmodels.RoleGuest
	PermissionViewTickets     = platformmodels.PermissionViewTickets
	PermissionCreateTickets   = platformmodels.PermissionCreateTickets
	PermissionEditTickets     = platformmodels.PermissionEditTickets
	PermissionDeleteTickets   = platformmodels.PermissionDeleteTickets
	PermissionAssignTickets   = platformmodels.PermissionAssignTickets
	PermissionViewAllTickets  = platformmodels.PermissionViewAllTickets
	PermissionManageUsers     = platformmodels.PermissionManageUsers
	PermissionManageQueues    = platformmodels.PermissionManageQueues
	PermissionManageSettings  = platformmodels.PermissionManageSettings
	PermissionViewReports     = platformmodels.PermissionViewReports
	PermissionManageTemplates = platformmodels.PermissionManageTemplates
	PermissionManageWorkflows = platformmodels.PermissionManageWorkflows
)

// Group consts
const (
	GroupTypeLocal    = platformmodels.GroupTypeLocal
	GroupTypeLDAP     = platformmodels.GroupTypeLDAP
	GroupTypeExternal = platformmodels.GroupTypeExternal
	GroupRoleMember   = platformmodels.GroupRoleMember
	GroupRoleAdmin    = platformmodels.GroupRoleAdmin
	GroupRoleOwner    = platformmodels.GroupRoleOwner
)

// Session consts
const (
	SessionKeyUserID          = platformmodels.SessionKeyUserID
	SessionKeyUserLogin       = platformmodels.SessionKeyUserLogin
	SessionKeyUserType        = platformmodels.SessionKeyUserType
	SessionKeyUserTitle       = platformmodels.SessionKeyUserTitle
	SessionKeyUserFullname    = platformmodels.SessionKeyUserFullname
	SessionKeyCreateTime      = platformmodels.SessionKeyCreateTime
	SessionKeyLastRequest     = platformmodels.SessionKeyLastRequest
	SessionKeyUserRemoteAddr  = platformmodels.SessionKeyUserRemoteAddr
	SessionKeyUserRemoteAgent = platformmodels.SessionKeyUserRemoteAgent
	UserTypeAgent             = platformmodels.UserTypeAgent
	UserTypeCustomer          = platformmodels.UserTypeCustomer
)

type SystemMaintenance = platformmodels.SystemMaintenance

// ---- Var aliases ----

var ValidScopes = platformmodels.ValidScopes
var PermissionTypes = platformmodels.PermissionTypes

// ---- Func forwarding ----

func RegisterScope(def *ScopeDefinition) { platformmodels.RegisterScope(def) }
func UnregisterScope(scope string)       { platformmodels.UnregisterScope(scope) }
func GetAvailableScopes(userRole string, isCustomer bool) []*ScopeDefinition {
	return platformmodels.GetAvailableScopes(userRole, isCustomer)
}
func IsValidScope(scope string) bool { return platformmodels.IsValidScope(scope) }
func IsScopeAllowed(scope string, userRole string, isCustomer bool) bool {
	return platformmodels.IsScopeAllowed(scope, userRole, isCustomer)
}
func GetAllScopes() []*ScopeDefinition { return platformmodels.GetAllScopes() }
