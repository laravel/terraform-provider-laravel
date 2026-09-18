package client

import "encoding/json"

// ---------------------------------------------------------------------
// JSON:API envelope types
// ---------------------------------------------------------------------

// Document is a generic JSON:API top-level document.
type Document[T any] struct {
	Data     T                `json:"data"`
	Included []ResourceObject `json:"included,omitempty"`
}

// ListDocument wraps a JSON:API list response.
type ListDocument[T any] struct {
	Data     []T              `json:"data"`
	Included []ResourceObject `json:"included,omitempty"`
	Meta     ListMeta         `json:"meta"`
}

// ListMeta is the paginator metadata every Laravel Cloud collection response
// carries. Every list route in the public API is paginated, so a single request
// only ever returns the first page; LastPage is how the rest are discovered.
type ListMeta struct {
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
	PerPage     int `json:"per_page"`
	Total       int `json:"total"`
}

// ResourceObject is a minimal JSON:API resource (used for included).
type ResourceObject struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
}

// ---------------------------------------------------------------------
// Application
// ---------------------------------------------------------------------

type ApplicationData struct {
	ID         string                `json:"id"`
	Type       string                `json:"type"`
	Attributes ApplicationAttributes `json:"attributes"`
}

type ApplicationAttributes struct {
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Region       string  `json:"region"`
	SlackChannel *string `json:"slack_channel"`
	AvatarURL    *string `json:"avatar_url"`
	CreatedAt    *string `json:"created_at"`
}

type CreateApplicationRequest struct {
	SourceControlProviderType *string `json:"source_control_provider_type,omitempty"`
	Repository                string  `json:"repository"`
	Name                      string  `json:"name"`
	Region                    string  `json:"region"`
	ClusterID                 *string `json:"cluster_id,omitempty"`
}

type UpdateApplicationRequest struct {
	SourceControlProviderType *string `json:"source_control_provider_type,omitempty"`
	Name                      *string `json:"name,omitempty"`
	Slug                      *string `json:"slug,omitempty"`
	DefaultEnvironmentID      *string `json:"default_environment_id,omitempty"`
	Repository                *string `json:"repository,omitempty"`
	SlackChannel              *string `json:"slack_channel,omitempty"`
}

// ---------------------------------------------------------------------
// Environment
// ---------------------------------------------------------------------

type EnvironmentData struct {
	ID         string                `json:"id"`
	Type       string                `json:"type"`
	Attributes EnvironmentAttributes `json:"attributes"`
}

// EnvironmentAttributes mirrors EnvironmentResource.attributes in the public
// OpenAPI spec. Only fields the API actually returns belong here.
//
// Several environment settings -- color, timeout, sleep_timeout,
// shutdown_timeout, uses_purge_edge_cache_on_deploy, uses_vanity_domain -- are
// accepted by PATCH but are *write-only*: they appear nowhere in the response
// schema. Decoding them here would silently yield the zero value on every read
// and overwrite the configured value in state, producing a plan that never
// converges. They are deliberately absent; see mapEnvironmentToState, which
// leaves the configured values untouched.
type EnvironmentAttributes struct {
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Status string `json:"status"`
	// PHPMajorVersion is the read-only major version returned by the API
	// (e.g. "8.4"). The writable php_version sent on update uses the
	// "8.4:1" form, so the two are deliberately distinct fields.
	PHPMajorVersion  string  `json:"php_major_version"`
	NodeVersion      string  `json:"node_version"`
	UsesPushToDeploy bool    `json:"uses_push_to_deploy"`
	UsesDeployHook   bool    `json:"uses_deploy_hook"`
	BuildCommand     *string `json:"build_command"`
	DeployCommand    *string `json:"deploy_command"`
	// VanityDomain is the read-only vanity domain hostname returned by the
	// API (empty/null when none). There is no uses_vanity_domain response
	// flag; presence of a vanity domain is derived from this field.
	VanityDomain          *string            `json:"vanity_domain"`
	UsesOctane            bool               `json:"uses_octane"`
	UsesHibernation       bool               `json:"uses_hibernation"`
	NetworkSettings       EnvironmentNetwork `json:"network_settings"`
	CreatedFromAutomation bool               `json:"created_from_automation"`
	CreatedAt             *string            `json:"created_at"`
}

