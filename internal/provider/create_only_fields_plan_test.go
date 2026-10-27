package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func applicationConfig(baseURL, extra string) string {
	return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_application" "app" {
  name                         = "monorepo-app"
  repository                   = "acme/monorepo"
  region                       = "us-east-2"
  source_control_provider_type = "github"
%s
}
`, extra)
}

// TestApplicationRootDirectoryPlan covers root_directory: sent on create, read
// back, replaced when changed -- and adopted rather than replaced when it was
// set outside Terraform and the configuration does not mention it, which is
// what an existing application looks like after upgrading the provider.
func TestApplicationRootDirectoryPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_application.app"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: applicationConfig(baseURL, `  root_directory = "apps/web"`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("root_directory"), knownvalue.StringExact("apps/web")),
				},
			},
			{
				Config:           applicationConfig(baseURL, `  root_directory = "apps/web"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: applicationConfig(baseURL, `  root_directory = "apps/api"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionDestroyBeforeCreate),
				}},
			},
			{
				// Unset in config: the reported directory is kept, not replaced.
				Config:           applicationConfig(baseURL, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("root_directory"), knownvalue.StringExact("apps/api")),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"source_control_provider_type"},
			},
		},
	})
}

// TestApplicationStarterKitPlan sends starter_kit on create only. The API never
// reports it, so the configured value is what state keeps.
func TestApplicationStarterKitPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_application.app"
	config := applicationConfig(baseURL, `  starter_kit = "laravel-react"`)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("starter_kit"), knownvalue.StringExact("laravel-react")),
				},
				Check: func(*terraform.State) error {
					f.mu.Lock()
					defer f.mu.Unlock()
					last := f.appCreates[len(f.appCreates)-1]
					if last.StarterKit == nil || *last.StarterKit != "laravel-react" {
						return fmt.Errorf("starter_kit was not sent on create: %+v", last)
					}
					return nil
				},
			},
			{
				Config:           config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// TestApplicationStarterKitConflictPlan rejects a starter kit combined with a root
// directory, which the API refuses.
func TestApplicationStarterKitConflictPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      applicationConfig(baseURL, "  starter_kit = \"laravel\"\n  root_directory = \"apps/web\""),
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
		},
	})
}

func bucketConfig(baseURL, extra string) string {
	return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_storage_bucket" "bucket" {
  name     = "assets"
  key_name = "primary"
%s
}
`, extra)
}

// TestStorageBucketLocationHintPlan reads the platform's default when unset
// and forces a new bucket when the hint changes.
func TestStorageBucketLocationHintPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_storage_bucket.bucket"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: bucketConfig(baseURL, ""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("location_hint"), knownvalue.StringExact("automatic")),
				},
			},
			{
				Config: bucketConfig(baseURL, `  location_hint = "weur"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionDestroyBeforeCreate),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("location_hint"), knownvalue.StringExact("weur")),
				},
			},
			{
				Config:           bucketConfig(baseURL, `  location_hint = "weur"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// TestDatabaseClusterWithoutDefaultDatabasePlan creates a cluster without the
// database the platform would otherwise put in it, which also means it can be
// destroyed without force_destroy.
func TestDatabaseClusterWithoutDefaultDatabasePlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_database_cluster.cluster"
	config := providerCfg(baseURL) + `
resource "laravel_cloud_database_cluster" "cluster" {
  name                    = "empty-db"
  type                    = "laravel_mysql_84"
  region                  = "us-east-1"
  config                  = jsonencode({ size = "db-1vcpu-1gb" })
  create_default_database = false
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("create_default_database"), knownvalue.Bool(false)),
				},
				Check: func(s *terraform.State) error {
					id := s.RootModule().Resources[addr].Primary.ID
					f.mu.Lock()
					defer f.mu.Unlock()
					if dbs := f.children["database:"+id]; len(dbs) != 0 {
						return fmt.Errorf("the cluster got a default database: %v", dbs)
					}
					return nil
				},
			},
			{
				Config:           config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}
