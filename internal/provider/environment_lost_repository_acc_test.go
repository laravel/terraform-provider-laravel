package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// TestAccEnvironmentResource_lostRepository creates an application and an
// environment on it while the only other application on the same repository is
// deleted. Laravel Cloud then deletes its record of the repository,
// leaves the new application with no repository, and refuses every environment
// on it with a 422 "The selected branch is no longer available." until the
// repository is set again.
//
// terraform_data deletes the other application in parallel with the
// application's create, which is how the race happened between test runs.
func TestAccEnvironmentResource_lostRepository(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC must be set for acceptance tests")
	}
	testAccPreCheck(t)
	// A repository no other application in the test organization uses, so
	// deleting the test's own application on it deletes Laravel Cloud's
	// record of the repository. Its default branch must be main.
	repository := testAccFixture(t, "LARAVEL_CLOUD_TEST_UNUSED_REPOSITORY")
	ctx := context.Background()
	c := testAccClient()
	baseURL := os.Getenv("LARAVEL_CLOUD_BASE_URL")
	if baseURL == "" {
		baseURL = "https://cloud.laravel.com/api"
	}

	other, err := c.CreateApplication(ctx, client.CreateApplicationRequest{
		Name:       testAccName(),
		Repository: repository,
		Region:     "us-east-2",
	})
	if err != nil {
		t.Fatalf("creating the other application: %v", err)
	}
	t.Cleanup(func() {
		// terraform_data normally deleted it already.
		if _, err := c.GetApplication(ctx, other.ID); client.IsNotFound(err) {
			return
		}
		if err := c.DeleteApplication(ctx, other.ID); err != nil && !client.IsNotFound(err) {
			t.Errorf("deleting the other application %s: %v", other.ID, err)
		}
	})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terraform_data" "delete_other" {
  provisioner "local-exec" {
    command = "sleep 2 && test \"$(curl -sS -o /dev/null -w '%%%%{http_code}' -X DELETE -H \"Authorization: Bearer $LARAVEL_CLOUD_API_TOKEN\" -H 'Accept: application/json' %[1]s/applications/%[2]s)\" = 204"
  }
}

resource "laravel_cloud_application" "test" {
  name       = %[3]q
  repository = %[4]q
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "staging"
  branch         = "main"
}
`, baseURL, other.ID, testAccName(), repository),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("laravel_cloud_environment.test", "id"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources["laravel_cloud_application.test"].Primary.ID
						app, err := c.GetApplication(ctx, id)
						if err != nil {
							return err
						}
						if app.Attributes.Repository == nil {
							return fmt.Errorf("application %s has no repository", id)
						}
						return nil
					},
				),
			},
		},
	})
}