// EnvironmentNetwork is the response-side network_settings object. The cache
// strategy is readable only from here -- there is no top-level cache_strategy
// attribute, even though PATCH accepts one under that name.
type EnvironmentNetwork struct {
	Cache struct {
		Strategy string `json:"strategy"`
	} `json:"cache"`
}

// AttachID renders an id as a JSON string for the nullable attachment fields
// on UpdateEnvironmentRequest.
func AttachID(id string) json.RawMessage {
	b, err := json.Marshal(id)
	if err != nil {
		// json.Marshal of a string cannot fail; fall back to detaching
		// rather than emitting invalid JSON into the request body.
		return DetachID()
	}
	return b
}

// DetachID renders JSON null, which is how the API is told to detach a
// database, cache or websocket application from an environment.
func DetachID() json.RawMessage { return json.RawMessage("null") }

type CreateEnvironmentRequest struct {
	Branch    string  `json:"branch"`
	Name      string  `json:"name"`
	ClusterID *string `json:"cluster_id,omitempty"`
}

type UpdateEnvironmentRequest struct {
	Name             *string `json:"name,omitempty"`
	Slug             *string `json:"slug,omitempty"`
	Color            *string `json:"color,omitempty"`
	Branch           *string `json:"branch,omitempty"`
	UsesPushToDeploy *bool   `json:"uses_push_to_deploy,omitempty"`
	UsesDeployHook   *bool   `json:"uses_deploy_hook,omitempty"`
	Timeout          *int    `json:"timeout,omitempty"`
	PHPVersion       *string `json:"php_version,omitempty"`
	BuildCommand     *string `json:"build_command,omitempty"`
	NodeVersion      *string `json:"node_version,omitempty"`
	DeployCommand    *string `json:"deploy_command,omitempty"`
	UsesVanityDomain *bool   `json:"uses_vanity_domain,omitempty"`
	// These three are nullable in the spec: JSON null detaches the resource
	// from the environment. An empty string is not null and is rejected, so
	// they are raw JSON rather than *string -- see AttachID / DetachID.
	DatabaseSchemaID           json.RawMessage `json:"database_schema_id,omitempty"`
	CacheID                    json.RawMessage `json:"cache_id,omitempty"`
	WebsocketApplicationID     json.RawMessage `json:"websocket_application_id,omitempty"`
	UsesOctane                 *bool           `json:"uses_octane,omitempty"`
	SleepTimeout               *int            `json:"sleep_timeout,omitempty"`
	ShutdownTimeout            *int            `json:"shutdown_timeout,omitempty"`
	UsesPurgeEdgeCacheOnDeploy *bool           `json:"uses_purge_edge_cache_on_deploy,omitempty"`
	CacheStrategy              *string         `json:"cache_strategy,omitempty"`
}

// ---------------------------------------------------------------------
// Instance
// ---------------------------------------------------------------------

type InstanceData struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Attributes InstanceAttributes `json:"attributes"`
	// Relationships carries the parent link, which is what makes import
	// work: the import id names this resource only.
	Relationships InstanceRelationships `json:"relationships"`
}

type InstanceAttributes struct {
	Name         string `json:"name"`
	InstanceType string `json:"type"`
	Size         string `json:"size"`
	ScalingType  string `json:"scaling_type"`
	// Nullable: an automatically scaled instance has no replica counts, and a
	// plain int would report 0 where the value is absent.
	MinReplicas                      *int64 `json:"min_replicas"`
	MaxReplicas                      *int64 `json:"max_replicas"`
	UsesScheduler                    bool   `json:"uses_scheduler"`
	ScalingCPUThresholdPercentage    *int   `json:"scaling_cpu_threshold_percentage"`
	ScalingMemoryThresholdPercentage *int   `json:"scaling_memory_threshold_percentage"`
	// Managed-queue fields (type == "managed_queue").
	SleepWithApp      *bool `json:"sleep_with_app"`
	VisibilityTimeout *int  `json:"visibility_timeout"`
	PollingInterval   *int  `json:"polling_interval"`
	ShutdownTimeout   *int  `json:"shutdown_timeout"`
	Paused            *bool `json:"paused"`
	IsDefault         *bool `json:"is_default"`
	// QueueStatus is a string enum per the spec (creating, updating,
	// available, deleting, deleted, unknown) or null. It is decoded through
	// QueueStatusValue rather than a plain *string so that a non-string
	// payload degrades to raw JSON instead of failing the whole decode.
	QueueStatus QueueStatusValue `json:"queue_status"`
	CreatedAt   *string          `json:"created_at"`
}

