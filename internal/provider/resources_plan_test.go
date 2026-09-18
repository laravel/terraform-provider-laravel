package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// This file plan-checks every resource the provider exposes against the
// in-memory fake API (see mock_cloud_test.go / mock_generic_test.go). Each test
// stands up the minimal real parent stack, asserts the create plan and a few
// applied computed values, then re-applies the same config and asserts an empty
// plan — the round-trip idempotency check that catches perpetual diffs.
//
// Like TestBasicAppPlan these need TF_ACC=1 and a terraform binary, but no token
// and no network. Run them with `make testplan`.

// runPlanStability runs the standard two-step check: apply (asserting the create
// plan and state), then re-apply the same config asserting an empty plan.
func runPlanStability(t *testing.T, config string, createPlan []plancheck.PlanCheck, state []statecheck.StateCheck) {
	t.Helper()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:            config,
				ConfigPlanChecks:  resource.ConfigPlanChecks{PreApply: createPlan},
				ConfigStateChecks: state,
			},
			{
				Config:           config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// providerCfg returns the provider block pointed at the fake API.
func providerCfg(baseURL string) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %q
}
`, baseURL)
}

// appEnvStack returns an application + environment the env-scoped resources hang
// off of (addresses laravel_cloud_application.app / laravel_cloud_environment.env).
const appEnvStack = `
resource "laravel_cloud_application" "app" {
  name                         = "stack-app"
  repository                   = "laravel/laravel"
  region                       = "us-east-2"
  source_control_provider_type = "github"
}

resource "laravel_cloud_environment" "env" {
  application_id = laravel_cloud_application.app.id
  name           = "production"
  branch         = "main"
}
`

const clusterStack = `
resource "laravel_cloud_database_cluster" "cluster" {
  # The platform creates a database inside every cluster, so a test
  # fixture has to opt in to sweeping it on destroy.
  force_destroy = true

  name   = "main-db"
  type   = "laravel_mysql_84"
  region = "us-east-1"
  config = jsonencode({ size = "db-1vcpu-1gb" })
}
`

const bucketStack = `
resource "laravel_cloud_storage_bucket" "bucket" {
  name     = "assets"
  key_name = "primary"
}
`

const serverStack = `
resource "laravel_cloud_websocket_server" "server" {
  name            = "reverb"
  type            = "reverb"
  region          = "us-east-1"
  max_connections = 1000
}
`

const instanceStack = `
resource "laravel_cloud_instance" "instance" {
  environment_id = laravel_cloud_environment.env.id
  name           = "web"
  size           = "flex.c-1vcpu-256mb"
  scaling_type   = "none"
  min_replicas   = 1
  max_replicas   = 1
}
`

func create(addr string) plancheck.PlanCheck {
	return plancheck.ExpectResourceAction(addr, plancheck.ResourceActionCreate)
}

// ---------------------------------------------------------------------------
// Top-level resources
// ---------------------------------------------------------------------------

func TestCachePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_cache.cache"
	config := providerCfg(baseURL) + `
