package provider

import (
	"fmt"
	"regexp"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestEnvironmentWriteOnlyAttributesPlan guards the environment attributes the
// API accepts on PATCH but never returns: color, timeout, sleep_timeout,
// shutdown_timeout and uses_purge_edge_cache_on_deploy.
//
// These are the classic non-converging-diff trap. When the provider decoded
// them as response fields they came back as the Go zero value on every refresh,
// overwrote the configured value in state, and every subsequent plan proposed
// writing them again -- forever, with no apply ever reaching a steady state.
//
// The test sets all five, applies, and then asserts an empty follow-up plan.
// It fails loudly if anything starts reading them back from the response again.
//
// cache_strategy is covered too, for the opposite reason: it IS readable, but
// only from network_settings.cache.strategy rather than a top-level attribute.
func TestEnvironmentWriteOnlyAttributesPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := environmentWriteOnlyConfig(baseURL)

	const envAddr = "laravel_cloud_environment.writeonly"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					// Every write-only value must survive apply as configured,
					// rather than collapsing to the zero value the API implies.
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("color"), knownvalue.StringExact("purple")),
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("timeout"), knownvalue.Int64Exact(45)),
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("sleep_timeout"), knownvalue.Int64Exact(12)),
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("shutdown_timeout"), knownvalue.Int64Exact(20)),
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("uses_purge_edge_cache_on_deploy"), knownvalue.Bool(true)),
					// cache_strategy round-trips through network_settings.
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("cache_strategy"), knownvalue.StringExact("bypass")),
				},
			},
			// The load-bearing step: re-planning the identical config must be a
			// no-op. Before the fix this produced a permanent diff on all five.
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestEnvironmentWriteOnlyOmittedPlan covers the same attributes when they are
// NOT set in config. They are Optional+Computed, so Terraform requires them to
// be known after apply -- and because the API returns no value to adopt, they
// must resolve to null. A fabricated zero value here would be re-sent on every
// apply and would also read as a change away from the user's intent.
func TestEnvironmentWriteOnlyOmittedPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := environmentMinimalConfig(baseURL)

	const envAddr = "laravel_cloud_environment.minimal"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("color"), knownvalue.Null()),
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("timeout"), knownvalue.Null()),
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("sleep_timeout"), knownvalue.Null()),
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("shutdown_timeout"), knownvalue.Null()),
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("uses_purge_edge_cache_on_deploy"), knownvalue.Null()),
				},
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func environmentWriteOnlyConfig(baseURL string) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_application" "example" {
  name       = "writeonly-app"
  repository = "laravel/laravel"
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "writeonly" {
  application_id = laravel_cloud_application.example.id
  name           = "production"
  branch         = "main"

  color                           = "purple"
  timeout                         = 45
  sleep_timeout                   = 12
  shutdown_timeout                = 20
  uses_purge_edge_cache_on_deploy = true
  cache_strategy                  = "bypass"
}
`, baseURL)
}

func environmentMinimalConfig(baseURL string) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_application" "example" {
  name       = "minimal-app"
  repository = "laravel/laravel"
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "minimal" {
  application_id = laravel_cloud_application.example.id
  name           = "production"
  branch         = "main"
}
`, baseURL)
}

// TestEnvironmentVanityDomainPlan covers the vanity domain, which the API
// exposes as its own PUT /environments/{id}/vanity-domain route rather than a
// field on the environment update body. It was previously Computed-only, so
// the hostname could not be set from Terraform at all.
func TestEnvironmentVanityDomainPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := environmentVanityConfig(baseURL, "my-app-staging")

	const envAddr = "laravel_cloud_environment.vanity"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("vanity_domain"),
						knownvalue.StringExact("my-app-staging")),
				},
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Renaming it must go through the dedicated endpoint in place,
			// not force a replacement.
			{
				Config: environmentVanityConfig(baseURL, "my-app-prod"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(envAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("vanity_domain"),
						knownvalue.StringExact("my-app-prod")),
				},
			},
		},
	})
}