type CreateInstanceRequest struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Size        string `json:"size"`
	ScalingType string `json:"scaling_type"`
	// MinReplicas/MaxReplicas are pointers because the API *rejects* them for
	// the "auto" scaling type ("Only applicable to the custom scaling type,
	// and rejected when used with auto"). They must be omittable.
	MinReplicas                      *int  `json:"min_replicas,omitempty"`
	MaxReplicas                      *int  `json:"max_replicas,omitempty"`
	UsesScheduler                    *bool `json:"uses_scheduler,omitempty"`
	ScalingCPUThresholdPercentage    *int  `json:"scaling_cpu_threshold_percentage,omitempty"`
	ScalingMemoryThresholdPercentage *int  `json:"scaling_memory_threshold_percentage,omitempty"`
	SleepWithApp                     *bool `json:"sleep_with_app,omitempty"`
	VisibilityTimeout                *int  `json:"visibility_timeout,omitempty"`
	ShutdownTimeout                  *int  `json:"shutdown_timeout,omitempty"`
	// polling_interval is deliberately absent: it appears in neither the
	// create nor the update request schema. It is read-only.
}

type UpdateInstanceRequest struct {
	Name                             *string `json:"name,omitempty"`
	Size                             *string `json:"size,omitempty"`
	ScalingType                      *string `json:"scaling_type,omitempty"`
	MinReplicas                      *int    `json:"min_replicas,omitempty"`
	MaxReplicas                      *int    `json:"max_replicas,omitempty"`
	UsesScheduler                    *bool   `json:"uses_scheduler,omitempty"`
	ScalingCPUThresholdPercentage    *int    `json:"scaling_cpu_threshold_percentage,omitempty"`
	ScalingMemoryThresholdPercentage *int    `json:"scaling_memory_threshold_percentage,omitempty"`
	SleepWithApp                     *bool   `json:"sleep_with_app,omitempty"`
	UsesOctane                       *bool   `json:"uses_octane,omitempty"`
	UsesInertiaSSR                   *bool   `json:"uses_inertia_ssr,omitempty"`
	HibernationTimeout               *int    `json:"hibernation_timeout,omitempty"`
	VisibilityTimeout                *int    `json:"visibility_timeout,omitempty"`
	ShutdownTimeout                  *int    `json:"shutdown_timeout,omitempty"`
	// polling_interval is read-only; see CreateInstanceRequest.
}

// ---------------------------------------------------------------------
// Domain
// ---------------------------------------------------------------------

type DomainData struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"`
	Attributes DomainAttributes `json:"attributes"`
	// Relationships carries the parent link, which is what makes import
	// work: the import id names this resource only.
	Relationships DomainRelationships `json:"relationships"`
}

type DomainAttributes struct {
	Name               string  `json:"name"`
	DomainType         string  `json:"type"`
	Stage              string  `json:"stage"`
	HostnameStatus     string  `json:"hostname_status"`
	SSLStatus          string  `json:"ssl_status"`
	OriginStatus       string  `json:"origin_status"`
	Redirect           *string `json:"redirect"`
	CloudflareStrategy *string `json:"cloudflare_strategy"`
	Downtime           *bool   `json:"downtime"`
	WildcardEnabled    bool    `json:"wildcard_enabled"`
	// ActionRequired reports what the operator still has to do
	// (add_txt_records, add_dns_records, failed) before the domain verifies.
	ActionRequired *string `json:"action_required"`
	LastVerifiedAt *string `json:"last_verified_at"`
	// DNSRecords carries the records that must exist for the domain to
	// verify and serve. Without it there is no way to complete the domain
	// workflow from Terraform.
	DNSRecords DomainDNSRecords `json:"dns_records"`
	CreatedAt  *string          `json:"created_at"`
}

// DomainDNSRecords is the dns_records object on a domain.
type DomainDNSRecords struct {
	SSL             []DomainSSLRecord `json:"ssl"`
	PreVerification string            `json:"pre_verification"`
	Origin          string            `json:"origin"`
	OriginCNAME     string            `json:"origin_cname"`
	DCV             string            `json:"dcv"`
}

// DomainSSLRecord is one CNAME/TXT record required for SSL issuance.
type DomainSSLRecord struct {
	Type  string  `json:"type"`
	Name  *string `json:"name"`
	Value *string `json:"value"`
}

