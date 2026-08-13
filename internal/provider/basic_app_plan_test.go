package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestBasicAppPlan drives the documented examples/basic-app configuration
// (application + production environment + outputs) against an in-memory fake
// Laravel Cloud API. It does NOT touch real cloud resources and needs no API
// token — only TF_ACC=1 and a terraform binary on PATH.
//
// It verifies three things the example promises:
//  1. The create plan: known config values are planned as-is and server-managed
//     values are planned as unknown.
//  2. The applied result: php_version "8.4:1" is reported back as
//     php_major_version "8.4", and the outputs resolve to the expected values.
//  3. Stability: re-planning the same config produces an empty plan (no
//     perpetual diff), which is the classic failure mode for the
//     php_version/php_major_version split.
func TestBasicAppPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := basicAppConfig(baseURL)

	const appAddr = "laravel_cloud_application.example"
	const envAddr = "laravel_cloud_environment.production"

	resource.Test(t, resource.TestCase{
		// No PreCheck: the fake API needs no token. ProtoV6ProviderFactories is
		// reused from provider_test.go.
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// 1 + 2: assert the create plan, then apply and assert the state.
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(appAddr, plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction(envAddr, plancheck.ResourceActionCreate),

						// Known config values are planned exactly as written.
						plancheck.ExpectKnownValue(appAddr, tfjsonpath.New("name"), knownvalue.StringExact("my-first-app")),
						plancheck.ExpectKnownValue(appAddr, tfjsonpath.New("repository"), knownvalue.StringExact("laravel/laravel")),
						plancheck.ExpectKnownValue(appAddr, tfjsonpath.New("region"), knownvalue.StringExact("us-east-2")),
						plancheck.ExpectKnownValue(appAddr, tfjsonpath.New("source_control_provider_type"), knownvalue.StringExact("github")),
						plancheck.ExpectKnownValue(envAddr, tfjsonpath.New("name"), knownvalue.StringExact("production")),
						plancheck.ExpectKnownValue(envAddr, tfjsonpath.New("branch"), knownvalue.StringExact("main")),
						plancheck.ExpectKnownValue(envAddr, tfjsonpath.New("php_version"), knownvalue.StringExact("8.4:1")),
						plancheck.ExpectKnownValue(envAddr, tfjsonpath.New("node_version"), knownvalue.StringExact("22")),
						plancheck.ExpectKnownValue(envAddr, tfjsonpath.New("uses_push_to_deploy"), knownvalue.Bool(true)),

						// Server-managed values are unknown until apply.
						plancheck.ExpectUnknownValue(appAddr, tfjsonpath.New("id")),
						plancheck.ExpectUnknownValue(appAddr, tfjsonpath.New("slug")),
						plancheck.ExpectUnknownValue(envAddr, tfjsonpath.New("id")),
						plancheck.ExpectUnknownValue(envAddr, tfjsonpath.New("status")),
						plancheck.ExpectUnknownValue(envAddr, tfjsonpath.New("php_major_version")),
						// application_id references the app's unknown id.
						plancheck.ExpectUnknownValue(envAddr, tfjsonpath.New("application_id")),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					// The configured values survive the apply round-trip.
					statecheck.ExpectKnownValue(appAddr, tfjsonpath.New("region"), knownvalue.StringExact("us-east-2")),
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("php_version"), knownvalue.StringExact("8.4:1")),
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("node_version"), knownvalue.StringExact("22")),
					// The "8.4:1" -> "8.4" transform.
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("php_major_version"), knownvalue.StringExact("8.4")),
					statecheck.ExpectKnownValue(envAddr, tfjsonpath.New("status"), knownvalue.StringExact("active")),

					// Outputs resolve to the expected values.
					statecheck.ExpectKnownOutputValue("environment_status", knownvalue.StringExact("active")),
					statecheck.ExpectKnownOutputValue("environment_php_major_version", knownvalue.StringExact("8.4")),
				},
			},
			// 3: re-applying the same config must be a no-op (no perpetual diff).
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

// basicAppConfig mirrors examples/basic-app/main.tf, with a provider block
// pointing at the fake API. Keep this in sync with that example so the test
// validates the documented configuration.
func basicAppConfig(baseURL string) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_application" "example" {
  name       = "my-first-app"
  repository = "laravel/laravel"
  region     = "us-east-2"

  source_control_provider_type = "github"
}

resource "laravel_cloud_environment" "production" {
  application_id = laravel_cloud_application.example.id
  name           = "production"
  branch         = "main"

  php_version  = "8.4:1"
  node_version = "22"

  uses_push_to_deploy = true
}

output "application_id" {
  value = laravel_cloud_application.example.id
}

output "environment_status" {
  value = laravel_cloud_environment.production.status
}

output "environment_php_major_version" {
  value = laravel_cloud_environment.production.php_major_version
}
`, baseURL)
}
