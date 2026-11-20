package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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