type CreateDomainRequest struct {
	Name               string  `json:"name"`
	WWWRedirect        *string `json:"www_redirect,omitempty"`
	WildcardEnabled    *bool   `json:"wildcard_enabled,omitempty"`
	VerificationMethod *string `json:"verification_method,omitempty"`
	CloudflareStrategy *string `json:"cloudflare_strategy,omitempty"`
	AllowDowntime      *bool   `json:"allow_downtime,omitempty"`
}

// UpdateDomainRequest is the entire PATCH /domains/{domain} body: the API
// accepts verification_method and nothing else, and it is required. Every other
// domain setting is create-only, which is why they force replacement.
type UpdateDomainRequest struct {
	VerificationMethod string `json:"verification_method"`
}

// ---------------------------------------------------------------------
// Database Cluster
// ---------------------------------------------------------------------

type DatabaseClusterData struct {
	ID         string                    `json:"id"`
	Type       string                    `json:"type"`
	Attributes DatabaseClusterAttributes `json:"attributes"`
}

type DatabaseClusterAttributes struct {
	Name       string              `json:"name"`
	DBType     string              `json:"type"`
	Status     string              `json:"status"`
	Region     string              `json:"region"`
	Config     map[string]any      `json:"config"`
	Connection *DatabaseConnection `json:"connection"`
	CreatedAt  *string             `json:"created_at"`
}

type DatabaseConnection struct {
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Driver   string `json:"driver"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type CreateDatabaseClusterRequest struct {
	Type string `json:"type"`
	// Version is required by the API for every current database type. The
	// retired identifiers that baked the version into the type (laravel_mysql_84
	// and friends) are still accepted for backwards compatibility and carry
	// their own version, which is why this is omitempty rather than mandatory.
	Version string `json:"version,omitempty"`
	Name    string `json:"name"`
	Region  string `json:"region"`
	// ClusterID is a string in the spec, not a number.
	ClusterID *string `json:"cluster_id,omitempty"`
	// Config is required by the API; it is sent even when empty.
	Config map[string]any `json:"config"`
}

// UpdateDatabaseClusterRequest is the entire PATCH body. config is required,
// so it is sent even when empty rather than dropped by omitempty.
type UpdateDatabaseClusterRequest struct {
	Config map[string]any `json:"config"`
}

// ---------------------------------------------------------------------
// Database (schema-level)
// ---------------------------------------------------------------------

type DatabaseData struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Attributes DatabaseAttributes `json:"attributes"`
}

type DatabaseAttributes struct {
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	CreatedAt *string `json:"created_at"`
}

type CreateDatabaseRequest struct {
	Name string `json:"name"`
}

// ---------------------------------------------------------------------
// Cache
// ---------------------------------------------------------------------

type CacheData struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes CacheAttributes `json:"attributes"`
}

type CacheAttributes struct {
	Name               string           `json:"name"`
	CacheType          string           `json:"type"`
	Status             string           `json:"status"`
	Region             string           `json:"region"`
	Size               string           `json:"size"`
	AutoUpgradeEnabled bool             `json:"auto_upgrade_enabled"`
	IsPublic           bool             `json:"is_public"`
	Connection         *CacheConnection `json:"connection"`
	CreatedAt          *string          `json:"created_at"`
}

// CacheConnection is the cache's connection block. Every field is nullable:
// a cache that is still provisioning returns nulls, and the credentials are
// merged in conditionally. Pointers keep "not reported yet" distinguishable
// from an empty hostname or a zero port.
type CacheConnection struct {
	Hostname *string `json:"hostname"`
	Port     *int64  `json:"port"`
	Protocol *string `json:"protocol"`
	Username *string `json:"username"`
	Password *string `json:"password"`
}

// IsReady reports whether the API has populated the connection block. A cache
// created a moment ago is still provisioning and reports an empty connection.
func (c *CacheConnection) IsReady() bool {
	return c != nil && c.Hostname != nil && *c.Hostname != "" && c.Port != nil && *c.Port != 0
}

type CreateCacheRequest struct {
	Type               string  `json:"type"`
	Name               string  `json:"name"`
	Region             string  `json:"region"`
	Size               string  `json:"size"`
	AutoUpgradeEnabled bool    `json:"auto_upgrade_enabled"`
	IsPublic           bool    `json:"is_public"`
	EvictionPolicy     *string `json:"eviction_policy,omitempty"`
}

type UpdateCacheRequest struct {
	Name               *string `json:"name,omitempty"`
	Size               *string `json:"size,omitempty"`
	AutoUpgradeEnabled *bool   `json:"auto_upgrade_enabled,omitempty"`
	IsPublic           *bool   `json:"is_public,omitempty"`
	EvictionPolicy     *string `json:"eviction_policy,omitempty"`
}

// ---------------------------------------------------------------------
// Object Storage Bucket
// ---------------------------------------------------------------------

type StorageBucketData struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"`
	Attributes StorageBucketAttributes `json:"attributes"`
}

