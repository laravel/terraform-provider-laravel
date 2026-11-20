package provider

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/laravel/terraform-provider-laravel/internal/client"
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
				tfjsonpath.New("types").AtSliceIndex(0).AtMapKey("supports_hibernation"),
				knownvalue.Bool(true)),
			statecheck.ExpectKnownValue("data.laravel_cloud_cache_types.all",
				tfjsonpath.New("types").AtSliceIndex(1).AtMapKey("supports_eviction_policy"),
				knownvalue.Bool(false)),
			// Only some sizes of a type that can scale to zero accept it.
			statecheck.ExpectKnownValue("data.laravel_cloud_cache_types.all",
				tfjsonpath.New("types").AtSliceIndex(0).AtMapKey("sizes"),
				knownvalue.ListExact([]knownvalue.Check{
					knownvalue.ObjectExact(map[string]knownvalue.Check{
						"value": knownvalue.StringExact("valkey-flex-250mb"), "label": knownvalue.StringExact("250 MB"),
						"supports_hibernation": knownvalue.Bool(true),
					}),
					knownvalue.ObjectExact(map[string]knownvalue.Check{
						"value": knownvalue.StringExact("valkey-pro.250mb"), "label": knownvalue.StringExact("250 MB"),
						"supports_hibernation": knownvalue.Bool(false),
					}),
				})),
		})
}

// TestDedicatedClustersDataSourcePlan covers the filters, which the API
// matches as substrings, and the empty-collection case, which must produce an
// empty list rather than a null one. A nil Go slice decodes to a null
// Terraform list, and length()/for_each over null is a hard error.
func TestDedicatedClustersDataSourcePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	const ds = "data.laravel_cloud_dedicated_clusters"
	runDataSourcePlan(t, baseURL, `
data "laravel_cloud_dedicated_clusters" "all" {}

data "laravel_cloud_dedicated_clusters" "databases" {
  region = "us-east-2"
  type   = "rds"
  status = "active"
}

data "laravel_cloud_dedicated_clusters" "none" {
  region = "eu-west-1"
}
`,
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(ds+".all", tfjsonpath.New("clusters"), knownvalue.ListSizeExact(2)),
			statecheck.ExpectKnownValue(ds+".databases", tfjsonpath.New("clusters"), knownvalue.ListSizeExact(1)),
			statecheck.ExpectKnownValue(ds+".databases",
				tfjsonpath.New("clusters").AtSliceIndex(0).AtMapKey("type"), knownvalue.StringExact("rds-databases")),
			statecheck.ExpectKnownValue(ds+".none", tfjsonpath.New("clusters"), knownvalue.ListSizeExact(0)),
		})
}

func TestEdgeNetworksDataSourcePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	const ds = "data.laravel_cloud_edge_networks"
	runDataSourcePlan(t, baseURL, `
data "laravel_cloud_edge_networks" "all" {}

data "laravel_cloud_edge_networks" "demo" {
  domain = "acme-demo"
  status = "available"
}

data "laravel_cloud_edge_networks" "none" {
  name = "nothing-matches"
}
`,
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(ds+".all", tfjsonpath.New("edge_networks"), knownvalue.ListSizeExact(2)),
			statecheck.ExpectKnownValue(ds+".all",
				tfjsonpath.New("edge_networks").AtSliceIndex(0).AtMapKey("tenancy_type"), knownvalue.StringExact("shared")),
			statecheck.ExpectKnownValue(ds+".demo", tfjsonpath.New("edge_networks"), knownvalue.ListSizeExact(1)),
			statecheck.ExpectKnownValue(ds+".demo",
				tfjsonpath.New("edge_networks").AtSliceIndex(0).AtMapKey("id"), knownvalue.StringExact("edgenet-0002")),
			statecheck.ExpectKnownValue(ds+".none", tfjsonpath.New("edge_networks"), knownvalue.ListSizeExact(0)),
		})
}

func TestWebsocketServerTypesDataSourcePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	const ds = "data.laravel_cloud_websocket_server_types.all"
	runDataSourcePlan(t, baseURL, `data "laravel_cloud_websocket_server_types" "all" {}`,
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(ds, tfjsonpath.New("types").AtSliceIndex(0).AtMapKey("type"), knownvalue.StringExact("reverb")),
			statecheck.ExpectKnownValue(ds, tfjsonpath.New("types").AtSliceIndex(0).AtMapKey("regions"),
				knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("us-east-1"), knownvalue.StringExact("us-east-2")})),
			// A tier the plan does not include is listed, marked unavailable.
			statecheck.ExpectKnownValue(ds, tfjsonpath.New("types").AtSliceIndex(0).AtMapKey("max_connections").AtSliceIndex(2),
				knownvalue.ObjectExact(map[string]knownvalue.Check{
					"value": knownvalue.Int64Exact(10000), "label": knownvalue.StringExact("10,000"),
					"available": knownvalue.Bool(false),
				})),
		})
}

// TestDatabasePresetsDataSourcePlan checks that type and region reach the
// API and that each preset's config is kept as the JSON the API returned.
func TestDatabasePresetsDataSourcePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	const ds = "data.laravel_cloud_database_presets.mysql"
	runDataSourcePlan(t, baseURL, `
data "laravel_cloud_database_presets" "mysql" {
  type   = "laravel_mysql"
  region = "us-east-2"
}
`,
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(ds, tfjsonpath.New("presets"), knownvalue.ListSizeExact(2)),
			statecheck.ExpectKnownValue(ds, tfjsonpath.New("presets").AtSliceIndex(1).AtMapKey("key"), knownvalue.StringExact("prod")),
			statecheck.ExpectKnownValue(ds, tfjsonpath.New("presets").AtSliceIndex(1).AtMapKey("config"),
				knownvalue.StringFunc(func(v string) error {
					var config map[string]any
					if err := json.Unmarshal([]byte(v), &config); err != nil {
						return err
					}
					if config["size"] != "mysql-flex-2gb" {
						return fmt.Errorf("config %s does not have size mysql-flex-2gb", v)
					}
					return nil
				})),
		})
}

func TestResendDomainsDataSourcePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	const ds = "data.laravel_cloud_resend_domains.all"
	records := tfjsonpath.New("domains").AtSliceIndex(0).AtMapKey("records")
	runDataSourcePlan(t, baseURL, `data "laravel_cloud_resend_domains" "all" {}`,
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(ds, tfjsonpath.New("domains").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact("mail.example.com")),
			statecheck.ExpectKnownValue(ds, tfjsonpath.New("domains").AtSliceIndex(0).AtMapKey("status"), knownvalue.StringExact("verified")),
			// Only MX records carry a priority.
			statecheck.ExpectKnownValue(ds, records.AtSliceIndex(0).AtMapKey("priority"), knownvalue.Null()),
			statecheck.ExpectKnownValue(ds, records.AtSliceIndex(1).AtMapKey("priority"), knownvalue.Int64Exact(10)),
		})
}

// TestResendReusableKeysDataSourcePlan lists the keys of the other
// environments, once each, and never the environment's own.
func TestResendReusableKeysDataSourcePlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	for id, key := range map[string]string{"env-a": "shared", "env-b": "shared", "env-c": "own", "env-d": ""} {
		env := &client.EnvironmentData{ID: id, Type: "environments"}
		if key != "" {
			env.Attributes.Resend = &client.EnvironmentResend{KeyName: key}
		}
		f.envs[id] = env
	}

	const ds = "data.laravel_cloud_resend_reusable_keys.c"
	runDataSourcePlan(t, baseURL, `
data "laravel_cloud_resend_reusable_keys" "c" {
  environment_id = "env-c"
}`,
		[]statecheck.StateCheck{
			statecheck.ExpectKnownValue(ds, tfjsonpath.New("keys"), knownvalue.ListExact([]knownvalue.Check{
				knownvalue.ObjectExact(map[string]knownvalue.Check{
					"id": knownvalue.StringExact("rkey-shared"), "name": knownvalue.StringExact("shared"),
					"environment_count": knownvalue.Int64Exact(2),
				}),
			})),
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
