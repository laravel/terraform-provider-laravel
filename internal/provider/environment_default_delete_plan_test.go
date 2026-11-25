package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// TestEnvironmentDefaultDeletePlan pins destroying an environment that took
// over its application's default environment. The API refuses to
// delete the default environment with a 422, so destroy failed and left it in
// state. It must now be dropped from state and left for the application's
// deletion, while any other environment is still deleted.
func TestEnvironmentDefaultDeletePlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)

	app := `
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_application" "app" {
  name       = "default-env-app"
  repository = "example/app"
  region     = "us-east-2"
}
`
	envs := `
resource "laravel_cloud_environment" "production" {
  application_id = laravel_cloud_application.app.id
  name           = "production"
  branch         = "main"
}

resource "laravel_cloud_environment" "staging" {
  application_id = laravel_cloud_application.app.id
  name           = "staging"
  branch         = "main"
}
`
	var defaultID, stagingID string

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(app+envs, baseURL),
				Check: func(s *terraform.State) error {
					appID := s.RootModule().Resources["laravel_cloud_application.app"].Primary.ID
					defaultID = s.RootModule().Resources["laravel_cloud_environment.production"].Primary.ID
					stagingID = s.RootModule().Resources["laravel_cloud_environment.staging"].Primary.ID
					f.mu.Lock()
					defer f.mu.Unlock()
					if got := f.apps[appID].Relationships.DefaultEnvironment.RelatedID(); got != defaultID {
						return fmt.Errorf("production environment %s was not the default %s: it was created rather than taken over", defaultID, got)
					}
					return nil
				},
			},
			// Destroying only the environments: the default one stays until
			// the application goes, the other one is deleted.
			{
				Config: fmt.Sprintf(app, baseURL),
				Check: func(*terraform.State) error {
					f.mu.Lock()
					defer f.mu.Unlock()
					if _, ok := f.envs[defaultID]; !ok {
						return fmt.Errorf("default environment %s was deleted", defaultID)
					}
					if _, ok := f.envs[stagingID]; ok {
						return fmt.Errorf("non-default environment %s was not deleted", stagingID)
					}
					return nil
				},
			},
		},
		CheckDestroy: func(*terraform.State) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			if _, ok := f.envs[defaultID]; ok {
				return fmt.Errorf("default environment %s outlived its application", defaultID)
			}
			return nil
		},
	})
}

// TestEnvironmentDefaultDeleteKeepsAttachmentsPlan: removing only the default
// environment from the configuration leaves it serving with its cache still
// attached. The delete used to detach the cache before finding that the
// environment could not be deleted, leaving a live environment without it.
func TestEnvironmentDefaultDeleteKeepsAttachmentsPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)

	base := providerCfg(baseURL) + `
resource "laravel_cloud_application" "app" {
  name       = "default-env-app"
  repository = "example/app"
  region     = "us-east-2"
}

resource "laravel_cloud_cache" "cache" {
  name                 = "sessions"
  type                 = "laravel_valkey"
  region               = "us-east-2"
  size                 = "250mb"
  auto_upgrade_enabled = false
  is_public            = false
}
`
	env := `
resource "laravel_cloud_environment" "production" {
  application_id = laravel_cloud_application.app.id
  name           = "production"
  branch         = "main"
  cache_id       = laravel_cloud_cache.cache.id
}
`
	var defaultID, cacheID string
	attachedCache := func() string {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.attachments[defaultID]["cache"]
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: base + env,
				Check: func(s *terraform.State) error {
					defaultID = s.RootModule().Resources["laravel_cloud_environment.production"].Primary.ID
					cacheID = s.RootModule().Resources["laravel_cloud_cache.cache"].Primary.ID
					if got := attachedCache(); got != cacheID {
						return fmt.Errorf("cache attached to the default environment is %q, want %q", got, cacheID)
					}
					return nil
				},
			},
			{
				Config: base,
				Check: func(*terraform.State) error {
					if got := attachedCache(); got != cacheID {
						return fmt.Errorf("removing the default environment from the configuration detached its cache: attached is %q, want %q", got, cacheID)
					}
					return nil
				},
			},
		},
	})
}