type StorageBucketAttributes struct {
	Name         string  `json:"name"`
	BucketType   string  `json:"type"`
	Status       string  `json:"status"`
	Visibility   string  `json:"visibility"`
	Jurisdiction string  `json:"jurisdiction"`
	Endpoint     *string `json:"endpoint"`
	URL          *string `json:"url"`
	// AllowedOrigins is deprecated by the API (removed May 17, 2026) in
	// favour of CorsSettings.
	AllowedOrigins []string `json:"allowed_origins"`
	// CorsSettings is captured as raw JSON because the JSON:API response
	// shape for it is not stable; it is surfaced as a string attribute.
	CorsSettings json.RawMessage `json:"cors_settings"`
	CreatedAt    *string         `json:"created_at"`
}

// CorsSettings is the modern CORS configuration object accepted on bucket
// create/update, superseding the deprecated top-level allowed_origins field.
type CorsSettings struct {
	AllowedOrigins []string `json:"allowed_origins,omitempty"`
	AllowedMethods []string `json:"allowed_methods,omitempty"`
	AllowedHeaders []string `json:"allowed_headers,omitempty"`
	ExposeHeaders  []string `json:"expose_headers,omitempty"`
	MaxAgeSeconds  *int     `json:"max_age_seconds,omitempty"`
}

type UpdateStorageBucketRequest struct {
	Name         *string       `json:"name,omitempty"`
	Visibility   *string       `json:"visibility,omitempty"`
	CorsSettings *CorsSettings `json:"cors_settings,omitempty"`
	// Deprecated: use CorsSettings. Removed from the API May 17, 2026.
	AllowedOrigins []string `json:"allowed_origins,omitempty"`
}

type CreateStorageBucketRequest struct {
	Name          string        `json:"name"`
	Visibility    string        `json:"visibility"`
	Jurisdiction  string        `json:"jurisdiction"`
	KeyName       string        `json:"key_name"`
	KeyPermission string        `json:"key_permission"`
	CorsSettings  *CorsSettings `json:"cors_settings,omitempty"`
	// Deprecated: use CorsSettings. Removed from the API May 17, 2026.
	AllowedOrigins []string `json:"allowed_origins,omitempty"`
}

// ---------------------------------------------------------------------
// Organization (read-only)
// ---------------------------------------------------------------------

type OrganizationData struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Attributes OrganizationAttributes `json:"attributes"`
}

type OrganizationAttributes struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// ---------------------------------------------------------------------
// Background Process (Daemon)
// ---------------------------------------------------------------------

type BackgroundProcessData struct {
	ID         string                      `json:"id"`
	Type       string                      `json:"type"`
	Attributes BackgroundProcessAttributes `json:"attributes"`
	// Relationships carries the parent link, which is what makes import
	// work: the import id names this resource only.
	Relationships BackgroundProcessRelationships `json:"relationships"`
}

type BackgroundProcessAttributes struct {
	ProcessType string  `json:"type"`
	Processes   int     `json:"processes"`
	Command     *string `json:"command"`
	// Config is returned by the API as an array of objects (the request
	// body sends it as a single object), so it is captured as raw JSON to
	// tolerate either shape rather than forcing a map decode.
	Config            json.RawMessage `json:"config"`
	StrategyType      string          `json:"strategy_type"`
	StrategyThreshold *int            `json:"strategy_threshold"`
	CreatedAt         *string         `json:"created_at"`
}

type CreateBackgroundProcessRequest struct {
	Type      string         `json:"type"`
	Processes int            `json:"processes"`
	Command   *string        `json:"command,omitempty"`
	Config    map[string]any `json:"config,omitempty"`
}

type UpdateBackgroundProcessRequest struct {
	Processes *int           `json:"processes,omitempty"`
	Command   *string        `json:"command,omitempty"`
	Config    map[string]any `json:"config,omitempty"`
}