resource "laravel_cloud_cache" "cache" {
  name                 = "redis"
  type                 = "upstash_redis"
  region               = "us-east-1"
  size                 = "250mb"
  auto_upgrade_enabled = true
  is_public            = false
}
`
	runPlanStability(t, config,
		[]plancheck.PlanCheck{
			create(addr),
			plancheck.ExpectKnownValue(addr, tfjsonpath.New("type"), knownvalue.StringExact("upstash_redis")),
			plancheck.ExpectUnknownValue(addr, tfjsonpath.New("id")),
			plancheck.ExpectUnknownValue(addr, tfjsonpath.New("status")),
			plancheck.ExpectUnknownValue(addr, tfjsonpath.New("connection_details")),
		},
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("status"), knownvalue.StringExact("available")),
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("connection_details").AtMapKey("hostname"), knownvalue.StringExact("cache.test.local")),
		},
	)
}

func TestDatabaseClusterPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_database_cluster.cluster"
	config := providerCfg(baseURL) + clusterStack
	runPlanStability(t, config,
		[]plancheck.PlanCheck{
			create(addr),
			plancheck.ExpectKnownValue(addr, tfjsonpath.New("type"), knownvalue.StringExact("laravel_mysql_84")),
			plancheck.ExpectUnknownValue(addr, tfjsonpath.New("id")),
		},
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("status"), knownvalue.StringExact("available")),
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("connection_details").AtMapKey("driver"), knownvalue.StringExact("mysql")),
		},
	)
}

func TestStorageBucketPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_storage_bucket.bucket"
	config := providerCfg(baseURL) + bucketStack
	runPlanStability(t, config,
		[]plancheck.PlanCheck{create(addr)},
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("status"), knownvalue.StringExact("available")),
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("visibility"), knownvalue.StringExact("private")),
		},
	)
}

func TestWebsocketServerPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_websocket_server.server"
	config := providerCfg(baseURL) + serverStack
	runPlanStability(t, config,
		[]plancheck.PlanCheck{create(addr)},
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("status"), knownvalue.StringExact("available")),
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("hostname"), knownvalue.StringExact("ws.test.local")),
		},
	)
}

// ---------------------------------------------------------------------------
// Environment-scoped resources
// ---------------------------------------------------------------------------

func TestInstancePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_instance.instance"
	config := providerCfg(baseURL) + appEnvStack + instanceStack
	runPlanStability(t, config,
		[]plancheck.PlanCheck{
			create(addr),
			plancheck.ExpectKnownValue(addr, tfjsonpath.New("name"), knownvalue.StringExact("web")),
		},
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("min_replicas"), knownvalue.Int64Exact(1)),
		},
	)
}

func TestDomainPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_domain.domain"
	config := providerCfg(baseURL) + appEnvStack + `
resource "laravel_cloud_domain" "domain" {
  environment_id = laravel_cloud_environment.env.id
  name           = "example.com"
}
`
	runPlanStability(t, config,
		[]plancheck.PlanCheck{create(addr)},
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("type"), knownvalue.StringExact("root")),
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("hostname_status"), knownvalue.StringExact("pending")),
		},
	)
}

func TestDeploymentPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_deployment.deploy"
	config := providerCfg(baseURL) + appEnvStack + `
resource "laravel_cloud_deployment" "deploy" {
  environment_id = laravel_cloud_environment.env.id
}
`
	runPlanStability(t, config,
		[]plancheck.PlanCheck{create(addr)},
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("status"), knownvalue.StringExact("pending")),
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("php_major_version"), knownvalue.StringExact("8.4")),
		},
	)
}

func TestCommandPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_command.command"
	config := providerCfg(baseURL) + appEnvStack + `
resource "laravel_cloud_command" "command" {
  environment_id = laravel_cloud_environment.env.id
  command        = "php artisan migrate --force"
}
`
	runPlanStability(t, config,
		[]plancheck.PlanCheck{create(addr)},
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("command"), knownvalue.StringExact("php artisan migrate --force")),
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("status"), knownvalue.StringExact("pending")),
		},
	)
}

func TestEnvironmentVariablesPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_environment_variables.vars"
	config := providerCfg(baseURL) + appEnvStack + `
resource "laravel_cloud_environment_variables" "vars" {
  environment_id = laravel_cloud_environment.env.id
  variables = {
    APP_ENV   = "production"
    LOG_LEVEL = "debug"
  }
}
`
	runPlanStability(t, config,
		[]plancheck.PlanCheck{create(addr)},
		// variables is Sensitive and id mirrors the (unknown) environment id, so
		// the empty-plan step is the meaningful assertion here.
		nil,
	)
}

func TestBackgroundProcessPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_background_process.bgp"
	config := providerCfg(baseURL) + appEnvStack + instanceStack + `