// TestEnvironmentDefaultServingModeChangePlan: changing serving_mode replaces
// an environment, which Laravel Cloud cannot do for an application's default
// environment. The plan must fail before anything changes, instead of the
// apply removing the environment from state and then failing. Any other
// environment is still replaced.
func TestEnvironmentDefaultServingModeChangePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	config := func(production, staging string) string {
		return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_application" "app" {
  name       = "default-env-app"
  repository = "example/app"
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "production" {
  application_id = laravel_cloud_application.app.id
  name           = "production"
  branch         = "main"
  serving_mode   = %q
}

resource "laravel_cloud_environment" "staging" {
  application_id = laravel_cloud_application.app.id
  name           = "staging"
  branch         = "main"
  serving_mode   = %q
}
`, production, staging)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config("process", "process")},
			{
				Config:      config("static", "process"),
				ExpectError: regexp.MustCompile(`serving_mode cannot be changed on the default environment`),
			},
			{
				Config: config("process", "static"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("laravel_cloud_environment.staging", plancheck.ResourceActionDestroyBeforeCreate),
					plancheck.ExpectResourceAction("laravel_cloud_environment.production", plancheck.ResourceActionNoop),
				}},
			},
		},
	})
}

// TestEnvironmentDefaultLookupFailurePlan: when the provider cannot tell
// whether an environment is its application's default, as when reading the
// application is refused, destroying it must stop before changing anything.
// It used to take a failed lookup as "not the default", detach the live
// default environment's cache, and then fail to delete it.
func TestEnvironmentDefaultLookupFailurePlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	// The application is made outside Terraform, so refusing to read it only
	// affects the environment's delete, not a refresh of the application.
	app, err := client.NewClient("test-token", client.WithBaseURL(baseURL)).CreateApplication(context.Background(),
		client.CreateApplicationRequest{Name: "lookup-app", Repository: "example/app", Region: "us-east-2"})
	if err != nil {
		t.Fatalf("creating the application: %v", err)
	}

	cache := providerCfg(baseURL) + `
resource "laravel_cloud_cache" "cache" {
  name                 = "sessions"
  type                 = "laravel_valkey"
  region               = "us-east-2"
  size                 = "250mb"
  auto_upgrade_enabled = false
  is_public            = false
}
`
	env := fmt.Sprintf(`
resource "laravel_cloud_environment" "production" {
  application_id = %q
  name           = "production"
  branch         = "main"
  cache_id       = laravel_cloud_cache.cache.id
}
`, app.ID)
	var envID, cacheID string
	attachedCache := func() string {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.attachments[envID]["cache"]
	}
	refuseReads := func(refuse bool) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.refuseApplicationReads = refuse
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cache + env,
				Check: func(s *terraform.State) error {
					envID = s.RootModule().Resources["laravel_cloud_environment.production"].Primary.ID
					cacheID = s.RootModule().Resources["laravel_cloud_cache.cache"].Primary.ID
					return nil
				},
			},
			{
				PreConfig:   func() { refuseReads(true) },
				Config:      cache,
				ExpectError: regexp.MustCompile(`Error checking whether the environment is its application's default`),
			},
			{
				PreConfig: func() {
					if got := attachedCache(); got != cacheID {
						t.Errorf("the failed destroy detached the default environment's cache: attached is %q, want %q", got, cacheID)
					}
					refuseReads(false)
				},
				Config: cache,
				Check: func(*terraform.State) error {
					if got := attachedCache(); got != cacheID {
						return fmt.Errorf("dropping the default environment detached its cache: attached is %q, want %q", got, cacheID)
					}
					return nil
				},
			},
		},
	})
}

// TestEnvironmentServingModeWithApplicationReplacementPlan: changing
// serving_mode on both an application and its default environment replaces
// the application, and Laravel Cloud creates its new default environment in
// the new mode, so the environment's replacement must not be refused.
func TestEnvironmentServingModeWithApplicationReplacementPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := func(mode string) string {
		return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_application" "app" {
  name         = "serving-app"
  repository   = "example/app"
  region       = "us-east-2"
  serving_mode = %[1]q
}

resource "laravel_cloud_environment" "production" {
  application_id = laravel_cloud_application.app.id
  name           = "production"
  branch         = "main"
  serving_mode   = %[1]q
}
`, mode)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config("process")},
			{
				Config: config("static"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("laravel_cloud_application.app", plancheck.ResourceActionDestroyBeforeCreate),
					plancheck.ExpectResourceAction("laravel_cloud_environment.production", plancheck.ResourceActionDestroyBeforeCreate),
				}},
				Check: resource.TestCheckResourceAttr("laravel_cloud_environment.production", "serving_mode", "static"),
			},
		},
	})
}
