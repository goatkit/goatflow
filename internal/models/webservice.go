package models

import (
	"time"
)

// WebserviceConfig represents a GenericInterface webservice configuration.
// Stored in gi_webservice_config table.
type WebserviceConfig struct {
	ID         int                   `json:"id"`
	Name       string                `json:"name"`
	Config     *WebserviceConfigData `json:"config,omitempty"`
	ConfigRaw  []byte                `json:"-"` // Raw YAML from database
	ValidID    int                   `json:"valid_id"`
	CreateTime time.Time             `json:"create_time"`
	CreateBy   int                   `json:"create_by"`
	ChangeTime time.Time             `json:"change_time"`
	ChangeBy   int                   `json:"change_by"`
}

// WebserviceConfigData represents the parsed YAML configuration.
// Structure matches OTRS's GenericInterface config format.
type WebserviceConfigData struct {
	Name             string           `yaml:"Name,omitempty" json:"name,omitempty"`
	Description      string           `yaml:"Description,omitempty" json:"description,omitempty"`
	RemoteSystem     string           `yaml:"RemoteSystem,omitempty" json:"remote_system,omitempty"`
	FrameworkVersion string           `yaml:"FrameworkVersion,omitempty" json:"framework_version,omitempty"`
	Debugger         DebuggerConfig   `yaml:"Debugger,omitempty" json:"debugger,omitempty"`
	Provider         ProviderConfig   `yaml:"Provider,omitempty" json:"provider,omitempty"`
	Requester        RequesterConfig  `yaml:"Requester,omitempty" json:"requester,omitempty"`
}

// DebuggerConfig controls debug logging for the webservice.
type DebuggerConfig struct {
	DebugThreshold string `yaml:"DebugThreshold,omitempty" json:"debug_threshold,omitempty"` // debug, info, notice, error
	TestMode       string `yaml:"TestMode,omitempty" json:"test_mode,omitempty"`             // 0 or 1
}

// ProviderConfig defines inbound operations (when this system receives requests).
type ProviderConfig struct {
	Operation map[string]OperationConfig `yaml:"Operation,omitempty" json:"operation,omitempty"`
	Transport TransportConfig            `yaml:"Transport,omitempty" json:"transport,omitempty"`
}

// RequesterConfig defines outbound invokers (when this system makes requests).
type RequesterConfig struct {
	Invoker   map[string]InvokerConfig `yaml:"Invoker,omitempty" json:"invoker,omitempty"`
	Transport TransportConfig          `yaml:"Transport,omitempty" json:"transport,omitempty"`
}

// OperationConfig defines an inbound operation.
type OperationConfig struct {
	Type            string        `yaml:"Type,omitempty" json:"type,omitempty"`
	Description     string        `yaml:"Description,omitempty" json:"description,omitempty"`
	MappingInbound  MappingConfig `yaml:"MappingInbound,omitempty" json:"mapping_inbound,omitempty"`
	MappingOutbound MappingConfig `yaml:"MappingOutbound,omitempty" json:"mapping_outbound,omitempty"`
}

// InvokerConfig defines an outbound invoker.
type InvokerConfig struct {
	Type            string        `yaml:"Type,omitempty" json:"type,omitempty"`
	Description     string        `yaml:"Description,omitempty" json:"description,omitempty"`
	Events          []EventConfig `yaml:"Events,omitempty" json:"events,omitempty"`
	MappingInbound  MappingConfig `yaml:"MappingInbound,omitempty" json:"mapping_inbound,omitempty"`
	MappingOutbound MappingConfig `yaml:"MappingOutbound,omitempty" json:"mapping_outbound,omitempty"`
}

// EventConfig defines event triggers for invokers.
type EventConfig struct {
	Event       string `yaml:"Event,omitempty" json:"event,omitempty"`
	Asynchronous string `yaml:"Asynchronous,omitempty" json:"asynchronous,omitempty"` // 0 or 1
}

// MappingConfig defines data transformation rules.
type MappingConfig struct {
	Type   string                 `yaml:"Type,omitempty" json:"type,omitempty"` // Simple, XSLT, etc.
	Config map[string]interface{} `yaml:"Config,omitempty" json:"config,omitempty"`
}

// TransportConfig defines the transport layer configuration.
type TransportConfig struct {
	Type   string                `yaml:"Type,omitempty" json:"type,omitempty"` // HTTP::REST, HTTP::SOAP
	Config TransportHTTPConfig   `yaml:"Config,omitempty" json:"config,omitempty"`
}

