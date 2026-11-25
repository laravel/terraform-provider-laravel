package provider

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

// TestAccApplicationResource_appEnvFields covers the application and
// environment fields: a static site created with serving and
// build_command, the read-only details, slack_channel turned off by removing
// it, a different default environment, and post_deploy_command, which the
// test organization does not have and Laravel Cloud silently ignores.
func TestAccApplicationResource_appEnvFields(t *testing.T) {
	rName := testAccName()
	// Set from the first step: the environment depends on the application, so
	// the configuration cannot reference it in default_environment_id.
	var stagingID string
	cfg := func(appExtra, envExtra string) string {
		return fmt.Sprintf(`
variable "staging_id" {
  type    = string
  default = null
}

resource "laravel_cloud_application" "test" {
  name                         = %[1]q
  repository                   = %[2]q
  source_control_provider_type = "github"
  region                       = "us-east-2"
  serving_mode                 = "static"
  serving_root                 = "public"
  build_command                = "echo built"
  %[3]s
}

resource "laravel_cloud_environment" "staging" {
  application_id = laravel_cloud_application.test.id
  name           = "staging"
  branch         = %[5]q
  serving_mode   = "process"
  %[4]s
}
`, rName, testAccRepository(), appExtra, envExtra, testAccBranch())
	}
	const app = "laravel_cloud_application.test"
	const env = "laravel_cloud_environment.staging"
	staging := config.Variables{"staging_id": lazyStringVariable{&stagingID}}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg(`slack_channel = "general"`, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(app, "slack_channel", "general"),
					resource.TestCheckResourceAttrSet(app, "default_environment_id"),
					resource.TestCheckResourceAttr(app, "framework", "laravel"),
					resource.TestCheckResourceAttr(app, "language", "php"),
					resource.TestCheckResourceAttr(app, "repository_default_branch", testAccBranch()),
					resource.TestCheckResourceAttr(env, "serving_mode", "process"),
					resource.TestCheckResourceAttr(env, "framework", "laravel"),
					resource.TestCheckResourceAttr(env, "language", "php"),
					resource.TestCheckResourceAttr(env, "display_name", "staging"),
					resource.TestCheckResourceAttr(env, "created_from_automation", "false"),
					resource.TestCheckResourceAttrSet(env, "uses_hibernation"),
					resource.TestCheckNoResourceAttr(env, "pull_request"),
					func(s *terraform.State) error {
						stagingID = s.RootModule().Resources[env].Primary.ID
						return nil
					},
				),
			},
			{
				Config:          cfg(`default_environment_id = var.staging_id`, ""),
				ConfigVariables: staging,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(app, "slack_channel"),
					func(s *terraform.State) error {
						if got := s.RootModule().Resources[app].Primary.Attributes["default_environment_id"]; got != stagingID {
							return fmt.Errorf("default_environment_id = %q, want staging %q", got, stagingID)
						}
						return nil
					},
				),
			},
			{
				Config:          cfg(`default_environment_id = var.staging_id`, `post_deploy_command = "php artisan about"`),
				ExpectError:     regexp.MustCompile(`Laravel Cloud did not apply post_deploy_command`),
				ConfigVariables: staging,
			},
		},
	})
}

// lazyStringVariable is a Terraform variable read when the step runs, for a
// value an earlier step produced.
type lazyStringVariable struct{ v *string }

func (l lazyStringVariable) MarshalJSON() ([]byte, error) { return json.Marshal(*l.v) }
