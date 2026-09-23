package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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
		for _, e := range f.envs {
			return e.Attributes.PHPMajorVersion
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

// source_control_provider_type only tells the API how to resolve a repository
// being changed; it discards the field otherwise. Adding it on its own must not
// reach the platform, which is what the plan warning on it promises.
func TestSourceControlProviderTypeSentWithRepositoryPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
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
			{Config: app("laravel/laravel", "")},
			{
				Config: app("laravel/laravel", `source_control_provider_type = "github"`),
				Check: lastUpdate(func(req client.UpdateApplicationRequest) error {
					if req.SourceControlProviderType != nil {
						return fmt.Errorf("sent source_control_provider_type %q without a repository change", *req.SourceControlProviderType)
					}
					return nil
				}),
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
		},
	})
}

// runImportBlockIsNoOp applies the config, then plans an import of the named
// resource against that same config and requires no changes.
func runImportBlockIsNoOp(t *testing.T, config, addr string) {
	t.Helper()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
