package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestInstanceCreateFollowUpPlan covers the instance attributes the create
// route ignores: uses_octane, uses_inertia_ssr and hibernation_timeout.
//
// They used to be written straight into state after a create that never sent
// them, so Terraform reported settings the platform had never been told about.
// The API does not report them back either, so no later refresh could catch it
// -- an instance configured with uses_octane = true simply was not running
// Octane, and every plan was clean. The provider now applies them with a
// follow-up update, which is the only route that accepts them.
func TestInstanceCreateFollowUpPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: instanceCreateFollowUpConfig(baseURL),
				Check: func(*terraform.State) error {
					f.mu.Lock()
					defer f.mu.Unlock()

					for id, attrs := range f.objects["instance"] {
						if attrs["uses_octane"] != true {
							return fmt.Errorf("instance %s has uses_octane %v, want true", id, attrs["uses_octane"])
						}
						if attrs["uses_inertia_ssr"] != true {
							return fmt.Errorf("instance %s has uses_inertia_ssr %v, want true", id, attrs["uses_inertia_ssr"])
						}
						if got, want := attrs["hibernation_timeout"], float64(30); got != want {
							return fmt.Errorf("instance %s has hibernation_timeout %v, want %v", id, got, want)
						}
					}
					return nil
				},
			},
			// The follow-up update must not leave a diff behind.
			{
				Config:   instanceCreateFollowUpConfig(baseURL),
				PlanOnly: true,
			},
		},
	})
}

func instanceCreateFollowUpConfig(baseURL string) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_application" "app" {
  name       = "followup-app"
  repository = "laravel/laravel"
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "env" {
  application_id = laravel_cloud_application.app.id
  name           = "production"
  branch         = "main"
}

resource "laravel_cloud_instance" "web" {
  environment_id = laravel_cloud_environment.env.id
  name           = "web"
  size           = "flex.c-1vcpu-256mb"
  scaling_type   = "auto"

  uses_octane         = true
  uses_inertia_ssr    = true
  hibernation_timeout = 30
}
`, baseURL)
}

// TestInstanceHibernationTimeoutRangePlan keeps the API's documented bound at
// plan time. The API answers an out-of-range value with "The hibernation
// timeout must be null or an integer between 1 and 60." -- and because the
// provider now applies this attribute with a post-create update, a 422 there
// would land as a warning on an instance that already exists.
func TestInstanceHibernationTimeoutRangePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      instanceHibernationConfig(baseURL, 300),
				ExpectError: regexp.MustCompile(`hibernation_timeout is out of range`),
			},
		},
	})
}

func instanceHibernationConfig(baseURL string, timeout int) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_application" "app" {
  name       = "hib-app"
  repository = "laravel/laravel"
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "env" {
  application_id = laravel_cloud_application.app.id
  name           = "production"
  branch         = "main"
}

resource "laravel_cloud_instance" "web" {
  environment_id = laravel_cloud_environment.env.id
  name           = "web"
  size           = "flex-512mb"
  scaling_type   = "auto"

  hibernation_timeout = %[2]d
}
`, baseURL, timeout)
}
