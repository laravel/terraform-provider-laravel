package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
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

// TestEnvironmentVariablesDriftPlan covers reading variables back.
//
// Read used to be a no-op on the belief that the API exposes no way to read
// them: GET /environments/{id}/variables does answer 405, but the environment
// payload carries them, keys and values both. Without that, a variable changed
// or deleted outside Terraform was invisible forever and every plan came back
// clean while the environment said otherwise.
func TestEnvironmentVariablesDriftPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	config := environmentVariablesConfig(baseURL, `
    API_KEY = "secret"
`)

	mutate := func(fn func(vars map[string]string)) {
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, vars := range f.vars {
			fn(vars)
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				// Someone edits the value in the dashboard. The refresh must
				// notice and plan to put the configured value back.
				PreConfig: func() { mutate(func(v map[string]string) { v["API_KEY"] = "changed-elsewhere" }) },
				Config:    config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("laravel_cloud_environment_variables.vars", plancheck.ResourceActionUpdate),
					},
				},
			},
			{
				// Someone deletes it entirely. Same story.
				PreConfig: func() { mutate(func(v map[string]string) { delete(v, "API_KEY") }) },
				Config:    config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("laravel_cloud_environment_variables.vars", plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

// TestEnvironmentVariablesIgnoresUnmanagedPlan is the safety property that
// makes reading variables back survivable.
//
// The environment also holds variables this resource never set. Adopting them
// on refresh would put them in state, the next plan would see them missing from
// the configuration, and Update -- which now deletes removed keys by name --
// would delete them. Reading has to stay strictly narrower than owning, or
// adding drift detection would quietly turn into deleting other people's
// variables on the next apply.
func TestEnvironmentVariablesIgnoresUnmanagedPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	config := environmentVariablesConfig(baseURL, `
    API_KEY = "secret"
`)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				// A variable arrives from outside Terraform.
				PreConfig: func() {
					f.mu.Lock()
					defer f.mu.Unlock()
					for _, vars := range f.vars {
						vars["SET_IN_DASHBOARD"] = "keep me"
					}
				},
				Config: config,
				// It is not ours, so it must not produce a diff...
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				// ...and must still be on the environment afterwards.
				Check: func(*terraform.State) error {
					f.mu.Lock()
					defer f.mu.Unlock()
					for envID, vars := range f.vars {
						if _, ok := vars["SET_IN_DASHBOARD"]; !ok {
							return fmt.Errorf("environment %s lost the unmanaged variable: %v", envID, vars)
						}
					}
					return nil
				},
			},
		},
	})
}
