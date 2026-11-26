package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// The tests below guard a defect reported against an imported application and
// environment: the first plan after the import proposed an in-place update
// adding repository and php_version, although both already held exactly the
// configured values on the platform.
//
// Read never mapped either attribute. The API reports the repository as an
// object ({"full_name": ...}) and the PHP version only as php_major_version, so
// neither lines up with the attribute by name, and an import left both null
// while the configuration supplied a value.
//
// ImportBlockWithID models `terraform plan` with an import block: the resource
// is dropped from state and planned for import against the configuration, and
// the step fails unless that plan is a no-op.

const importReadbackStack = `
resource "laravel_cloud_application" "app" {
  name       = "imported-app"
  repository = "laravel/laravel"
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "env" {
  application_id = laravel_cloud_application.app.id
  name           = "production"
  branch         = "main"
  php_version    = "8.3:1"
}
`

func TestApplicationImportReadsRepositoryPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	runImportBlockIsNoOp(t, providerCfg(baseURL)+importReadbackStack, "laravel_cloud_application.app")
}

func TestEnvironmentImportReadsPHPVersionPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	runImportBlockIsNoOp(t, providerCfg(baseURL)+importReadbackStack, "laravel_cloud_environment.env")
}

// An environment that leaves php_version unset must not start planning a
// change once the attribute is read back.
func TestEnvironmentImportWithoutPHPVersionPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	runImportBlockIsNoOp(t, providerCfg(baseURL)+appEnvStack, "laravel_cloud_environment.env")
}

// Read only fills these attributes in when nothing is recorded. A recorded
// value is kept even when the API reports something else, so a refresh never
// plans a change -- and never a PATCH to the platform -- on its own.
func TestRecordedValuesKeptOnReadPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	config := providerCfg(baseURL) + importReadbackStack

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					f.mu.Lock()
					defer f.mu.Unlock()
					for _, app := range f.apps {
						app.Attributes.Repository = &client.ApplicationRepository{FullName: "laravel/renamed"}
					}
					for _, env := range f.envs {
						env.Attributes.PHPMajorVersion = "8.5"
					}
				},
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

// An unset php_version records nothing, so pinning one later always reaches the
// platform -- even when it names the version the environment ran at create
// time, which a change made outside Terraform has since replaced. Recording
// the platform's version on create made that pin plan nothing.
func TestEnvironmentPinPHPVersionAfterOutsideChangePlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	env := func(extra string) string {
		return providerCfg(baseURL) + `
resource "laravel_cloud_application" "app" {
  name       = "pin-app"
  repository = "laravel/laravel"
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "env" {
  application_id = laravel_cloud_application.app.id
  name           = "production"
  branch         = "main"
  ` + extra + `
}
`
	}
	phpMajor := func() string {
		f.mu.Lock()
		defer f.mu.Unlock()
		// The application's default environment is "13 x"; this test's is
		// "production".
		for _, e := range f.envs {
			if e.Attributes.Name == "production" {
				return e.Attributes.PHPMajorVersion
			}
		}
		return ""
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// The fake starts every environment on 8.4.
			{Config: env("")},
			{
				PreConfig: func() {
					f.mu.Lock()
					defer f.mu.Unlock()
					for _, e := range f.envs {
						e.Attributes.PHPMajorVersion = "8.3"
					}
				},
				Config: env(`php_version = "8.4:1"`),
				Check: func(*terraform.State) error {
					if got := phpMajor(); got != "8.4" {
						return fmt.Errorf("platform PHP version = %q after pinning 8.4:1, want 8.4", got)
					}
					return nil
				},
			},
		},
	})
}