func environmentVanityConfig(baseURL, vanity string) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_application" "example" {
  name       = "vanity-app"
  repository = "laravel/laravel"
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "vanity" {
  application_id = laravel_cloud_application.example.id
  name           = "production"
  branch         = "main"
  vanity_domain  = %[2]q
}
`, baseURL, vanity)
}

// TestEnvironmentDatabaseIDRenamePlan covers database_id and its deprecated
// alias database_schema_id. Both send database_schema_id to the API, so moving
// a configuration from the old name to the new one must not detach and
// re-attach the database, and setting both is rejected.
func TestEnvironmentDatabaseIDRenamePlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)

	const envAddr = "laravel_cloud_environment.db"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: environmentDatabaseConfig(baseURL, `database_schema_id = "db-1"`),
				Check:  expectDatabaseUpdates(f, `"db-1"`),
			},
			// The rename shows in the plan, but applying it only rewrites
			// state: the database is already attached.
			{
				Config: environmentDatabaseConfig(baseURL, `database_id = "db-1"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(envAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("database_id"), knownvalue.StringExact("db-1")),
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("database_schema_id"), knownvalue.Null()),
				},
				Check: expectDatabaseUpdates(f, `"db-1"`),
			},
			{
				Config: environmentDatabaseConfig(baseURL, `database_id = "db-1"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: environmentDatabaseConfig(baseURL, `
  database_id        = "db-1"
  database_schema_id = "db-1"`),
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			// Dropping the attribute detaches the database.
			{
				Config: environmentDatabaseConfig(baseURL, ""),
				Check:  expectDatabaseUpdates(f, `"db-1"`, "null"),
			},
		},
	})
}

// TestEnvironmentDatabaseIDCreateDestroyPlan covers database_id on the Create
// path and the detach sent before an environment with a database is deleted.
func TestEnvironmentDatabaseIDCreateDestroyPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             expectDatabaseUpdates(f, `"db-1"`, "null"),
		Steps: []resource.TestStep{
			{
				Config: environmentDatabaseConfig(baseURL, `database_id = "db-1"`),
				Check:  expectDatabaseUpdates(f, `"db-1"`),
			},
		},
	})
}

// TestEnvironmentDatabaseIDIgnoreChangesPlan covers ignore_changes on the
// deprecated alias surviving a rename. Terraform then keeps the old value in
// the plan next to database_id, which validation cannot see; the plan must be
// rejected rather than leave a stale value that removing database_id would
// later fall back to and re-attach.
func TestEnvironmentDatabaseIDIgnoreChangesPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)

	const ignoreAlias = `
  lifecycle {
    ignore_changes = [database_schema_id]
  }`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: environmentDatabaseConfig(baseURL, `database_schema_id = "db-1"`+ignoreAlias),
				Check:  expectDatabaseUpdates(f, `"db-1"`),
			},
			{
				Config:      environmentDatabaseConfig(baseURL, `database_id = "db-1"`+ignoreAlias),
				ExpectError: regexp.MustCompile(`Conflicting database attributes`),
			},
			// Dropping ignore_changes lets the rename through, still without
			// touching the attachment.
			{
				Config: environmentDatabaseConfig(baseURL, `database_id = "db-1"`),
				Check:  expectDatabaseUpdates(f, `"db-1"`),
			},
		},
	})
}

func environmentDatabaseConfig(baseURL, attrs string) string {
	return providerCfg(baseURL) + `
resource "laravel_cloud_application" "example" {
  name       = "db-app"
  repository = "laravel/laravel"
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "db" {
  application_id = laravel_cloud_application.example.id
  name           = "production"
  branch         = "main"
  ` + attrs + `
}
`
}

// expectDatabaseUpdates checks the database_schema_id of every environment
// PATCH that carried one, in order, since the fake cloud was created.
func expectDatabaseUpdates(f *fakeCloud, want ...string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		var got []string
		for _, u := range f.envUpdates {
			if u.DatabaseSchemaID != nil {
				got = append(got, string(u.DatabaseSchemaID))
			}
		}
		if !slices.Equal(got, want) {
			return fmt.Errorf("database_schema_id updates = %q, want %q", got, want)
		}
		return nil
	}
}
