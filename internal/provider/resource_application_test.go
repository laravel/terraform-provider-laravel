package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccApplicationResource_basic(t *testing.T) {
	rName := testAccName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccApplicationConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_application.test", "name", rName),
					resource.TestCheckResourceAttrSet("laravel_cloud_application.test", "id"),
					resource.TestCheckResourceAttr("laravel_cloud_application.test", "region", "us-east-2"),
				),
			},
			// Import
			{
				ResourceName:      "laravel_cloud_application.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccApplicationConfig(name string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}
`, name, testAccRepository())
}

// TestAccApplicationResource_slugAndRootDirectory sets configured slugs on an
// application and an environment, on create and on update, then changes the
// repository to one without the configured root_directory. Laravel
// Cloud clears it; the apply must say so rather than leave a plan that
// replaces the application and deletes its environments.
func TestAccApplicationResource_slugAndRootDirectory(t *testing.T) {
	rName := testAccName()
	slug := strings.ToLower(rName)
	config := func(repository, rootDirectory, appSlug, envSlug string) string {
		return fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name                         = %[1]q
  repository                   = %[2]q
  source_control_provider_type = "github"
  region                       = "us-east-2"
  slug                         = %[4]q
  %[3]s
}

resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "staging"
  branch         = "main"
  slug           = %[5]q
}
`, rName, repository, rootDirectory, appSlug, envSlug)
	}
	// The monorepo holds a backend/ directory; the test repository has none.
	monorepo := testAccFixture(t, "LARAVEL_CLOUD_TEST_MONOREPO_REPOSITORY")
	const backend = `root_directory = "backend"`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(monorepo, backend, slug+"-custom", "staging-custom"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_application.test", "slug", slug+"-custom"),
					resource.TestCheckResourceAttr("laravel_cloud_application.test", "root_directory", "backend"),
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "slug", "staging-custom"),
				),
			},
			{
				Config: config(monorepo, backend, slug+"-renamed", "staging-renamed"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_application.test", "slug", slug+"-renamed"),
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "slug", "staging-renamed"),
				),
			},
			{
				Config:      config(testAccRepository(), backend, slug+"-renamed", "staging-renamed"),
				ExpectError: regexp.MustCompile(`Laravel Cloud cleared root_directory`),
			},
			{
				Config:           config(testAccRepository(), "", slug+"-renamed", "staging-renamed"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				Check:            resource.TestCheckNoResourceAttr("laravel_cloud_application.test", "root_directory"),
			},
		},
	})
}
