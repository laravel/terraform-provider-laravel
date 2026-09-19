package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// An Optional+Computed attribute that the configuration leaves unset is marked
// unknown on update. Without UseStateForUnknown the Update path compared the
// unknown plan value against state, found them unequal, and sent the zero
// value -- clearing a slug the API had generated, or resetting platform-chosen
// instance timeouts. The response was then mapped into state, so the damage
// was invisible on the next plan.
//
// Commit f607640 fixed this on Create and reported that a sweep found no
// others; the Update path was missed.

func TestApplicationUpdateKeepsGeneratedSlugPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	before := providerCfg(baseURL) + `
resource "laravel_cloud_application" "app" {
  name                         = "slug-app"
  repository                   = "laravel/laravel"
  region                       = "us-east-2"
  source_control_provider_type = "github"
}
`
	after := providerCfg(baseURL) + `
resource "laravel_cloud_application" "app" {
  name                         = "slug-app-renamed"
  repository                   = "laravel/laravel"
  region                       = "us-east-2"
  source_control_provider_type = "github"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: before,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("laravel_cloud_application.app",
						tfjsonpath.New("slug"), knownvalue.StringExact("slug-app")),
				},
			},
			{
				Config: after,
				// The rename must not blank the slug the API generated.
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("laravel_cloud_application.app",
						tfjsonpath.New("slug"), knownvalue.StringExact("slug-app")),
				},
			},
		},
	})
}

func TestInstanceUpdateKeepsPlatformTimeoutsPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	instance := func(name string) string {
		return providerCfg(baseURL) + appEnvStack + `
resource "laravel_cloud_instance" "web" {
  environment_id = laravel_cloud_environment.env.id
  name           = "` + name + `"
  size           = "flex.c-1vcpu-256mb"
  scaling_type   = "custom"
  min_replicas   = 1
  max_replicas   = 1
}
`
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: instance("web")},
			{
				Config: instance("web-renamed"),
				// Renaming must not push zero values for the three
				// Optional+Computed attributes the config never set.
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("laravel_cloud_instance.web",
						tfjsonpath.New("shutdown_timeout"), knownvalue.Null()),
					statecheck.ExpectKnownValue("laravel_cloud_instance.web",
						tfjsonpath.New("visibility_timeout"), knownvalue.Null()),
				},
			},
		},
	})
}
