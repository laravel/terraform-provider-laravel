package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// The data sources previously had no plan-level coverage at all: nothing drove
// a real terraform plan through their Read methods or checked the shape of what
// landed in state. They were also the one surface the example configs could not
// exercise, because every one of them needs a live API token.
//
// These tests run each data source against the in-memory fake, whose payloads
// are modelled on real captured responses.

func TestOrganizationDataSourcePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	runDataSourcePlan(t, baseURL, `data "laravel_cloud_organization" "current" {}`,
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue("data.laravel_cloud_organization.current",
				tfjsonpath.New("name"), knownvalue.StringExact("Acme")),
			statecheck.ExpectKnownValue("data.laravel_cloud_organization.current",
				tfjsonpath.New("slug"), knownvalue.StringExact("acme")),
		})
}

// TestRegionsDataSourcePlan also pins the field naming: the API property is
// "region", not "name", and reading the wrong one yields empty strings.
func TestRegionsDataSourcePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	runDataSourcePlan(t, baseURL, `data "laravel_cloud_regions" "all" {}`,
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue("data.laravel_cloud_regions.all",
				tfjsonpath.New("regions").AtSliceIndex(0).AtMapKey("name"),
				knownvalue.StringExact("us-east-2")),
			statecheck.ExpectKnownValue("data.laravel_cloud_regions.all",
				tfjsonpath.New("regions").AtSliceIndex(0).AtMapKey("label"),
				knownvalue.StringExact("Ohio")),
		})
}

// TestInstanceSizesDataSourcePlan covers the two things this data source got
// wrong: managed-queue sizes were dropped entirely, and their cpu_count is
// fractional, which an integer attribute cannot represent.
func TestInstanceSizesDataSourcePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	runDataSourcePlan(t, baseURL, `data "laravel_cloud_instance_sizes" "all" {}`,
		[]statecheck.StateCheck{
			// Both classes are surfaced, tagged so they can be told apart.
			statecheck.ExpectKnownValue("data.laravel_cloud_instance_sizes.all",
				tfjsonpath.New("sizes"), knownvalue.ListSizeExact(2)),
			statecheck.ExpectKnownValue("data.laravel_cloud_instance_sizes.all",
				tfjsonpath.New("sizes").AtSliceIndex(0).AtMapKey("instance_class"),
				knownvalue.StringExact("general")),
			statecheck.ExpectKnownValue("data.laravel_cloud_instance_sizes.all",
				tfjsonpath.New("sizes").AtSliceIndex(1).AtMapKey("instance_class"),
				knownvalue.StringExact("managed_queue")),
			statecheck.ExpectKnownValue("data.laravel_cloud_instance_sizes.all",
				tfjsonpath.New("sizes").AtSliceIndex(1).AtMapKey("cpu_count"),
				knownvalue.Float64Exact(0.0625)),
		})
}

// TestDatabaseTypesDataSourcePlan pins versions, which is what a caller needs
// to populate the required version argument on a cluster.
func TestDatabaseTypesDataSourcePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	runDataSourcePlan(t, baseURL, `data "laravel_cloud_database_types" "all" {}`,
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue("data.laravel_cloud_database_types.all",
				tfjsonpath.New("types").AtSliceIndex(0).AtMapKey("type"),
				knownvalue.StringExact("laravel_mysql")),
			statecheck.ExpectKnownValue("data.laravel_cloud_database_types.all",
				tfjsonpath.New("types").AtSliceIndex(0).AtMapKey("versions"),
				knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("8.4")})),
			// Sizes are mined out of config_schema.
			statecheck.ExpectKnownValue("data.laravel_cloud_database_types.all",
				tfjsonpath.New("types").AtSliceIndex(0).AtMapKey("sizes"),
				knownvalue.ListSizeExact(2)),
			// A retired identifier reports no versions, and an absent
			// config_schema must normalise to an empty list, not null.
			statecheck.ExpectKnownValue("data.laravel_cloud_database_types.all",
				tfjsonpath.New("types").AtSliceIndex(1).AtMapKey("sizes"),
				knownvalue.ListSizeExact(0)),
		})
}

func TestCacheTypesDataSourcePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	runDataSourcePlan(t, baseURL, `data "laravel_cloud_cache_types" "all" {}`,
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue("data.laravel_cloud_cache_types.all",
				tfjsonpath.New("types").AtSliceIndex(0).AtMapKey("type"),
				knownvalue.StringExact("laravel_valkey")),
			statecheck.ExpectKnownValue("data.laravel_cloud_cache_types.all",
				tfjsonpath.New("types").AtSliceIndex(0).AtMapKey("sizes").AtSliceIndex(0).AtMapKey("value"),
				knownvalue.StringExact("valkey-pro.250mb")),
		})
}

// TestDedicatedClustersDataSourcePlan covers the empty-collection case, which
// must produce an empty list rather than a null one. A nil Go slice decodes to
// a null Terraform list, and length()/for_each over null is a hard error, so
// the assertion below is the point of the test -- it passed `nil` checks
// before and could not fail.
func TestDedicatedClustersDataSourcePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	runDataSourcePlan(t, baseURL, `data "laravel_cloud_dedicated_clusters" "all" {}`,
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue("data.laravel_cloud_dedicated_clusters.all",
				tfjsonpath.New("clusters"), knownvalue.ListSizeExact(0)),
		})
}

func TestEdgeNetworksDataSourcePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	runDataSourcePlan(t, baseURL, `data "laravel_cloud_edge_networks" "all" {}`,
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue("data.laravel_cloud_edge_networks.all",
				tfjsonpath.New("edge_networks").AtSliceIndex(0).AtMapKey("domain"),
				knownvalue.StringExact("laravel.cloud")),
			statecheck.ExpectKnownValue("data.laravel_cloud_edge_networks.all",
				tfjsonpath.New("edge_networks").AtSliceIndex(0).AtMapKey("tenancy_type"),
				knownvalue.StringExact("shared")),
		})
}

// TestIPAddressesDataSourcePlan covers both shapes of the /ip route: the
// region-keyed object, and the flat form returned when region is passed. The
// addresses are flattened and sorted so the list does not churn between plans.
func TestIPAddressesDataSourcePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	runDataSourcePlan(t, baseURL, `data "laravel_cloud_ip_addresses" "all" {}`,
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue("data.laravel_cloud_ip_addresses.all",
				tfjsonpath.New("ip_addresses"), knownvalue.ListSizeExact(3)),
		})

	_, baseURL2 := newFakeCloud(t)
	runDataSourcePlan(t, baseURL2, `
data "laravel_cloud_ip_addresses" "one" {
  region = "us-east-1"
}`,
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue("data.laravel_cloud_ip_addresses.one",
				tfjsonpath.New("ip_addresses"), knownvalue.ListSizeExact(2)),
		})
}

// runDataSourcePlan applies a data-source-only config and then re-plans it,
// which must be empty -- a data source that churns between reads causes a
// perpetual diff in every configuration that depends on it.
func runDataSourcePlan(t *testing.T, baseURL, body string, checks []statecheck.StateCheck) {
	t.Helper()

	config := fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}
%[2]s
`, baseURL, body)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:            config,
				ConfigStateChecks: checks,
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}
