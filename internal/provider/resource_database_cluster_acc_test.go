package provider

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// With storage_autoscale_max_gb set, the platform grows storage on its
// own once the disk fills. storage in config is then a minimum, not a target:
// the API refuses any value below the current storage, and any storage change
// within six hours of the last one. Before the fix, the next plan proposed
// setting storage back to the configured value, and every apply -- including
// one for an unrelated config change -- failed with a 422.
//
// The test grows storage through the API, the same update the autoscaler
// makes, instead of filling the disk.
func TestAccDatabaseCluster_storageAutoscale(t *testing.T) {
	name := testAccName()
	var clusterID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDatabaseClusterAutoscaleConfig(name, 0),
				Check: func(s *terraform.State) error {
					clusterID = s.RootModule().Resources["laravel_cloud_database_cluster.test"].Primary.ID
					return nil
				},
			},
			// Autoscaling grew storage from 10 to 15 GiB: nothing to change.
			{
				PreConfig: func() { testAccGrowClusterStorage(t, clusterID, 15) },
				Config:    testAccDatabaseClusterAutoscaleConfig(name, 0),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Another config change still applies, and leaves storage alone.
			{
				PreConfig: func() { testAccWaitClusterAvailable(t, clusterID) },
				Config:    testAccDatabaseClusterAutoscaleConfig(name, 1),
				Check: func(*terraform.State) error {
					cluster, err := testAccClient().GetDatabaseCluster(context.Background(), clusterID)
					if err != nil {
						return err
					}
					if got := cluster.Attributes.Config["storage"]; got != float64(15) {
						return fmt.Errorf("storage = %v, want 15", got)
					}
					if got := cluster.Attributes.Config["retention_days"]; got != float64(1) {
						return fmt.Errorf("retention_days = %v, want 1", got)
					}
					return nil
				},
			},
		},
	})
}

func testAccDatabaseClusterAutoscaleConfig(name string, retentionDays int) string {
	return fmt.Sprintf(`
resource "laravel_cloud_database_cluster" "test" {
  name                    = %[1]q
  type                    = "laravel_mysql"
  version                 = "8.4"
  region                  = "us-east-2"
  create_default_database = false
  force_destroy           = true
  config = jsonencode({
    size                     = "mysql-flex-512mb"
    storage                  = 10
    storage_autoscale_max_gb = 50
    is_public                = false
    uses_scheduled_snapshots = false
    retention_days           = %[2]d
  })
}
`, name, retentionDays)
}

// testAccGrowClusterStorage makes the update the storage autoscaler makes,
// once the new cluster is ready to take it.
func testAccGrowClusterStorage(t *testing.T, clusterID string, storage int) {
	t.Helper()
	c := testAccClient()
	err := client.RetryOnConflict(context.Background(), databaseBusyAttempts, databaseBusyInterval, func() error {
		_, err := c.UpdateDatabaseCluster(context.Background(), clusterID, client.UpdateDatabaseClusterRequest{
			Config: map[string]any{"storage": storage},
		})
		return err
	}, client.IsBusy)
	if err != nil {
		t.Fatalf("growing storage of cluster %s: %v", clusterID, err)
	}
}

// testAccWaitClusterAvailable waits until a cluster has finished updating.
func testAccWaitClusterAvailable(t *testing.T, clusterID string) {
	t.Helper()
	for i := 0; i < databaseBusyAttempts; i++ {
		cluster, err := testAccClient().GetDatabaseCluster(context.Background(), clusterID)
		if err != nil {
			t.Fatalf("reading cluster %s: %v", clusterID, err)
		}
		if cluster.Attributes.Status == "available" {
			return
		}
		time.Sleep(databaseBusyInterval)
	}
	t.Fatalf("cluster %s did not become available", clusterID)
}