// ---------------------------------------------------------------------
// WebSocket Server (Cluster)
// ---------------------------------------------------------------------

type WebsocketServerData struct {
	ID         string                    `json:"id"`
	Type       string                    `json:"type"`
	Attributes WebsocketServerAttributes `json:"attributes"`
}

type WebsocketServerAttributes struct {
	Name                           string  `json:"name"`
	ServerType                     string  `json:"type"`
	Region                         string  `json:"region"`
	Status                         string  `json:"status"`
	MaxConnections                 int     `json:"max_connections"`
	ConnectionDistributionStrategy string  `json:"connection_distribution_strategy"`
	Hostname                       string  `json:"hostname"`
	CreatedAt                      *string `json:"created_at"`
}

type CreateWebsocketServerRequest struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Region         string `json:"region"`
	MaxConnections int    `json:"max_connections"`
}

type UpdateWebsocketServerRequest struct {
	Name           *string `json:"name,omitempty"`
	MaxConnections *int    `json:"max_connections,omitempty"`
}

// ---------------------------------------------------------------------
// WebSocket Application
// ---------------------------------------------------------------------

type WebsocketApplicationData struct {
	ID         string                         `json:"id"`
	Type       string                         `json:"type"`
	Attributes WebsocketApplicationAttributes `json:"attributes"`
}

type WebsocketApplicationAttributes struct {
	Name            string   `json:"name"`
	AppID           string   `json:"app_id"`
	AllowedOrigins  []string `json:"allowed_origins"`
	PingInterval    int      `json:"ping_interval"`
	ActivityTimeout int      `json:"activity_timeout"`
	MaxMessageSize  int      `json:"max_message_size"`
	MaxConnections  int      `json:"max_connections"`
	Key             string   `json:"key"`
	Secret          string   `json:"secret"`
	CreatedAt       *string  `json:"created_at"`
}

type CreateWebsocketApplicationRequest struct {
	Name            string   `json:"name"`
	AllowedOrigins  []string `json:"allowed_origins,omitempty"`
	PingInterval    *int     `json:"ping_interval,omitempty"`
	ActivityTimeout *int     `json:"activity_timeout,omitempty"`
}

type UpdateWebsocketApplicationRequest struct {
	Name *string `json:"name,omitempty"`
	// AllowedOrigins is a pointer to a slice, not a plain slice, because an
	// empty array is how origins are cleared. With `[]string` + omitempty an
	// empty list marshals to nothing, the field is dropped from the body, and
	// the API keeps the previous origins -- so clearing them was impossible.
	// A nil pointer omits the field; a pointer to an empty slice sends [].
	AllowedOrigins  *[]string `json:"allowed_origins,omitempty"`
	PingInterval    *int      `json:"ping_interval,omitempty"`
	ActivityTimeout *int      `json:"activity_timeout,omitempty"`
}

// ---------------------------------------------------------------------
// Storage Bucket Key
// ---------------------------------------------------------------------

type StorageBucketKeyData struct {
	ID         string                     `json:"id"`
	Type       string                     `json:"type"`
	Attributes StorageBucketKeyAttributes `json:"attributes"`
}

type StorageBucketKeyAttributes struct {
	Name       string `json:"name"`
	Permission string `json:"permission"`
	// Both credentials are nullable: the API returns them on create but may
	// answer null on a later read, so they are pointers to keep "absent"
	// distinguishable from "empty". Decoding them as plain strings would turn a
	// null into "" and overwrite the stored secret on the first refresh.
	AccessKeyID     *string `json:"access_key_id"`
	AccessKeySecret *string `json:"access_key_secret"`
	CreatedAt       *string `json:"created_at"`
}

type CreateStorageBucketKeyRequest struct {
	Name       string `json:"name"`
	Permission string `json:"permission"`
}

type UpdateStorageBucketKeyRequest struct {
	Name string `json:"name"`
}

// ---------------------------------------------------------------------
// Environment Variables
// ---------------------------------------------------------------------

type EnvironmentVariable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type AddEnvironmentVariablesRequest struct {
	Method    string                `json:"method"`
	Variables []EnvironmentVariable `json:"variables"`
}

// ---------------------------------------------------------------------
// Deployment
// ---------------------------------------------------------------------

type DeploymentData struct {
	ID         string               `json:"id"`
	Type       string               `json:"type"`
	Attributes DeploymentAttributes `json:"attributes"`
}

