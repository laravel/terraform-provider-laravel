package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

func TestAccEnvironmentResource_basic(t *testing.T) {
	appName := testAccName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read. laravel/laravel's default branch, 13.x, is not
			// production-like, so the application's default environment is
			// named "13 x" and this "production" environment is a new one.
			// TestAccEnvironmentResource_adoptDefault covers taking over the
			// default environment.
			{
				Config: testAccEnvironmentConfig(appName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "name", "production"),
					resource.TestCheckResourceAttrSet("laravel_cloud_environment.test", "id"),
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "branch", testAccBranch()),
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "php_version", "8.4:1"),
				),
			},
			// Import
			{
				ResourceName:      "laravel_cloud_environment.test",
				ImportState:       true,
				ImportStateVerify: true,
				// application_id is not in the GET response and branch is
				// write-only (the API accepts it but never reports it back), so
				// an imported environment cannot know them and leaves them null.
				ImportStateVerifyIgnore: []string{"application_id", "branch"},
			},
		},
	})
}

// TestAccEnvironmentResource_adoptDefault takes over the environment Laravel
// Cloud creates with the application and then destroys it. The API
// refuses to delete an application's default environment, so destroy must
// leave it to the application's own deletion.
func TestAccEnvironmentResource_adoptDefault(t *testing.T) {
	appName := testAccName()
	// Laravel Cloud names an application's default environment "production"
	// when the repository's default branch is production-like (main, master,
	// prod, live or production), so a "production" environment takes it over.
	repository := testAccFixture(t, "LARAVEL_CLOUD_TEST_MAIN_BRANCH_REPOSITORY")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "production"
  branch         = "main"
}
`, appName, repository),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "name", "production"),
					testAccCheckDefaultEnvironment("laravel_cloud_application.test", "laravel_cloud_environment.test"),
				),
			},
		},
	})
}

// testAccCheckDefaultEnvironment checks that the environment is the
// application's default one, so the test really covers taking it over.
func testAccCheckDefaultEnvironment(appResource, envResource string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		appID := s.RootModule().Resources[appResource].Primary.ID
		envID := s.RootModule().Resources[envResource].Primary.ID
		app, err := testAccClient().GetApplication(context.Background(), appID)
		if err != nil {
			return err
		}
		if got := app.Relationships.DefaultEnvironment.RelatedID(); got != envID {
			return fmt.Errorf("application %s has default environment %q, want the managed environment %q", appID, got, envID)
		}
		return nil
	}
}

func TestAccEnvironmentResource_update(t *testing.T) {
	appName := testAccName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with initial settings
			{
				Config: testAccEnvironmentConfig(appName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "timeout", "30"),
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "uses_octane", "true"),
				),
			},
			// Update timeout
			{
				Config: testAccEnvironmentConfigUpdated(appName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "timeout", "45"),
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "uses_octane", "false"),
				),
			},
		},
	})
}

// TestAccEnvironmentResource_vanityDomain sets a vanity domain on a real
// environment. The API takes a single label and reports the full
// hostname, so a configured label must still plan empty after apply. It sets
// the domain only once: the API allows one change per environment every
// 30 minutes.
func TestAccEnvironmentResource_vanityDomain(t *testing.T) {
	appName := testAccName()
	vanity := testAccName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEnvironmentVanityConfig(appName, vanity),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "vanity_domain", vanity),
					resource.TestMatchResourceAttr("laravel_cloud_environment.test", "vanity_hostname",
						regexp.MustCompile(`^`+regexp.QuoteMeta(vanity)+`\.[a-z0-9.-]+$`)),
				),
			},
			{
				ResourceName:      "laravel_cloud_environment.test",
				ImportState:       true,
				ImportStateVerify: true,
				// php_version is not configured here; an import rebuilds it
				// from the reported major version.
				ImportStateVerifyIgnore: []string{"application_id", "branch", "php_version"},
			},
		},
	})
}

// TestAccEnvironmentResource_readBack covers color, timeout and
// shutdown_timeout, which the API reports. An unset shutdown_timeout
// takes the API's value, and a change made outside Terraform shows in the next
// plan and is put back.
func TestAccEnvironmentResource_readBack(t *testing.T) {
	appName := testAccName()
	var envID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEnvironmentReadBackConfig(appName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "color", "red"),
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "timeout", "30"),
					// Unset, so it is the platform default rather than null.
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "shutdown_timeout", "30"),
					func(s *terraform.State) error {
						envID = s.RootModule().Resources["laravel_cloud_environment.test"].Primary.ID
						return nil
					},
				),
			},
			// The dashboard changes color and timeout: the plan must notice.
			{
				PreConfig: func() {
					color, timeout := "green", 45
					if _, err := testAccClient().UpdateEnvironment(context.Background(), envID,
						client.UpdateEnvironmentRequest{Color: &color, Timeout: &timeout}); err != nil {
						t.Fatalf("changing the environment outside Terraform: %v", err)
					}
				},
				Config:             testAccEnvironmentReadBackConfig(appName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccEnvironmentReadBackConfig(appName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "color", "red"),
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "timeout", "30"),
				),
			},
			{
				ResourceName:            "laravel_cloud_environment.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"application_id", "branch", "php_version"},
			},
		},
	})
}

func testAccEnvironmentReadBackConfig(appName string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "staging"
  branch         = %[3]q
  color          = "red"
  timeout        = 30
}
`, appName, testAccRepository(), testAccBranch())
}

func testAccEnvironmentVanityConfig(appName, vanity string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "production"
  branch         = %[3]q
  vanity_domain  = %[4]q
}
`, appName, testAccRepository(), testAccBranch(), vanity)
}

func testAccEnvironmentConfig(appName string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "production"
  branch         = %[3]q
  php_version    = "8.4:1"
  node_version   = "22"

  uses_push_to_deploy = true
  uses_octane         = true
  timeout             = 30
}
`, appName, testAccRepository(), testAccBranch())
}

func testAccEnvironmentConfigUpdated(appName string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "production"
  branch         = %[3]q
  php_version    = "8.4:1"
  node_version   = "22"

  uses_push_to_deploy = true
  uses_octane         = false
  timeout             = 45
}
`, appName, testAccRepository(), testAccBranch())
}
