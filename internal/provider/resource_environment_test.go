package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEnvironmentResource_basic(t *testing.T) {
	appName := testAccName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read — adopts the auto-created "production" environment
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
				// application_id is not in the GET response; branch and timeout
				// are write-only (the API accepts them but never reports them
				// back), so an imported environment cannot know them and leaves
				// them null.
				ImportStateVerifyIgnore: []string{"application_id", "branch", "timeout"},
			},
		},
	})
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