type DeploymentAttributes struct {
	Status          string  `json:"status"`
	BranchName      string  `json:"branch_name"`
	CommitHash      string  `json:"commit_hash"`
	CommitMessage   string  `json:"commit_message"`
	CommitAuthor    string  `json:"commit_author"`
	FailureReason   *string `json:"failure_reason"`
	PHPMajorVersion string  `json:"php_major_version"`
	BuildCommand    string  `json:"build_command"`
	NodeVersion     string  `json:"node_version"`
	UsesOctane      bool    `json:"uses_octane"`
	StartedAt       *string `json:"started_at"`
	FinishedAt      *string `json:"finished_at"`
}

// ---------------------------------------------------------------------
// Database Snapshot
// ---------------------------------------------------------------------

type DatabaseSnapshotData struct {
	ID         string                     `json:"id"`
	Type       string                     `json:"type"`
	Attributes DatabaseSnapshotAttributes `json:"attributes"`
}

type DatabaseSnapshotAttributes struct {
	// Name is nullable in the spec: snapshots the platform creates on a
	// schedule have none. A plain string would collapse null to "" and, since
	// name forces replacement, make an imported scheduled snapshot look like
	// it needed recreating.
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	SnapshotType string  `json:"type"`
	Status       string  `json:"status"`
	StorageBytes *int64  `json:"storage_bytes"`
	PITREnabled  bool    `json:"pitr_enabled"`
	PITREndsAt   *string `json:"pitr_ends_at"`
	CompletedAt  *string `json:"completed_at"`
	CreatedAt    *string `json:"created_at"`
}

type CreateDatabaseSnapshotRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

// ---------------------------------------------------------------------
// Command
// ---------------------------------------------------------------------

type CommandData struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Attributes CommandAttributes `json:"attributes"`
}

type CommandAttributes struct {
	Command       string  `json:"command"`
	Output        *string `json:"output"`
	Status        string  `json:"status"`
	ExitCode      *int    `json:"exit_code"`
	FailureReason *string `json:"failure_reason"`
	StartedAt     *string `json:"started_at"`
	FinishedAt    *string `json:"finished_at"`
	CreatedAt     *string `json:"created_at"`
}

type CreateCommandRequest struct {
	Command string `json:"command"`
}

// ---------------------------------------------------------------------
// Database Restore
// ---------------------------------------------------------------------

type CreateDatabaseRestoreRequest struct {
	Name               string  `json:"name"`
	RestoreTime        *string `json:"restore_time,omitempty"`
	DatabaseSnapshotID *string `json:"database_snapshot_id,omitempty"`
}

// ---------------------------------------------------------------------
// Region (read-only)
// ---------------------------------------------------------------------

type RegionInfo struct {
	// The API names this property "region", not "name". Decoding it as "name"
	// left the identifier empty, and an empty region is rejected on create.
	Name string `json:"region"`
	// Label is a human-readable name, e.g. "US East (Ohio)".
	Label string `json:"label"`
	// Flag is a country identifier such as "us" or "germany", not an emoji.
	Flag string `json:"flag"`
}

// ---------------------------------------------------------------------
// IP Addresses (read-only)
// ---------------------------------------------------------------------

// IPAddressesResponse is the GET /ip payload. Unlike every other route this is
// not a JSON:API document and has no "data" envelope: it is a bare object keyed
// by region, each holding that region's egress addresses.
//
//	{"us-east-1": {"ipv4": ["3.1.2.3"], "ipv6": ["2600:1f16::/56"]}, ...}
type IPAddressesResponse map[string]RegionIPAddresses