resource "laravel_cloud_background_process" "bgp" {
  instance_id = laravel_cloud_instance.instance.id
  type        = "worker"
  processes   = 2
}
`
	runPlanStability(t, config,
		[]plancheck.PlanCheck{create(addr)},
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("type"), knownvalue.StringExact("worker")),
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("processes"), knownvalue.Int64Exact(2)),
		},
	)
}

// ---------------------------------------------------------------------------
// Cluster-scoped resources
// ---------------------------------------------------------------------------

func TestDatabasePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_database.db"
	config := providerCfg(baseURL) + clusterStack + `
resource "laravel_cloud_database" "db" {
  cluster_id = laravel_cloud_database_cluster.cluster.id
  name       = "app_db"
}
`
	runPlanStability(t, config,
		[]plancheck.PlanCheck{
			create(addr),
			plancheck.ExpectKnownValue(addr, tfjsonpath.New("name"), knownvalue.StringExact("app_db")),
		},
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("status"), knownvalue.StringExact("available")),
		},
	)
}

// expectNoDatabasesLeft asserts the fake's database store is empty — used as a
// CheckDestroy to prove the resource's Delete actually removed the schema (and
// did not silently no-op against a wrong/blocked route).
func expectNoDatabasesLeft(f *fakeCloud) func(*terraform.State) error {
	return func(*terraform.State) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		if n := len(f.objects["database"]); n != 0 {
			return fmt.Errorf("expected the database to be deleted via the nested route, but %d schema(s) remain", n)
		}
		return nil
	}
}

// TestDatabaseDeletedViaNestedRoute guards database deletion: the flat
// DELETE /databases/{id} route is not accepted by the API, so the resource must
// delete through the cluster-nested route. The fake serves DELETE only at the nested
// route (see the database crudSpec), so a flat-route delete 405s and the schema
// would survive destroy — which CheckDestroy then catches.
func TestDatabaseDeleteNestedRoutePlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	config := providerCfg(baseURL) + clusterStack + `
resource "laravel_cloud_database" "db" {
  cluster_id = laravel_cloud_database_cluster.cluster.id
  name       = "app_db"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             expectNoDatabasesLeft(f),
		Steps: []resource.TestStep{
			{Config: config},
		},
	})
}

// TestDatabaseImportState verifies a database imports via the composite
// "cluster_id:database_id" id (databases are only addressable through the
// cluster-nested routes, so the cluster id must travel with the import id) and
// can then be deleted — exercising both the import parsing and delete-after-import.
func TestDatabaseImportPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	config := providerCfg(baseURL) + clusterStack + `
resource "laravel_cloud_database" "db" {
  cluster_id = laravel_cloud_database_cluster.cluster.id
  name       = "app_db"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             expectNoDatabasesLeft(f),
		Steps: []resource.TestStep{
			{Config: config},
			{
				ResourceName:      "laravel_cloud_database.db",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["laravel_cloud_database.db"]
					if !ok {
						return "", fmt.Errorf("laravel_cloud_database.db not found in state")
					}
					return rs.Primary.Attributes["cluster_id"] + ":" + rs.Primary.Attributes["id"], nil
				},
			},
		},
	})
}

func TestDatabaseSnapshotPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_database_snapshot.snap"
	config := providerCfg(baseURL) + clusterStack + `
resource "laravel_cloud_database_snapshot" "snap" {
  cluster_id = laravel_cloud_database_cluster.cluster.id
  name       = "nightly"
}
`
	runPlanStability(t, config,
		[]plancheck.PlanCheck{create(addr)},
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("type"), knownvalue.StringExact("manual")),
			// A snapshot is created pending; it does not become available
			// within the create call, and has no size reported until it does.
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("status"), knownvalue.StringExact("pending")),
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("storage_bytes"), knownvalue.Null()),
		},
	)
}

func TestDatabaseRestorePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_database_restore.restore"
	config := providerCfg(baseURL) + clusterStack + `