// TransportHTTPConfig defines HTTP transport settings.
type TransportHTTPConfig struct {
	// Common settings
	Host           string `yaml:"Host,omitempty" json:"host,omitempty"`
	DefaultCommand string `yaml:"DefaultCommand,omitempty" json:"default_command,omitempty"` // GET, POST, PUT, DELETE
	Timeout        string `yaml:"Timeout,omitempty" json:"timeout,omitempty"`                // seconds

	// REST-specific settings
	InvokerControllerMapping map[string]ControllerMapping `yaml:"InvokerControllerMapping,omitempty" json:"invoker_controller_mapping,omitempty"`

	// Provider REST settings
	RouteOperationMapping map[string]RouteMapping `yaml:"RouteOperationMapping,omitempty" json:"route_operation_mapping,omitempty"`
	MaxLength             string                  `yaml:"MaxLength,omitempty" json:"max_length,omitempty"`
	KeepAlive             string                  `yaml:"KeepAlive,omitempty" json:"keep_alive,omitempty"`
	AdditionalHeaders     map[string]string       `yaml:"AdditionalHeaders,omitempty" json:"additional_headers,omitempty"`

	// SOAP-specific settings
	Encoding  string `yaml:"Encoding,omitempty" json:"encoding,omitempty"`
	Endpoint  string `yaml:"Endpoint,omitempty" json:"endpoint,omitempty"`
	NameSpace string `yaml:"NameSpace,omitempty" json:"namespace,omitempty"`
	SOAPAction string `yaml:"SOAPAction,omitempty" json:"soap_action,omitempty"`

	// Authentication
	Authentication AuthConfig `yaml:"Authentication,omitempty" json:"authentication,omitempty"`

	// SSL settings
	SSL SSLConfig `yaml:"SSL,omitempty" json:"ssl,omitempty"`

	// Proxy settings
	Proxy ProxyConfig `yaml:"Proxy,omitempty" json:"proxy,omitempty"`
}

// ControllerMapping maps invokers to REST endpoints.
type ControllerMapping struct {
	Controller string `yaml:"Controller,omitempty" json:"controller,omitempty"`
	Command    string `yaml:"Command,omitempty" json:"command,omitempty"` // GET, POST, etc.
}

// RouteMapping maps operations to REST routes (for providers).
type RouteMapping struct {
	Route         string   `yaml:"Route,omitempty" json:"route,omitempty"`
	RequestMethod []string `yaml:"RequestMethod,omitempty" json:"request_method,omitempty"`
}

// AuthConfig defines authentication settings.
type AuthConfig struct {
	AuthType string `yaml:"AuthType,omitempty" json:"auth_type,omitempty"` // BasicAuth, JWT, OAuth2, APIKey

	// Basic Auth
	BasicAuthUser     string `yaml:"BasicAuthUser,omitempty" json:"basic_auth_user,omitempty"`
	BasicAuthPassword string `yaml:"BasicAuthPassword,omitempty" json:"basic_auth_password,omitempty"`

	// API Key
	APIKey       string `yaml:"APIKey,omitempty" json:"api_key,omitempty"`
	APIKeyHeader string `yaml:"APIKeyHeader,omitempty" json:"api_key_header,omitempty"` // Header name, defaults to X-API-Key

	// OAuth2
	OAuth2TokenURL     string `yaml:"OAuth2TokenURL,omitempty" json:"oauth2_token_url,omitempty"`
	OAuth2ClientID     string `yaml:"OAuth2ClientID,omitempty" json:"oauth2_client_id,omitempty"`
	OAuth2ClientSecret string `yaml:"OAuth2ClientSecret,omitempty" json:"oauth2_client_secret,omitempty"`
	OAuth2Scope        string `yaml:"OAuth2Scope,omitempty" json:"oauth2_scope,omitempty"`

	// JWT
	JWTAuthKeyFilePath         string `yaml:"JWTAuthKeyFilePath,omitempty" json:"jwt_auth_key_file_path,omitempty"`
	JWTAuthKeyFilePassword     string `yaml:"JWTAuthKeyFilePassword,omitempty" json:"jwt_auth_key_file_password,omitempty"`
	JWTAuthAlgorithm           string `yaml:"JWTAuthAlgorithm,omitempty" json:"jwt_auth_algorithm,omitempty"`
	JWTAuthCertificateFilePath string `yaml:"JWTAuthCertificateFilePath,omitempty" json:"jwt_auth_certificate_file_path,omitempty"`
	JWTAuthTTL                 string `yaml:"JWTAuthTTL,omitempty" json:"jwt_auth_ttl,omitempty"`
	JWTAuthPayload             string `yaml:"JWTAuthPayload,omitempty" json:"jwt_auth_payload,omitempty"`
	JWTAuthAdditionalHeaderData string `yaml:"JWTAuthAdditionalHeaderData,omitempty" json:"jwt_auth_additional_header_data,omitempty"`
}

