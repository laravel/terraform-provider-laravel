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

// TestDatabaseUserPlan covers laravel_cloud_database_user against
// the fake API: a user created in the same apply as its cluster (refused while
// the cluster is busy), the password kept although only the create response
// has it, type and databases changed in place, drift when a granted database
// is deleted outside Terraform, import, and destroy.
func TestDatabaseUserPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_database_user.reader"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			for id, u := range f.objects[databaseUserType] {
				if u["type"] != "superuser" {
					return fmt.Errorf("database user %s (%v) still exists after destroy", id, u["username"])
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: databaseUserPlanConfig(baseURL, "read_only", "[laravel_cloud_database.app.id]"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("status"), knownvalue.StringExact("available")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("type"), knownvalue.StringExact("read_only")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("password"), knownvalue.StringRegexp(regexp.MustCompile(`^password-\d+$`))),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("databases"), knownvalue.SetSizeExact(1)),
				},
			},
			// Reads do not return the password; it must stay in state.
			{
				Config: databaseUserPlanConfig(baseURL, "read_only", "[laravel_cloud_database.app.id]"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("password"), knownvalue.StringRegexp(regexp.MustCompile(`^password-\d+$`))),
				},
			},
			{
				Config: databaseUserPlanConfig(baseURL, "read_write", "[laravel_cloud_database.app.id, laravel_cloud_database.reports.id]"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("type"), knownvalue.StringExact("read_write")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("databases"), knownvalue.SetSizeExact(2)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("password"), knownvalue.StringRegexp(regexp.MustCompile(`^password-\d+$`))),
				},
				Check: func(*terraform.State) error {
					f.mu.Lock()
					defer f.mu.Unlock()
					patches := f.patches[databaseUserType]
					if len(patches) != 1 {
						return fmt.Errorf("want 1 PATCH, got %d: %v", len(patches), patches)
					}
					if dbs, _ := patches[0]["databases"].([]any); patches[0]["type"] != "read_write" || len(dbs) != 2 {
						return fmt.Errorf("PATCH body = %v, want type read_write and both databases", patches[0])
					}
					return nil
				},
			},
			// Access to none is an empty set, sent as an empty list.
			{
				Config: databaseUserPlanConfig(baseURL, "read_write", "[]"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("databases"), knownvalue.SetSizeExact(0)),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources[addr].Primary
					return rs.Attributes["cluster_id"] + ":" + rs.ID, nil
				},
			},
		},
	})
}

// TestDatabaseUserCreateWithClusterPlan: a user with access to no databases
// is created in the same apply as its cluster, which refuses it while it is
// still being created.
func TestDatabaseUserCreateWithClusterPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	runPlanStability(t, providerCfg(baseURL)+clusterStack+`
resource "laravel_cloud_database_user" "none" {
  cluster_id = laravel_cloud_database_cluster.cluster.id
  username   = "_tenant_1"
  type       = "read_write"
  databases  = []
}
`, nil, []statecheck.StateCheck{
		statecheck.ExpectKnownValue("laravel_cloud_database_user.none", tfjsonpath.New("status"), knownvalue.StringExact("available")),
	})
}

// TestDatabaseUserDatabaseDeletedPlan: deleting a granted database outside
// Terraform removes it from the user's access, which the next plan shows as
// an in-place update of the user, not a replacement.
func TestDatabaseUserDatabaseDeletedPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_database_user.reader"
	config := databaseUserPlanConfig(baseURL, "read_only", "[laravel_cloud_database.reports.id]")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					f.mu.Lock()
					defer f.mu.Unlock()
					for id, db := range f.objects["database"] {
						if db["name"] == "reports" {
							delete(f.objects["database"], id)
						}
					}
				},
				// The database is created again, and the user is given
				// access to it again in place.
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("laravel_cloud_database.reports", plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("databases"), knownvalue.SetSizeExact(1)),
				},
			},
		},
	})
}

// TestDatabaseUserImportSuperuserPlan: the cluster's superuser is listed with
// its users, but the API refuses to change or delete it, so importing it is
// refused with a clear error.
func TestDatabaseUserImportSuperuserPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	config := providerCfg(baseURL) + clusterStack + `
resource "laravel_cloud_database_user" "su" {
  cluster_id = laravel_cloud_database_cluster.cluster.id
  username   = "forge"
  type       = "read_write"
  databases  = []
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: providerCfg(baseURL) + clusterStack},
			{
				Config:        config,
				ResourceName:  "laravel_cloud_database_user.su",
				ImportState:   true,
				ImportStateId: "placeholder",
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					clusterID := s.RootModule().Resources["laravel_cloud_database_cluster.cluster"].Primary.ID
					f.mu.Lock()
					defer f.mu.Unlock()
					for _, id := range f.children[databaseUserType+":"+clusterID] {
						if f.objects[databaseUserType][id]["type"] == "superuser" {
							return clusterID + ":" + id, nil
						}
					}
					return "", fmt.Errorf("no superuser in cluster %s", clusterID)
				},
				ExpectError: regexp.MustCompile(`cluster's superuser`),
			},
		},
	})
}

func databaseUserPlanConfig(baseURL, userType, databases string) string {
	return providerCfg(baseURL) + clusterStack + fmt.Sprintf(`
resource "laravel_cloud_database" "app" {
  cluster_id = laravel_cloud_database_cluster.cluster.id
  name       = "app"
}

resource "laravel_cloud_database" "reports" {
  cluster_id = laravel_cloud_database_cluster.cluster.id
  name       = "reports"
}

resource "laravel_cloud_database_user" "reader" {
  cluster_id = laravel_cloud_database_cluster.cluster.id
  username   = "app_reader"
  type       = %q
  databases  = %s
}
`, userType, databases)
}
