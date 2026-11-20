package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestDatabaseClusterBusyPlan is the fast counterpart of
// TestAccDatabase_createWithCluster: a database created in the same
// apply as its cluster, and the cluster destroyed right after its databases.
// The fake refuses both while the cluster is busy, with the API's real 422
// messages, so a provider that does not recognise them fails here.
func TestDatabaseClusterBusyPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             expectNoClustersLeft(f),
		Steps: []resource.TestStep{
			{Config: providerCfg(baseURL) + clusterStack + `
resource "laravel_cloud_database" "db" {
  cluster_id = laravel_cloud_database_cluster.cluster.id
  name       = "app_db"
}
`},
		},
	})
}

// TestDatabaseRestoreForceDestroyPlan: a restored cluster holds
// copies of the source's databases, so it can only be destroyed by deleting
// them, which force_destroy opts in to. Without it the destroy fails and names
// them; setting it later changes the restore in place.
func TestDatabaseRestoreForceDestroyPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             expectNoClustersLeft(f),
		Steps: []resource.TestStep{
			{Config: databaseRestoreConfig(baseURL, false)},
			{
				Config:      databaseRestoreConfig(baseURL, false),
				Destroy:     true,
				ExpectError: regexp.MustCompile(`still contains 1\s+database\(s\): production`),
			},
			{
				Config: databaseRestoreConfig(baseURL, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("laravel_cloud_database_restore.restore", plancheck.ResourceActionUpdate),
				}},
			},
		},
	})
}

func databaseRestoreConfig(baseURL string, forceDestroy bool) string {
	return providerCfg(baseURL) + clusterStack + fmt.Sprintf(`
resource "laravel_cloud_database_restore" "restore" {
  database_cluster_id = laravel_cloud_database_cluster.cluster.id
  name                = "restored-db"
  restore_time        = "2026-01-01T00:00:00Z"
  force_destroy       = %t
}
`, forceDestroy)
}

func expectNoClustersLeft(f *fakeCloud) resource.TestCheckFunc {
	return func(*terraform.State) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		if n := len(f.objects[clusterType]); n != 0 {
			return fmt.Errorf("want every database cluster deleted, %d left", n)
		}
		return nil
	}
}

// TestDatabaseRestoreSharedIDPlan guards the source cluster of a Serverless
// Postgres restore. The API gives the restored cluster the source's ID, so a
// DELETE for the restore deletes the source; removing the restore from the
// configuration must leave the source and its databases alone.
func TestDatabaseRestoreSharedIDPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	cluster := providerCfg(baseURL) + `
resource "laravel_cloud_database_cluster" "cluster" {
  force_destroy = true
  name          = "serverless"
  type          = "neon_serverless_postgres_17"
  region        = "us-east-2"
  config        = jsonencode({ cu_min = 0.25, cu_max = 0.25 })
}
`
	restore := `
resource "laravel_cloud_database_restore" "restore" {
  database_cluster_id = laravel_cloud_database_cluster.cluster.id
  name                = "restored-db"
  restore_time        = "2026-01-01T00:00:00Z"
  force_destroy       = true
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cluster + restore},
			{
				Config: cluster,
				Check: func(*terraform.State) error {
					f.mu.Lock()
					defer f.mu.Unlock()
					if len(f.objects[clusterType]) != 1 || len(f.objects["database"]) != 1 {
						return fmt.Errorf("want the source cluster and its database kept, got clusters %v, databases %v",
							f.objects[clusterType], f.objects["database"])
					}
					return nil
				},
			},
		},
	})
}
