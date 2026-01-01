package models

import (
	"time"
)

// CIType represents the type of configuration item.
type CIType string

const (
	CITypeServer         CIType = "server"
	CITypeWorkstation    CIType = "workstation"
	CITypeNetwork        CIType = "network"
	CITypeStorage        CIType = "storage"
	CITypeSoftware       CIType = "software"
	CITypeApplication    CIType = "application"
	CITypeDatabase       CIType = "database"
	CITypeService        CIType = "service"
	CITypeVirtualMachine CIType = "virtual_machine"
	CITypeContainer      CIType = "container"
	CITypePrinter        CIType = "printer"
	CITypeMobile         CIType = "mobile"
	CITypeOther          CIType = "other"
)

// CIStatus represents the status of a configuration item.
type CIStatus string

const (
	CIStatusActive        CIStatus = "active"
	CIStatusInactive      CIStatus = "inactive"
	CIStatusRetired       CIStatus = "retired"
	CIStatusDisposed      CIStatus = "disposed"
	CIStatusInMaintenance CIStatus = "in_maintenance"
	CIStatusInStorage     CIStatus = "in_storage"
	CIStatusOrdered       CIStatus = "ordered"
	CIStatusInTransit     CIStatus = "in_transit"
)

// CIEnvironment represents the environment of a CI.
type CIEnvironment string

const (
	CIEnvironmentProduction  CIEnvironment = "production"
	CIEnvironmentStaging     CIEnvironment = "staging"
	CIEnvironmentDevelopment CIEnvironment = "development"
	CIEnvironmentTesting     CIEnvironment = "testing"
	CIEnvironmentDR          CIEnvironment = "disaster_recovery"
)

// ConfigurationItem represents an item in the CMDB.
type ConfigurationItem struct {
	ID          uint          `json:"id" gorm:"primaryKey"`
	CINumber    string        `json:"ci_number" gorm:"uniqueIndex;not null"`
	Name        string        `json:"name" gorm:"not null;index"`
	DisplayName string        `json:"display_name"`
	Type        CIType        `json:"type" gorm:"not null;index"`
	SubType     string        `json:"sub_type"`
	Status      CIStatus      `json:"status" gorm:"not null;default:'active';index"`
	Environment CIEnvironment `json:"environment" gorm:"index"`
	Description string        `json:"description"`

	// Classification
	Category        string `json:"category"`
	SubCategory     string `json:"sub_category"`
	Criticality     int    `json:"criticality" gorm:"default:3"` // 1-5 scale
	BusinessService string `json:"business_service"`

	// Ownership and management
	OwnerID        *uint  `json:"owner_id"`
	Owner          *User  `json:"owner,omitempty" gorm:"foreignKey:OwnerID"`
	ManagedByID    *uint  `json:"managed_by_id"`
	ManagedBy      *User  `json:"managed_by,omitempty" gorm:"foreignKey:ManagedByID"`
	SupportGroupID *uint  `json:"support_group_id"`
	SupportGroup   *Group `json:"support_group,omitempty" gorm:"foreignKey:SupportGroupID"`
	Department     string `json:"department"`

	// Location information
	Location     string `json:"location"`
	Building     string `json:"building"`
	Floor        string `json:"floor"`
	Room         string `json:"room"`
	Rack         string `json:"rack"`
	RackPosition string `json:"rack_position"`

	// Technical details
	Manufacturer    string `json:"manufacturer"`
	Model           string `json:"model"`
	SerialNumber    string `json:"serial_number" gorm:"index"`
	AssetTag        string `json:"asset_tag" gorm:"index"`
	Version         string `json:"version"`
	OperatingSystem string `json:"operating_system"`
	IPAddress       string `json:"ip_address"`
	MACAddress      string `json:"mac_address"`
	Hostname        string `json:"hostname"`
	Domain          string `json:"domain"`

	// Hardware specifications
	CPU      string `json:"cpu"`
	CPUCores int    `json:"cpu_cores"`
	RAM      int    `json:"ram"`     // GB
	Storage  int    `json:"storage"` // GB

	// Financial information
	PurchaseDate       *time.Time `json:"purchase_date"`
	PurchasePrice      float64    `json:"purchase_price"`
	Currency           string     `json:"currency"`
	PONumber           string     `json:"po_number"`
	InvoiceNumber      string     `json:"invoice_number"`
	VendorID           *uint      `json:"vendor_id"`
	Vendor             *Vendor    `json:"vendor,omitempty" gorm:"foreignKey:VendorID"`
	WarrantyExpiry     *time.Time `json:"warranty_expiry"`
	LeaseExpiry        *time.Time `json:"lease_expiry"`
	DepreciationMethod string     `json:"depreciation_method"`
	CurrentValue       float64    `json:"current_value"`

	// Lifecycle management
	InstallDate         *time.Time `json:"install_date"`
	CommissionDate      *time.Time `json:"commission_date"`
	DecommissionDate    *time.Time `json:"decommission_date"`
	LastAuditDate       *time.Time `json:"last_audit_date"`
	NextAuditDate       *time.Time `json:"next_audit_date"`
	LastMaintenanceDate *time.Time `json:"last_maintenance_date"`
	NextMaintenanceDate *time.Time `json:"next_maintenance_date"`
	EndOfLife           *time.Time `json:"end_of_life"`

	// Relationships
	ParentCIID   *uint               `json:"parent_ci_id"`
	ParentCI     *ConfigurationItem  `json:"parent_ci,omitempty" gorm:"foreignKey:ParentCIID"`
	ChildCIs     []ConfigurationItem `json:"child_cis,omitempty" gorm:"foreignKey:ParentCIID"`
	RelatedCIs   []ConfigurationItem `json:"related_cis,omitempty" gorm:"many2many:ci_relationships;"`
	Dependencies []ConfigurationItem `json:"dependencies,omitempty" gorm:"many2many:ci_dependencies;"`

	// Compliance and security
	ComplianceStatus   string     `json:"compliance_status"`
	SecurityLevel      string     `json:"security_level"`
	DataClassification string     `json:"data_classification"`
	LastSecurityScan   *time.Time `json:"last_security_scan"`
	ComplianceTags     []string   `json:"compliance_tags" gorm:"type:text[]"`

	// Additional attributes (flexible schema)
	Attributes map[string]interface{} `json:"attributes" gorm:"type:jsonb"`

	// Documentation
	DocumentationURL   string   `json:"documentation_url"`
	ConfigurationNotes string   `json:"configuration_notes"`
	Tags               []string `json:"tags" gorm:"type:text[]"`

	// Audit fields
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	CreatedByID      uint       `json:"created_by_id"`
	CreatedBy        *User      `json:"created_by,omitempty" gorm:"foreignKey:CreatedByID"`
	LastModifiedByID *uint      `json:"last_modified_by_id"`
	LastModifiedBy   *User      `json:"last_modified_by,omitempty" gorm:"foreignKey:LastModifiedByID"`
	LastVerifiedAt   *time.Time `json:"last_verified_at"`
	LastVerifiedByID *uint      `json:"last_verified_by_id"`
	LastVerifiedBy   *User      `json:"last_verified_by,omitempty" gorm:"foreignKey:LastVerifiedByID"`
}

