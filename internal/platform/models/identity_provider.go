package models

import "time"

type IdentityProvider struct {
	ID                uint      `json:"id"`
	OrgID             *uint     `json:"org_id,omitempty"`
	Name              string    `json:"name"`
	ProviderType      string    `json:"provider_type"`
	ClientID          string    `json:"client_id"`
	ClientSecret      string    `json:"-"`
	DiscoveryURL      string    `json:"discovery_url"`
	SigningCert       string    `json:"signing_cert"`
	PrivateKey        string    `json:"-"`
	EntityID          string    `json:"entity_id"`
	ACSURL            string    `json:"acs_url"`
	Scopes            string    `json:"scopes"`
	UserClaimEmail    string    `json:"user_claim_email"`
	UserClaimName     string    `json:"user_claim_name"`
	UserClaimGroups   string    `json:"user_claim_groups"`
	Enabled           bool      `json:"enabled"`
	AutoProvision     bool      `json:"auto_provision"`
	UserTable         string    `json:"user_table"`
	AutoAddToGroup    string    `json:"auto_add_to_group"`
	CreateTime        time.Time `json:"create_time"`
	CreateBy          uint      `json:"create_by"`
	ChangeTime        time.Time `json:"change_time"`
	ChangeBy          uint      `json:"change_by"`
}

func (m *IdentityProvider) IsActive() bool {
	return m.Enabled
}

func (m *IdentityProvider) GetProviderType() string {
	return m.ProviderType
}