// source_control_provider_type is reported back, so adding the reported one
// changes nothing. The API reads it only to resolve a repository, so it goes
// out with one: with a repository change, and with the current repository when
// only the provider type changes, which points the application at the
// repository of that name on the other provider. Only a configured one goes
// out, since sending it makes the API use the token creator's connection to
// that provider: unset, a repository change reads it back instead.
func TestSourceControlProviderTypeSentWithRepositoryPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	const addr = "laravel_cloud_application.app"
	app := func(repository, extra string) string {
		return providerCfg(baseURL) + `
resource "laravel_cloud_application" "app" {
  name       = "scm-app"
  repository = "` + repository + `"
  region     = "us-east-2"
  ` + extra + `
}
`
	}
	lastUpdate := func(check func(client.UpdateApplicationRequest) error) resource.TestCheckFunc {
		return func(*terraform.State) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.appUpdates) == 0 {
				return fmt.Errorf("no application update was sent")
			}
			return check(f.appUpdates[len(f.appUpdates)-1])
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: app("laravel/laravel", ""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("source_control_provider_type"), knownvalue.StringExact("github")),
				},
			},
			{
				// Unset, it stays out of a repository change, which Laravel Cloud
				// then resolves through the organization's connections, and the
				// provider it reports is read back.
				Config: app("laravel/moved", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectUnknownValue(addr, tfjsonpath.New("source_control_provider_type")),
				}},
				Check: lastUpdate(func(req client.UpdateApplicationRequest) error {
					if req.Repository == nil || *req.Repository != "laravel/moved" || req.SourceControlProviderType != nil {
						return fmt.Errorf("an unset provider type must stay out of a repository change, got %+v", req)
					}
					return nil
				}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("source_control_provider_type"), knownvalue.StringExact("github")),
				},
			},
			{
				Config:           app("laravel/moved", `source_control_provider_type = "github"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: app("laravel/renamed", `source_control_provider_type = "github"`),
				Check: lastUpdate(func(req client.UpdateApplicationRequest) error {
					if req.Repository == nil || req.SourceControlProviderType == nil || *req.SourceControlProviderType != "github" {
						return fmt.Errorf("a repository change must carry source_control_provider_type, got %+v", req)
					}
					return nil
				}),
			},
			{
				Config: app("laravel/renamed", `source_control_provider_type = "gitlab"`),
				// Another repository, whose details the apply reads back.
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectUnknownValue(addr, tfjsonpath.New("repository_default_branch")),
					plancheck.ExpectUnknownValue(addr, tfjsonpath.New("framework")),
					plancheck.ExpectUnknownValue(addr, tfjsonpath.New("language")),
				}},
				Check: lastUpdate(func(req client.UpdateApplicationRequest) error {
					if req.Repository == nil || *req.Repository != "laravel/renamed" || req.SourceControlProviderType == nil || *req.SourceControlProviderType != "gitlab" {
						return fmt.Errorf("a provider type change must carry the repository, got %+v", req)
					}
					return nil
				}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("source_control_provider_type"), knownvalue.StringExact("gitlab")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("repository_default_branch"), knownvalue.StringExact("master")),
				},
			},
			{
				Config:           app("laravel/renamed", `source_control_provider_type = "gitlab"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// TestRepositoryMoveRestoresBranchesPlan: Laravel Cloud detaches every
// environment's branch when the application moves to another repository,
// including the one of the same name on another provider. The refresh records
// that, so the next plan sets the configured branch again instead of
// reporting nothing to do.
func TestRepositoryMoveRestoresBranchesPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	const env = "laravel_cloud_environment.staging"
	config := func(provider string) string {
		return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_application" "app" {
  name                         = "move-app"
  repository                   = "acme/move-app"
  region                       = "us-east-2"
  source_control_provider_type = %q
}

resource "laravel_cloud_environment" "staging" {
  application_id = laravel_cloud_application.app.id
  name           = "staging"
  branch         = "staging"
}
`, provider)
	}
	branch := func(want string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			id := s.RootModule().Resources[env].Primary.ID
			f.mu.Lock()
			defer f.mu.Unlock()
			if got := f.envs[id].BranchName; got != want {
				return fmt.Errorf("staging branch = %q, want %q", got, want)
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config("github")},
			{
				Config:             config("gitlab"),
				Check:              branch(""),
				ExpectNonEmptyPlan: true,
			},
			{
				Config: config("gitlab"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(env, plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction("laravel_cloud_application.app", plancheck.ResourceActionNoop),
				}},
				Check: branch("staging"),
			},
			{
				Config:           config("gitlab"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// runImportBlockIsNoOp applies the config, then plans an import of the named
// resource against that same config and requires no changes. Import blocks
// need Terraform 1.5.
func runImportBlockIsNoOp(t *testing.T, config, addr string) {
	t.Helper()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_5_0)},
		Steps: []resource.TestStep{
			{Config: config},
			{
				Config:          config,
				ResourceName:    addr,
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
			},
		},
	})
}
