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

type EnvironmentAttributes struct {
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Status string `json:"status"`
	Color  string `json:"color"`
	// PHPMajorVersion is the read-only major version returned by the API
	// (e.g. "8.4"). The writable php_version sent on update uses the
	// "8.4:1" form, so the two are deliberately distinct fields.
	PHPMajorVersion  string  `json:"php_major_version"`
	NodeVersion      string  `json:"node_version"`
	UsesPushToDeploy bool    `json:"uses_push_to_deploy"`
	UsesDeployHook   bool    `json:"uses_deploy_hook"`
	Timeout          int     `json:"timeout"`
	BuildCommand     *string `json:"build_command"`
	DeployCommand    *string `json:"deploy_command"`
	// VanityDomain is the read-only vanity domain hostname returned by the
	// API (empty/null when none). There is no uses_vanity_domain response
	// flag; presence of a vanity domain is derived from this field.
	VanityDomain               *string `json:"vanity_domain"`
	UsesOctane                 bool    `json:"uses_octane"`
	SleepTimeout               int     `json:"sleep_timeout"`
	ShutdownTimeout            int     `json:"shutdown_timeout"`
	UsesPurgeEdgeCacheOnDeploy bool    `json:"uses_purge_edge_cache_on_deploy"`
	CacheStrategy              string  `json:"cache_strategy"`
	CreatedFromAutomation      bool    `json:"created_from_automation"`
	CreatedAt                  *string `json:"created_at"`
}

type CreateEnvironmentRequest struct {
	Branch    string  `json:"branch"`
	Name      string  `json:"name"`
	ClusterID *string `json:"cluster_id,omitempty"`
}

type UpdateEnvironmentRequest struct {
	Name                       *string `json:"name,omitempty"`
	Slug                       *string `json:"slug,omitempty"`
	Color                      *string `json:"color,omitempty"`
	Branch                     *string `json:"branch,omitempty"`
	UsesPushToDeploy           *bool   `json:"uses_push_to_deploy,omitempty"`
	UsesDeployHook             *bool   `json:"uses_deploy_hook,omitempty"`
	Timeout                    *int    `json:"timeout,omitempty"`
	PHPVersion                 *string `json:"php_version,omitempty"`
	BuildCommand               *string `json:"build_command,omitempty"`
	NodeVersion                *string `json:"node_version,omitempty"`
	DeployCommand              *string `json:"deploy_command,omitempty"`
	UsesVanityDomain           *bool   `json:"uses_vanity_domain,omitempty"`
	DatabaseSchemaID           *string `json:"database_schema_id,omitempty"`
	CacheID                    *string `json:"cache_id,omitempty"`
	WebsocketApplicationID     *string `json:"websocket_application_id,omitempty"`
	UsesOctane                 *bool   `json:"uses_octane,omitempty"`
	SleepTimeout               *int    `json:"sleep_timeout,omitempty"`
	ShutdownTimeout            *int    `json:"shutdown_timeout,omitempty"`
	UsesPurgeEdgeCacheOnDeploy *bool   `json:"uses_purge_edge_cache_on_deploy,omitempty"`
	CacheStrategy              *string `json:"cache_strategy,omitempty"`
}

// ---------------------------------------------------------------------
// Instance
// ---------------------------------------------------------------------

type InstanceData struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Attributes InstanceAttributes `json:"attributes"`
}

type InstanceAttributes struct {
	Name                             string `json:"name"`
	InstanceType                     string `json:"type"`
	Size                             string `json:"size"`
	ScalingType                      string `json:"scaling_type"`
	MinReplicas                      int    `json:"min_replicas"`
	MaxReplicas                      int    `json:"max_replicas"`
	UsesScheduler                    bool   `json:"uses_scheduler"`
	ScalingCPUThresholdPercentage    *int   `json:"scaling_cpu_threshold_percentage"`
	ScalingMemoryThresholdPercentage *int   `json:"scaling_memory_threshold_percentage"`
	// Managed-queue fields (type == "managed_queue").
	SleepWithApp      *bool           `json:"sleep_with_app"`
	VisibilityTimeout *int            `json:"visibility_timeout"`
	PollingInterval   *int            `json:"polling_interval"`
	ShutdownTimeout   *int            `json:"shutdown_timeout"`
	Paused            *bool           `json:"paused"`
	IsDefault         *bool           `json:"is_default"`
	QueueStatus       json.RawMessage `json:"queue_status"`
	CreatedAt         *string         `json:"created_at"`
}