resource "laravel_cloud_database_restore" "restore" {
  database_cluster_id = laravel_cloud_database_cluster.cluster.id
  name                = "restored-db"
  restore_time        = "2026-01-01T00:00:00Z"
}
`
	runPlanStability(t, config,
		[]plancheck.PlanCheck{create(addr)},
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("restored_cluster_status"), knownvalue.StringExact("creating")),
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("region"), knownvalue.StringExact("us-east-1")),
		},
	)
}

// ---------------------------------------------------------------------------
// Other nested resources
// ---------------------------------------------------------------------------

func TestStorageBucketKeyPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_storage_bucket_key.key"
	config := providerCfg(baseURL) + bucketStack + `
resource "laravel_cloud_storage_bucket_key" "key" {
  bucket_id  = laravel_cloud_storage_bucket.bucket.id
  name       = "deploy-key"
  permission = "read_write"
}
`
	runPlanStability(t, config,
		[]plancheck.PlanCheck{create(addr)},
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("permission"), knownvalue.StringExact("read_write")),
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("access_key_id"), knownvalue.StringExact("AKIATEST0001")),
		},
	)
}

// Read resolves a key from the bucket's key listing, so it needs bucket_id --
// and bucket_id forces replacement. Importing by bare id would leave it null,
// so the import must carry it as "bucket_id:key_id". This is also the route
// back for any keys that were orphaned outside Terraform.
func TestStorageBucketKeyImportPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := providerCfg(baseURL) + bucketStack + `
resource "laravel_cloud_storage_bucket_key" "key" {
  bucket_id  = laravel_cloud_storage_bucket.bucket.id
  name       = "deploy-key"
  permission = "read_write"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				ResourceName:      "laravel_cloud_storage_bucket_key.key",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["laravel_cloud_storage_bucket_key.key"]
					if !ok {
						return "", fmt.Errorf("laravel_cloud_storage_bucket_key.key not found in state")
					}
					return rs.Primary.Attributes["bucket_id"] + ":" + rs.Primary.Attributes["id"], nil
				},
			},
		},
	})
}

func TestWebsocketApplicationPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_websocket_application.wsapp"
	config := providerCfg(baseURL) + serverStack + `
resource "laravel_cloud_websocket_application" "wsapp" {
  server_id        = laravel_cloud_websocket_server.server.id
  name             = "chat"
  allowed_origins  = ["https://example.com"]
  ping_interval    = 60
  activity_timeout = 120
}
`
	runPlanStability(t, config,
		[]plancheck.PlanCheck{create(addr)},
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("app_id"), knownvalue.StringExact("wsapp-0001")),
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("max_connections"), knownvalue.Int64Exact(1000)),
		},
	)
}

// GET /websocket-applications/{id} omits the owning server, and server_id
// forces replacement -- so importing by bare id leaves it null and plans a
// destroy/recreate of a healthy application. Import takes "server_id:app_id".
func TestWebsocketApplicationImportPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := providerCfg(baseURL) + serverStack + `
resource "laravel_cloud_websocket_application" "wsapp" {
  server_id        = laravel_cloud_websocket_server.server.id
  name             = "chat"
  allowed_origins  = ["https://example.com"]
  ping_interval    = 60
  activity_timeout = 120
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				ResourceName:      "laravel_cloud_websocket_application.wsapp",
				ImportState:       true,
				ImportStateVerify: true,
				// key and secret are returned only when the application is
				// created, so an imported application cannot recover them.
				// They stay null rather than being filled with empty strings.
				ImportStateVerifyIgnore: []string{"key", "secret"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["laravel_cloud_websocket_application.wsapp"]
					if !ok {
						return "", fmt.Errorf("laravel_cloud_websocket_application.wsapp not found in state")
					}
					return rs.Primary.Attributes["server_id"] + ":" + rs.Primary.Attributes["id"], nil
				},
			},
		},
	})
}