type RegionIPAddresses struct {
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

// ---------------------------------------------------------------------
// Dedicated Cluster (read-only)
// ---------------------------------------------------------------------

type DedicatedClusterData struct {
	ID         string                     `json:"id"`
	Type       string                     `json:"type"`
	Attributes DedicatedClusterAttributes `json:"attributes"`
}

type DedicatedClusterAttributes struct {
	Name        string  `json:"name"`
	Region      string  `json:"region"`
	ClusterType string  `json:"type"`
	TenancyType string  `json:"tenancy_type"`
	Status      string  `json:"status"`
	CreatedAt   *string `json:"created_at"`
}

// ---------------------------------------------------------------------
// Data source response types (non-JSON:API)
// ---------------------------------------------------------------------

// InstanceSizesResponse is the GET /instances/sizes payload. The spec requires
// BOTH classes; modelling only "general" silently dropped every managed-queue
// size from the data source.
type InstanceSizesResponse struct {
	Data struct {
		General      []InstanceSizeInfo `json:"general"`
		ManagedQueue []InstanceSizeInfo `json:"managed_queue"`
	} `json:"data"`
}

type InstanceSizeInfo struct {
	Name         string `json:"name"`
	Label        string `json:"label"`
	Description  string `json:"description"`
	CPUType      string `json:"cpu_type"`
	ComputeClass string `json:"compute_class"`
	// CPUCount is a float because managed-queue sizes express fractional
	// vCPUs (the spec types it "integer" for general and "number" for
	// managed_queue); an int here fails to unmarshal e.g. 0.5.
	CPUCount  float64 `json:"cpu_count"`
	MemoryMiB int     `json:"memory_mib"`
}

type DatabaseTypesResponse struct {
	Data []DatabaseTypeInfo `json:"data"`
}

type DatabaseTypeInfo struct {
	Type string `json:"type"`
	// Versions lists the engine versions accepted alongside this type. It is
	// required in the response and is what a caller needs to populate the
	// required `version` field when creating a cluster.
	Versions     []string         `json:"versions"`
	Label        string           `json:"label"`
	Regions      []string         `json:"regions"`
	ConfigSchema []map[string]any `json:"config_schema"`
	Sizes        []string         `json:"-"` // extracted from config_schema after unmarshalling
}

type CacheTypesResponse struct {
	Data []CacheTypeInfo `json:"data"`
}

type CacheTypeInfo struct {
	Type                string          `json:"type"`
	Label               string          `json:"label"`
	Regions             []string        `json:"regions"`
	Sizes               []CacheSizeInfo `json:"sizes"`
	SupportsAutoUpgrade bool            `json:"supports_auto_upgrade"`
}

type CacheSizeInfo struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// QueueStatusValue holds the managed-queue status. The public spec types it as
// a string enum, but the value is decoded defensively: a JSON string is
// unquoted (storing it raw put literal quote characters into Terraform state),
// while any other shape is preserved verbatim so that an API that returns, say,
// a status object does not break the entire instance read.
type QueueStatusValue struct {
	// Value is the status as a bare string, or the compact JSON encoding of
	// whatever non-string payload arrived. It is empty when absent or null.
	Value string
}

func (q *QueueStatusValue) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		q.Value = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		q.Value = s
		return nil
	}
	q.Value = string(b)
	return nil
}

func (q QueueStatusValue) MarshalJSON() ([]byte, error) {
	if q.Value == "" {
		return []byte("null"), nil
	}
	return json.Marshal(q.Value)
}

// IsZero reports whether no queue status was returned.
func (q QueueStatusValue) IsZero() bool { return q.Value == "" }

// ---------------------------------------------------------------------
// Edge Network
// ---------------------------------------------------------------------

type EdgeNetworkData struct {
	ID         string                `json:"id"`
	Type       string                `json:"type"`
	Attributes EdgeNetworkAttributes `json:"attributes"`
}

type EdgeNetworkAttributes struct {
	Name        string  `json:"name"`
	Domain      string  `json:"domain"`
	TenancyType string  `json:"tenancy_type"`
	Status      string  `json:"status"`
	CreatedAt   *string `json:"created_at"`
}

// ---------------------------------------------------------------------
// JSON:API relationships
// ---------------------------------------------------------------------

// Relationship is a JSON:API to-one relationship. It is how a resource's
// parent is discovered on import, where the parent id is not in the import
// string and cannot be inferred from anything else in state.
type Relationship struct {
	Data *ResourceIdentifier `json:"data"`
}

// ResourceIdentifier is a JSON:API resource identifier object.
type ResourceIdentifier struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// ID returns the related resource's id, or "" when the relationship is absent
// or explicitly null.
func (r Relationship) RelatedID() string {
	if r.Data == nil {
		return ""
	}
	return r.Data.ID
}

// InstanceRelationships are the relationships on an instance resource object.
type InstanceRelationships struct {
	Environment Relationship `json:"environment"`
}

// DomainRelationships are the relationships on a domain resource object.
type DomainRelationships struct {
	Environment Relationship `json:"environment"`
}

// BackgroundProcessRelationships are the relationships on a background process.
type BackgroundProcessRelationships struct {
	Instance Relationship `json:"instance"`
}
