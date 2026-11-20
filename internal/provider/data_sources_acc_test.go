package provider

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccDataSources_listsAndFilters reads the data sources
// against the live API, and the region/type/status and name/domain/status
// filters on the dedicated cluster and edge network lists. The reusable
// Resend keys need an environment, so the test creates an application with a
// staging environment.
func TestAccDataSources_listsAndFilters(t *testing.T) {
	name := testAccName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "staging"
  branch         = %[3]q
}

data "laravel_cloud_websocket_server_types" "all" {}

data "laravel_cloud_database_presets" "mysql" {
  type   = "laravel_mysql"
  region = "us-east-2"
}

data "laravel_cloud_resend_domains" "all" {}

data "laravel_cloud_resend_reusable_keys" "staging" {
  environment_id = laravel_cloud_environment.test.id
}

data "laravel_cloud_dedicated_clusters" "all" {}

data "laravel_cloud_dedicated_clusters" "filtered" {
  region = "us-east-2"
  status = "active"
}

data "laravel_cloud_dedicated_clusters" "none" {
  type = "tfacc-no-such-type"
}

data "laravel_cloud_edge_networks" "shared" {
  domain = "laravel.cloud"
  status = "available"
}

data "laravel_cloud_edge_networks" "none" {
  name = "tfacc-no-such-network"
}
`, name, testAccRepository(), testAccBranch()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.laravel_cloud_websocket_server_types.all", "types.0.type", "reverb"),
					resource.TestCheckResourceAttrSet("data.laravel_cloud_websocket_server_types.all", "types.0.max_connections.0.value"),

					resource.TestCheckResourceAttrSet("data.laravel_cloud_database_presets.mysql", "presets.0.key"),
					resource.TestCheckResourceAttrWith("data.laravel_cloud_database_presets.mysql", "presets.0.config",
						func(v string) error {
							if !strings.Contains(v, `"size"`) {
								return fmt.Errorf("config %s has no size", v)
							}
							return nil
						}),

					// An organization without sending domains or other
					// environments with Resend gets empty lists, not null.
					resource.TestCheckResourceAttrSet("data.laravel_cloud_resend_domains.all", "domains.#"),
					resource.TestCheckResourceAttrSet("data.laravel_cloud_resend_reusable_keys.staging", "keys.#"),

					testAccCheckListFiltered("data.laravel_cloud_dedicated_clusters.filtered", "clusters", map[string]string{
						"region": "us-east-2", "status": "active",
					}),
					testAccCheckFilterNarrows("data.laravel_cloud_dedicated_clusters.all", "data.laravel_cloud_dedicated_clusters.filtered", "clusters"),
					resource.TestCheckResourceAttr("data.laravel_cloud_dedicated_clusters.none", "clusters.#", "0"),

					testAccCheckListFiltered("data.laravel_cloud_edge_networks.shared", "edge_networks", map[string]string{
						"domain": "laravel.cloud", "status": "available",
					}),
					resource.TestCheckTypeSetElemNestedAttrs("data.laravel_cloud_edge_networks.shared", "edge_networks.*",
						map[string]string{"domain": "laravel.cloud", "tenancy_type": "shared"}),
					resource.TestCheckResourceAttr("data.laravel_cloud_edge_networks.none", "edge_networks.#", "0"),
				),
			},
		},
	})
}

// testAccCheckListFiltered checks that every item of a data source's list
// contains each filter value in the attribute of the same name.
func testAccCheckListFiltered(addr, list string, filters map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return fmt.Errorf("%s not found in state", addr)
		}
		n, err := strconv.Atoi(rs.Primary.Attributes[list+".#"])
		if err != nil {
			return fmt.Errorf("%s: %s.# is %q", addr, list, rs.Primary.Attributes[list+".#"])
		}
		for i := 0; i < n; i++ {
			for attr, want := range filters {
				got := rs.Primary.Attributes[fmt.Sprintf("%s.%d.%s", list, i, attr)]
				if !strings.Contains(got, want) {
					return fmt.Errorf("%s: %s.%d.%s is %q, which does not contain the filter %q", addr, list, i, attr, got, want)
				}
			}
		}
		return nil
	}
}

// testAccCheckFilterNarrows checks that a filtered list has no more items
// than the unfiltered one.
func testAccCheckFilterNarrows(allAddr, filteredAddr, list string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		all, _ := strconv.Atoi(s.RootModule().Resources[allAddr].Primary.Attributes[list+".#"])
		filtered, _ := strconv.Atoi(s.RootModule().Resources[filteredAddr].Primary.Attributes[list+".#"])
		if filtered > all {
			return fmt.Errorf("the filtered list has %d items, more than the %d unfiltered", filtered, all)
		}
		return nil
	}
}
