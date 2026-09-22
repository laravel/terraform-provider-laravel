package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestEnvironmentVariablesRemovalPlan proves that dropping a key from the
// variables map takes it off the environment, not just out of state.
//
// The resource only ever sent a "set" call carrying the keys still in config.
// Because "set" writes the keys it is given rather than replacing the whole
// set -- which is why the API keeps a separate delete route at all -- a removed
// variable stayed live in the environment while Terraform reported it gone.
// For a resource whose whole purpose is holding credentials, that is the wrong
// direction to be wrong in: taking a leaked secret out of config looked like it
// worked and changed nothing.
//
// The fake models the upsert semantics, so this test fails if the provider goes
// back to relying on "set" alone.
func TestEnvironmentVariablesRemovalPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: environmentVariablesConfig(baseURL, `
    API_KEY  = "secret"
    LOG_LEVEL = "debug"
`),
				Check: func(*terraform.State) error { return nil },
			},
			{
				// LOG_LEVEL is gone from config; API_KEY changes value.
				Config: environmentVariablesConfig(baseURL, `
    API_KEY = "rotated"
`),
				Check: func(*terraform.State) error {
					f.mu.Lock()
					defer f.mu.Unlock()

					for envID, vars := range f.vars {
						if _, stale := vars["LOG_LEVEL"]; stale {
							return fmt.Errorf("environment %s still has LOG_LEVEL after it was removed from config: %v", envID, vars)
						}
						if got := vars["API_KEY"]; got != "rotated" {
							return fmt.Errorf("environment %s has API_KEY %q, want %q", envID, got, "rotated")
						}
					}
					return nil
				},
			},
		},
	})
}

func environmentVariablesConfig(baseURL, vars string) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_application" "app" {
  name       = "vars-app"
  repository = "laravel/laravel"
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "env" {
  application_id = laravel_cloud_application.app.id
  name           = "production"
  branch         = "main"
}

resource "laravel_cloud_environment_variables" "vars" {
  environment_id = laravel_cloud_environment.env.id
  variables = {%[2]s}
}
`, baseURL, vars)
}
