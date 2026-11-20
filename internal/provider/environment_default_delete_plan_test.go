package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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