// SSLConfig defines SSL/TLS settings.
type SSLConfig struct {
	SSLVerifyHostname string `yaml:"SSLVerifyHostname,omitempty" json:"ssl_verify_hostname,omitempty"` // 0 or 1
	SSLVerifyCert     string `yaml:"SSLVerifyCert,omitempty" json:"ssl_verify_cert,omitempty"`         // 0 or 1
	SSLCAFile         string `yaml:"SSLCAFile,omitempty" json:"ssl_ca_file,omitempty"`
	SSLCADir          string `yaml:"SSLCADir,omitempty" json:"ssl_ca_dir,omitempty"`
	SSLCertFile       string `yaml:"SSLCertFile,omitempty" json:"ssl_cert_file,omitempty"`
	SSLKeyFile        string `yaml:"SSLKeyFile,omitempty" json:"ssl_key_file,omitempty"`
}

// ProxyConfig defines HTTP proxy settings.
type ProxyConfig struct {
	UseProxy     string `yaml:"UseProxy,omitempty" json:"use_proxy,omitempty"` // 0 or 1
	ProxyHost    string `yaml:"ProxyHost,omitempty" json:"proxy_host,omitempty"`
	ProxyPort    string `yaml:"ProxyPort,omitempty" json:"proxy_port,omitempty"`
	ProxyUser    string `yaml:"ProxyUser,omitempty" json:"proxy_user,omitempty"`
	ProxyPassword string `yaml:"ProxyPassword,omitempty" json:"proxy_password,omitempty"`
}

// WebserviceConfigHistory represents a historical version of a webservice config.
type WebserviceConfigHistory struct {
	ID         int64     `json:"id"`
	ConfigID   int       `json:"config_id"`
	Config     []byte    `json:"config"`
	ConfigMD5  string    `json:"config_md5"`
	CreateTime time.Time `json:"create_time"`
	CreateBy   int       `json:"create_by"`
	ChangeTime time.Time `json:"change_time"`
	ChangeBy   int       `json:"change_by"`
}

// IsValid returns true if the webservice is active.
func (w *WebserviceConfig) IsValid() bool {
	return w.ValidID == 1
}

// GetInvoker returns an invoker by name from the Requester config.
func (w *WebserviceConfig) GetInvoker(name string) *InvokerConfig {
	if w.Config == nil || w.Config.Requester.Invoker == nil {
		return nil
	}
	if inv, ok := w.Config.Requester.Invoker[name]; ok {
		return &inv
	}
	return nil
}

// GetOperation returns an operation by name from the Provider config.
func (w *WebserviceConfig) GetOperation(name string) *OperationConfig {
	if w.Config == nil || w.Config.Provider.Operation == nil {
		return nil
	}
	if op, ok := w.Config.Provider.Operation[name]; ok {
		return &op
	}
	return nil
}

// InvokerNames returns a list of all invoker names.
func (w *WebserviceConfig) InvokerNames() []string {
	if w.Config == nil || w.Config.Requester.Invoker == nil {
		return nil
	}
	names := make([]string, 0, len(w.Config.Requester.Invoker))
	for name := range w.Config.Requester.Invoker {
		names = append(names, name)
	}
	return names
}

// OperationNames returns a list of all operation names.
func (w *WebserviceConfig) OperationNames() []string {
	if w.Config == nil || w.Config.Provider.Operation == nil {
		return nil
	}
	names := make([]string, 0, len(w.Config.Provider.Operation))
	for name := range w.Config.Provider.Operation {
		names = append(names, name)
	}
	return names
}

// TransportType returns the requester transport type (HTTP::REST, HTTP::SOAP).
func (w *WebserviceConfig) TransportType() string {
	if w.Config == nil {
		return ""
	}
	return w.Config.Requester.Transport.Type
}

// RequesterHost returns the requester transport host URL.
func (w *WebserviceConfig) RequesterHost() string {
	if w.Config == nil {
		return ""
	}
	return w.Config.Requester.Transport.Config.Host
}
