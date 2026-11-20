package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestEnvironmentLostRepositoryPlan: Laravel Cloud leaves an
// application created while the last other application on its repository is
// deleted with no repository, and then refuses every environment on it with a
// 422 "The selected branch is no longer available." Waiting never helps; the
// repository must be set again. Environment create does that in the same apply,
// and an application found without one plans the repository again. A branch
// Cloud has not synced yet gets the same 422 and is waited out.
func TestEnvironmentLostRepositoryPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)

	config := fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_application" "app" {
  name       = "lost-repository-app"
  repository = "example/playground"
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "staging" {
  application_id = laravel_cloud_application.app.id
  name           = "staging"
  branch         = "main"
}
`, baseURL)

	var appID string
	repositoryRestored := func(s *terraform.State) error {
		appID = s.RootModule().Resources["laravel_cloud_application.app"].Primary.ID
		f.mu.Lock()
		defer f.mu.Unlock()
		repo := f.apps[appID].Attributes.Repository
		if repo == nil || repo.FullName != "example/playground" {
			return fmt.Errorf("application %s repository = %v, want example/playground", appID, repo)
		}
		return nil
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Lost while the application is created: the environment create
			// re-attaches the repository and succeeds in the same apply.
			{
				PreConfig: func() {
					f.mu.Lock()
					f.loseRepositories = 1
					f.mu.Unlock()
				},
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					repositoryRestored,
					resource.TestCheckResourceAttrSet("laravel_cloud_environment.staging", "id"),
				),
			},
			// Lost later, with nothing left to create: the next apply sets it.
			{
				PreConfig: func() {
					f.mu.Lock()
					f.apps[appID].Attributes.Repository = nil
					f.mu.Unlock()
				},
				Config: config,
				Check:  repositoryRestored,
			},
			// A branch Laravel Cloud has not synced yet, on an application that
			// has its repository: the create is retried until it is.
			{
				PreConfig: func() {
					f.mu.Lock()
					f.unsyncedBranches = 2
					f.mu.Unlock()
				},
				Config: config + `
resource "laravel_cloud_environment" "preview" {
  application_id = laravel_cloud_application.app.id
  name           = "preview"
  branch         = "feature/test"
}
`,
				Check: resource.TestCheckResourceAttrSet("laravel_cloud_environment.preview", "id"),
			},
		},
	})
}
