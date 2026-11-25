package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// testAccDedicatedAppCluster returns the ID of the organization's first
// active dedicated cluster for applications in us-east-2, the region
// testAccInternalNetworkingConfig creates the application in, and skips the
// test when there is none. Internal networking needs a dedicated cluster.
func testAccDedicatedAppCluster(t *testing.T) string {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC must be set for acceptance tests")
	}
	testAccPreCheck(t)
	clusters, err := testAccClient().ListDedicatedClusters(context.Background(), map[string]string{"type": "applications", "status": "active", "region": "us-east-2"})
	if err != nil {
		t.Fatalf("listing dedicated clusters: %v", err)
	}
	for _, c := range clusters {
		if c.Attributes.ClusterType == "applications" && c.Attributes.Region == "us-east-2" {
			return c.ID
		}
	}
	t.Skip("the organization has no active dedicated cluster for applications in us-east-2 (internal networking is Private Cloud only)")
	return ""
}

func testAccInternalNetworkingConfig(name, clusterID, access string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
  cluster_id = %[4]q
}

resource "laravel_cloud_environment" "backend" {
  application_id = laravel_cloud_application.test.id
  name           = "staging"
  branch         = %[3]q
  cluster_id     = %[4]q
}

resource "laravel_cloud_environment" "api" {
  application_id = laravel_cloud_application.test.id
  name           = "staging-api"
  branch         = %[3]q
  cluster_id     = %[4]q
}

resource "laravel_cloud_environment_internal_networking" "backend" {
  environment_id = laravel_cloud_environment.backend.id
  %[5]s
}
`, name, testAccRepository(), testAccBranch(), clusterID, access)
}

// TestAccEnvironmentInternalNetworking_basic turns internal networking on
// for an environment on a dedicated cluster, narrows it to one other
// environment in place, and imports it.
func TestAccEnvironmentInternalNetworking_basic(t *testing.T) {
	clusterID := testAccDedicatedAppCluster(t)
	name := testAccName()
	const addr = "laravel_cloud_environment_internal_networking.backend"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccInternalNetworkingConfig(name, clusterID, `access_level = "all"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "access_level", "all"),
					resource.TestMatchResourceAttr(addr, "hostname", regexp.MustCompile(`\.private\.laravel\.cloud$`)),
					resource.TestCheckNoResourceAttr(addr, "environment_ids"),
				),
			},
			{
				Config: testAccInternalNetworkingConfig(name, clusterID, `access_level = "specific"
  environment_ids = [laravel_cloud_environment.api.id]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "access_level", "specific"),
					resource.TestCheckTypeSetElemAttrPair(addr, "environment_ids.*", "laravel_cloud_environment.api", "id"),
				),
			},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
