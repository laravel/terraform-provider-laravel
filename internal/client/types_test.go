package client

import (
	"encoding/json"
	"strings"
	"testing"
)

// These tests assert that the client structs decode payloads shaped like the
// current Laravel Cloud public API responses. They guard the field-tag fixes
// made when aligning the provider with the API and run fully offline.

func TestEnvironmentAttributes_Unmarshal(t *testing.T) {
	// The API returns php_major_version (not php_version) and a vanity_domain
	// string (there is no uses_vanity_domain response flag).
	const body = `{
		"name": "production",
		"slug": "production",
		"status": "running",
		"php_major_version": "8.4",
		"vanity_domain": "prod-abc.laravel.cloud",
		"created_from_automation": true,
		"uses_octane": true
	}`

	var attrs EnvironmentAttributes
	if err := json.Unmarshal([]byte(body), &attrs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if attrs.PHPMajorVersion != "8.4" {
		t.Errorf("PHPMajorVersion = %q, want %q", attrs.PHPMajorVersion, "8.4")
	}
	if attrs.VanityDomain == nil || *attrs.VanityDomain != "prod-abc.laravel.cloud" {
		t.Errorf("VanityDomain = %v, want prod-abc.laravel.cloud", attrs.VanityDomain)
	}
	if attrs.Status != "running" {
		t.Errorf("Status = %q, want running", attrs.Status)
	}
	if !attrs.CreatedFromAutomation {
		t.Error("CreatedFromAutomation = false, want true")
	}
}

func TestEnvironmentAttributes_NoVanityDomain(t *testing.T) {
	var attrs EnvironmentAttributes
	if err := json.Unmarshal([]byte(`{"name":"staging","vanity_domain":null}`), &attrs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if attrs.VanityDomain != nil {
		t.Errorf("VanityDomain = %v, want nil", attrs.VanityDomain)
	}
}

func TestBackgroundProcessAttributes_ConfigIsArray(t *testing.T) {
	// The API echoes config back as an array of objects even though it is sent
	// as a single object; json.RawMessage must tolerate the array shape.
	const body = `{
		"type": "worker",
		"processes": 2,
		"config": [{"connection": "redis", "queue": "default"}],
		"strategy_type": "queue_size",
		"strategy_threshold": 100
	}`

	var attrs BackgroundProcessAttributes
	if err := json.Unmarshal([]byte(body), &attrs); err != nil {
		t.Fatalf("unmarshal with array config: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(attrs.Config)), "[") {
		t.Errorf("Config = %s, want a JSON array", attrs.Config)
	}
	if attrs.StrategyThreshold == nil || *attrs.StrategyThreshold != 100 {
		t.Errorf("StrategyThreshold = %v, want 100", attrs.StrategyThreshold)
	}
}

func TestBackgroundProcessAttributes_NullStrategyThreshold(t *testing.T) {
	var attrs BackgroundProcessAttributes
	if err := json.Unmarshal([]byte(`{"type":"worker","processes":1,"strategy_threshold":null}`), &attrs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if attrs.StrategyThreshold != nil {
		t.Errorf("StrategyThreshold = %v, want nil", attrs.StrategyThreshold)
	}
}

func TestDatabaseClusterAttributes_Connection(t *testing.T) {
	const body = `{
		"name": "db",
		"type": "laravel_mysql_84",
		"status": "available",
		"region": "us-east-2",
		"connection": {
			"hostname": "db.example.com",
			"port": 3306,
			"protocol": "tcp",
			"driver": "mysql",
			"username": "forge",
			"password": "secret"
		}
	}`

	var attrs DatabaseClusterAttributes
	if err := json.Unmarshal([]byte(body), &attrs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if attrs.Connection == nil {
		t.Fatal("Connection = nil, want populated")
	}
	if attrs.Connection.Hostname != "db.example.com" || attrs.Connection.Port != 3306 {
		t.Errorf("Connection = %+v, want hostname db.example.com port 3306", *attrs.Connection)
	}
}

func TestDomainAttributes_CloudflareAndDowntime(t *testing.T) {
	const body = `{"name":"example.com","type":"root","cloudflare_strategy":"dns_proxy","downtime":false}`
	var attrs DomainAttributes
	if err := json.Unmarshal([]byte(body), &attrs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if attrs.CloudflareStrategy == nil || *attrs.CloudflareStrategy != "dns_proxy" {
		t.Errorf("CloudflareStrategy = %v, want dns_proxy", attrs.CloudflareStrategy)
	}
	if attrs.Downtime == nil || *attrs.Downtime {
		t.Errorf("Downtime = %v, want false", attrs.Downtime)
	}
}

func TestOrganizationAttributes_Slug(t *testing.T) {
	var attrs OrganizationAttributes
	if err := json.Unmarshal([]byte(`{"name":"Acme","slug":"acme"}`), &attrs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if attrs.Slug != "acme" {
		t.Errorf("Slug = %q, want acme", attrs.Slug)
	}
}

func TestInstanceAttributes_ManagedQueue(t *testing.T) {
	const body = `{
		"name": "queue",
		"type": "managed_queue",
		"size": "worker-1",
		"scaling_type": "fixed",
		"min_replicas": 1,
		"max_replicas": 1,
		"visibility_timeout": 30,
		"polling_interval": 5,
		"paused": false,
		"is_default": true,
		"queue_status": "available"
	}`

	var attrs InstanceAttributes
	if err := json.Unmarshal([]byte(body), &attrs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if attrs.InstanceType != "managed_queue" {
		t.Errorf("InstanceType = %q, want managed_queue", attrs.InstanceType)
	}
	if attrs.VisibilityTimeout == nil || *attrs.VisibilityTimeout != 30 {
		t.Errorf("VisibilityTimeout = %v, want 30", attrs.VisibilityTimeout)
	}
	if attrs.IsDefault == nil || !*attrs.IsDefault {
		t.Errorf("IsDefault = %v, want true", attrs.IsDefault)
	}
	// The spec types queue_status as a bare string enum. It must land in the
	// struct unquoted -- storing the raw JSON put literal quotes into state.
	if attrs.QueueStatus.Value != "available" {
		t.Errorf("QueueStatus = %q, want available", attrs.QueueStatus.Value)
	}
}

// TestInstanceAttributes_QueueStatusNonString pins the defensive path: if the
// API ever answers with something other than a string, the instance read must
// still succeed rather than failing to decode entirely.
func TestInstanceAttributes_QueueStatusNonString(t *testing.T) {
	var attrs InstanceAttributes
	body := `{"name":"queue","queue_status":{"available":0,"delayed":2}}`
	if err := json.Unmarshal([]byte(body), &attrs); err != nil {
		t.Fatalf("unmarshal must not fail on a non-string queue_status: %v", err)
	}
	if attrs.QueueStatus.Value != `{"available":0,"delayed":2}` {
		t.Errorf("QueueStatus = %q, want the raw JSON preserved", attrs.QueueStatus.Value)
	}
}

// TestInstanceAttributes_QueueStatusNull covers a non-queue instance.
func TestInstanceAttributes_QueueStatusNull(t *testing.T) {
	var attrs InstanceAttributes
	if err := json.Unmarshal([]byte(`{"name":"web","queue_status":null}`), &attrs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !attrs.QueueStatus.IsZero() {
		t.Errorf("QueueStatus = %q, want zero", attrs.QueueStatus.Value)
	}
}

func TestStorageBucketAttributes_CorsSettings(t *testing.T) {
	const body = `{
		"name": "assets",
		"type": "object_storage",
		"visibility": "public",
		"allowed_origins": ["https://a.com"],
		"cors_settings": {"allowed_origins": ["https://a.com"], "max_age_seconds": 3600}
	}`

	var attrs StorageBucketAttributes
	if err := json.Unmarshal([]byte(body), &attrs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(attrs.AllowedOrigins) != 1 || attrs.AllowedOrigins[0] != "https://a.com" {
		t.Errorf("AllowedOrigins = %v, want [https://a.com]", attrs.AllowedOrigins)
	}
	if len(attrs.CorsSettings) == 0 {
		t.Error("CorsSettings is empty, want raw JSON object")
	}
}

// GET /meta/regions names the identifier "region", not "name", and returns a
// country slug for "flag" rather than an emoji. This fixture is a verbatim
// element from the live response. The previous version of this test invented a
// "name" key and asserted an emoji, and never checked the identifier at all --
// so it passed while the identifier decoded to "" and every consumer that fed
// it to a resource's region argument sent an empty string and got a 422.
func TestRegionInfo_UsesRealResponseKeys(t *testing.T) {
	var r RegionInfo
	if err := json.Unmarshal([]byte(`{"region":"us-east-2","label":"US East (Ohio)","flag":"us"}`), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.Name != "us-east-2" {
		t.Errorf("Name = %q, want %q -- the JSON key is \"region\"", r.Name, "us-east-2")
	}
	if r.Label != "US East (Ohio)" {
		t.Errorf("Label = %q, want %q", r.Label, "US East (Ohio)")
	}
	if r.Flag != "us" {
		t.Errorf("Flag = %q, want %q (a country slug, not an emoji)", r.Flag, "us")
	}
}

// A payload carrying "name" is not something the API produces; if one ever
// appears the identifier must stay empty rather than silently half-decoding.
func TestRegionInfo_IgnoresLegacyNameKey(t *testing.T) {
	var r RegionInfo
	if err := json.Unmarshal([]byte(`{"name":"us-east-2","label":"Ohio","flag":"us"}`), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.Name != "" {
		t.Errorf("Name = %q, want empty -- only \"region\" carries the identifier", r.Name)
	}
}

func TestDocumentEnvelope_Organization(t *testing.T) {
	// Exercises the full JSON:API document path used by the client.
	const body = `{"data":{"id":"org_1","type":"organizations","attributes":{"name":"Acme","slug":"acme"}}}`
	var doc Document[OrganizationData]
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.Data.ID != "org_1" || doc.Data.Attributes.Slug != "acme" {
		t.Errorf("doc.Data = %+v, want id org_1 slug acme", doc.Data)
	}
}