// CIRelationship represents relationships between configuration items.
type CIRelationship struct {
	ID           uint               `json:"id" gorm:"primaryKey"`
	SourceCIID   uint               `json:"source_ci_id" gorm:"not null"`
	SourceCI     *ConfigurationItem `json:"source_ci,omitempty" gorm:"foreignKey:SourceCIID"`
	TargetCIID   uint               `json:"target_ci_id" gorm:"not null"`
	TargetCI     *ConfigurationItem `json:"target_ci,omitempty" gorm:"foreignKey:TargetCIID"`
	RelationType string             `json:"relation_type"` // depends_on, connects_to, runs_on, uses, etc.
	Description  string             `json:"description"`
	IsActive     bool               `json:"is_active" gorm:"default:true"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

// CIHistory tracks changes to configuration items.
type CIHistory struct {
	ID              uint               `json:"id" gorm:"primaryKey"`
	CIID            uint               `json:"ci_id" gorm:"not null;index"`
	CI              *ConfigurationItem `json:"ci,omitempty" gorm:"foreignKey:CIID"`
	FieldName       string             `json:"field_name" gorm:"not null"`
	OldValue        string             `json:"old_value"`
	NewValue        string             `json:"new_value"`
	ChangeType      string             `json:"change_type"` // create, update, delete
	ChangedByID     uint               `json:"changed_by_id" gorm:"not null"`
	ChangedBy       *User              `json:"changed_by,omitempty" gorm:"foreignKey:ChangedByID"`
	ChangeReason    string             `json:"change_reason"`
	ChangeTicketRef string             `json:"change_ticket_ref"`
	CreatedAt       time.Time          `json:"created_at"`
}

// Service represents a business or IT service.
type Service struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	ServiceCode string `json:"service_code" gorm:"uniqueIndex;not null"`
	Name        string `json:"name" gorm:"not null"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Type        string `json:"type"`                         // business, technical, infrastructure
	Status      string `json:"status"`                       // active, inactive, deprecated
	Criticality int    `json:"criticality" gorm:"default:3"` // 1-5 scale

	// Ownership
	OwnerID          *uint  `json:"owner_id"`
	Owner            *User  `json:"owner,omitempty" gorm:"foreignKey:OwnerID"`
	ServiceManagerID *uint  `json:"service_manager_id"`
	ServiceManager   *User  `json:"service_manager,omitempty" gorm:"foreignKey:ServiceManagerID"`
	SupportGroupID   *uint  `json:"support_group_id"`
	SupportGroup     *Group `json:"support_group,omitempty" gorm:"foreignKey:SupportGroupID"`

	// Service level
	ServiceLevel       string  `json:"service_level"` // gold, silver, bronze
	SLADocument        string  `json:"sla_document"`
	AvailabilityTarget float64 `json:"availability_target"` // Percentage
	RPO                int     `json:"rpo"`                 // Recovery Point Objective in minutes
	RTO                int     `json:"rto"`                 // Recovery Time Objective in minutes

	// Dependencies
	DependentCIs      []ConfigurationItem `json:"dependent_cis,omitempty" gorm:"many2many:service_ci_dependencies;"`
	DependentServices []Service           `json:"dependent_services,omitempty" gorm:"many2many:service_dependencies;"`

	// Business information
	BusinessUnit  string  `json:"business_unit"`
	CostCenter    string  `json:"cost_center"`
	MonthlyCost   float64 `json:"monthly_cost"`
	AnnualRevenue float64 `json:"annual_revenue"`
	UserCount     int     `json:"user_count"`

	// Documentation
	DocumentationURL     string `json:"documentation_url"`
	RunbookURL           string `json:"runbook_url"`
	DisasterRecoveryPlan string `json:"disaster_recovery_plan"`

	// Audit fields
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Vendor represents a vendor/supplier.
type Vendor struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	VendorCode  string `json:"vendor_code" gorm:"uniqueIndex;not null"`
	Name        string `json:"name" gorm:"not null"`
	Description string `json:"description"`
	Type        string `json:"type"`   // hardware, software, service
	Status      string `json:"status"` // active, inactive, blacklisted

	// Contact information
	ContactName  string `json:"contact_name"`
	ContactEmail string `json:"contact_email"`
	ContactPhone string `json:"contact_phone"`
	Address      string `json:"address"`
	City         string `json:"city"`
	State        string `json:"state"`
	Country      string `json:"country"`
	PostalCode   string `json:"postal_code"`
	Website      string `json:"website"`

	// Contract information
	ContractNumber string     `json:"contract_number"`
	ContractStart  *time.Time `json:"contract_start"`
	ContractEnd    *time.Time `json:"contract_end"`
	ContractValue  float64    `json:"contract_value"`
	PaymentTerms   string     `json:"payment_terms"`
	SupportLevel   string     `json:"support_level"`
	SupportPhone   string     `json:"support_phone"`
	SupportEmail   string     `json:"support_email"`

	// Performance metrics
	Rating        float64 `json:"rating"` // 1-5 scale
	DeliveryScore float64 `json:"delivery_score"`
	QualityScore  float64 `json:"quality_score"`
	ResponseTime  int     `json:"response_time"` // Hours

	// Audit fields
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CIListRequest represents a request to list configuration items.
type CIListRequest struct {
	Page        int           `json:"page" form:"page"`
	PerPage     int           `json:"per_page" form:"per_page"`
	Type        CIType        `json:"type" form:"type"`
	Status      CIStatus      `json:"status" form:"status"`
	Environment CIEnvironment `json:"environment" form:"environment"`
	OwnerID     uint          `json:"owner_id" form:"owner_id"`
	Location    string        `json:"location" form:"location"`
	Search      string        `json:"search" form:"search"`
	SortBy      string        `json:"sort_by" form:"sort_by"`
	SortOrder   string        `json:"sort_order" form:"sort_order"`
}

// CIListResponse represents a response containing a list of configuration items.
type CIListResponse struct {
	ConfigurationItems []*ConfigurationItem `json:"configuration_items"`
	Total              int64                `json:"total"`
	Page               int                  `json:"page"`
	PerPage            int                  `json:"per_page"`
	TotalPages         int                  `json:"total_pages"`
}