type CreateInstanceRequest struct {
	Name                             string `json:"name"`
	Type                             string `json:"type"`
	Size                             string `json:"size"`
	ScalingType                      string `json:"scaling_type"`
	MinReplicas                      int    `json:"min_replicas"`
	MaxReplicas                      int    `json:"max_replicas"`
	UsesScheduler                    *bool  `json:"uses_scheduler,omitempty"`
	ScalingCPUThresholdPercentage    *int   `json:"scaling_cpu_threshold_percentage,omitempty"`
	ScalingMemoryThresholdPercentage *int   `json:"scaling_memory_threshold_percentage,omitempty"`
	SleepWithApp                     *bool  `json:"sleep_with_app,omitempty"`
	VisibilityTimeout                *int   `json:"visibility_timeout,omitempty"`
	PollingInterval                  *int   `json:"polling_interval,omitempty"`
	ShutdownTimeout                  *int   `json:"shutdown_timeout,omitempty"`
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
	PollingInterval                  *int    `json:"polling_interval,omitempty"`
	ShutdownTimeout                  *int    `json:"shutdown_timeout,omitempty"`
}

// ---------------------------------------------------------------------
// Domain
// ---------------------------------------------------------------------

type DomainData struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"`
	Attributes DomainAttributes `json:"attributes"`
}

type DomainAttributes struct {
	Name               string  `json:"name"`
	DomainType         string  `json:"type"`
	HostnameStatus     string  `json:"hostname_status"`
	SSLStatus          string  `json:"ssl_status"`
	OriginStatus       string  `json:"origin_status"`
	Redirect           *string `json:"redirect"`
	CloudflareStrategy *string `json:"cloudflare_strategy"`
	Downtime           *bool   `json:"downtime"`
	CreatedAt          *string `json:"created_at"`
}

type CreateDomainRequest struct {
	Name               string  `json:"name"`
	WWWRedirect        *string `json:"www_redirect,omitempty"`
	WildcardEnabled    *bool   `json:"wildcard_enabled,omitempty"`
	VerificationMethod *string `json:"verification_method,omitempty"`
	CloudflareStrategy *string `json:"cloudflare_strategy,omitempty"`
	AllowDowntime      *bool   `json:"allow_downtime,omitempty"`
}

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
	Type      string         `json:"type"`
	Name      string         `json:"name"`
	Region    string         `json:"region"`
	ClusterID *int           `json:"cluster_id,omitempty"`
	Config    map[string]any `json:"config,omitempty"`
}

type UpdateDatabaseClusterRequest struct {
	Config map[string]any `json:"config,omitempty"`
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

type CacheConnection struct {
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Username string `json:"username"`
	Password string `json:"password"`
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
	Name            *string  `json:"name,omitempty"`
	AllowedOrigins  []string `json:"allowed_origins,omitempty"`
	PingInterval    *int     `json:"ping_interval,omitempty"`
	ActivityTimeout *int     `json:"activity_timeout,omitempty"`
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
	Name         string  `json:"name"`
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

type InstanceSizesResponse struct {
	Data struct {
		General []InstanceSizeInfo `json:"general"`
	} `json:"data"`
}

type InstanceSizeInfo struct {
	Name         string `json:"name"`
	Label        string `json:"label"`
	Description  string `json:"description"`
	CPUType      string `json:"cpu_type"`
	ComputeClass string `json:"compute_class"`
	CPUCount     int    `json:"cpu_count"`
	MemoryMiB    int    `json:"memory_mib"`
}

type DatabaseTypesResponse struct {
	Data []DatabaseTypeInfo `json:"data"`
}

type DatabaseTypeInfo struct {
	Type         string           `json:"type"`
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
