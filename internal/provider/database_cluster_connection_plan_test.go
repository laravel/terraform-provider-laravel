package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// The tests below guard a defect reported against an imported database
// cluster: the first plan, with nothing changed, printed "this attribute value
// will no longer be marked as sensitive after applying this change" above the
// connection password.
//
// connection_details is Computed with no plan modifier, so any in-place update
// -- and an import always plans one, for version and force_destroy, which the
// API never reports -- planned the whole object as unknown. An unknown object
// cannot carry the sensitive mark on its nested password, which is what
// Terraform warns about.

const clusterAddr = "laravel_cloud_database_cluster.main"

var clusterPasswordPath = tfjsonpath.New("connection_details").AtMapKey("password")

func connectionCluster(baseURL, storage, extra string) string {
	return providerCfg(baseURL) + `
resource "laravel_cloud_database_cluster" "main" {
  name    = "conn-db"
  type    = "laravel_mysql"
  version = "8.4"
  region  = "us-east-2"
  ` + extra + `

  config = jsonencode({
    size                     = "mysql-flex-512mb"
    storage                  = ` + storage + `
    is_public                = false
    uses_scheduled_snapshots = false
    retention_days           = 7
    suspend_seconds          = 0
  })
}
`
}

func TestDatabaseClusterImportKeepsPasswordSensitivePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := connectionCluster(baseURL, "10", "force_destroy = true")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				Config:          config,
				ResourceName:    clusterAddr,
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				// version and force_destroy are never reported, so the import
				// still plans recording them.
				ExpectNonEmptyPlan: true,
				ImportPlanChecks: resource.ImportPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectSensitiveValue(clusterAddr, clusterPasswordPath),
					},
				},
			},
		},
	})
}

// An update that leaves config alone cannot change the connection, so the
// recorded details are kept rather than planned as unknown.
func TestDatabaseClusterConnectionKeptWithoutConfigChangePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: connectionCluster(baseURL, "10", "")},
			{
				Config: connectionCluster(baseURL, "10", "force_destroy = true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(clusterAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(clusterAddr,
							tfjsonpath.New("connection_details").AtMapKey("hostname"),
							knownvalue.StringExact("db.test.local")),
						plancheck.ExpectSensitiveValue(clusterAddr, clusterPasswordPath),
					},
				},
			},
		},
	})
}

// A config change may move the connection (is_public changes the hostname), so
// the details are unknown -- but the password must stay marked sensitive, and
// apply must accept whatever the API reports.
func TestDatabaseClusterConfigChangeKeepsPasswordSensitivePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: connectionCluster(baseURL, "10", "force_destroy = true")},
			{
				Config: connectionCluster(baseURL, "20", "force_destroy = true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(clusterAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectUnknownValue(clusterAddr,
							tfjsonpath.New("connection_details").AtMapKey("hostname")),
						plancheck.ExpectSensitiveValue(clusterAddr, clusterPasswordPath),
					},
				},
			},
		},
	})
}

// An import records the API's full effective config, defaults the
// configuration never sets included, so the first plan used to propose
// removing them -- and applying it sent a config PATCH to a cluster nothing had
// changed on. This models the reported flow: a cluster that exists only on the
// platform, imported, then planned against a configuration that sets a subset
// of its config.
func TestDatabaseClusterImportPlansNoConfigChangePlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	config := connectionCluster(baseURL, "10", "force_destroy = true")
	clusterPatches := func() []map[string]any {
		f.mu.Lock()
		defer f.mu.Unlock()
		return append([]map[string]any(nil), f.patches[clusterType]...)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					var cfg map[string]any
					_ = json.Unmarshal([]byte(`{"size":"mysql-flex-512mb","storage":10,"is_public":false,"uses_scheduled_snapshots":false,"retention_days":7,"suspend_seconds":0}`), &cfg)
					c := client.NewClient("test-token", client.WithBaseURL(baseURL))
					if _, err := c.CreateDatabaseCluster(context.Background(), client.CreateDatabaseClusterRequest{
						Type: "laravel_mysql", Version: "8.4", Name: "conn-db", Region: "us-east-2", Config: cfg,
					}); err != nil {
						t.Fatalf("seeding cluster: %v", err)
					}
				},
				Config:             config,
				ResourceName:       clusterAddr,
				ImportState:        true,
				ImportStateId:      "cluster-1",
				ImportStatePersist: true,
			},
			{
				// Only version and force_destroy, which the API never reports,
				// are recorded; nothing else moves.
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(clusterAddr, plancheck.ResourceActionUpdate),
						expectAttributeUnchanged{clusterAddr, "config"},
						expectAttributeUnchanged{clusterAddr, "connection_details"},
						expectAttributeUnchanged{clusterAddr, "status"},
						expectAttributeUnchanged{clusterAddr, "created_at"},
					},
				},
				Check: func(*terraform.State) error {
					if p := clusterPatches(); len(p) != 0 {
						return fmt.Errorf("applying the import sent %d cluster PATCH(es): %v", len(p), p)
					}
					return nil
				},
			},
			{
				// A real config change afterwards still reaches the API.
				Config: connectionCluster(baseURL, "20", "force_destroy = true"),
				Check: func(*terraform.State) error {
					p := clusterPatches()
					if len(p) != 1 {
						return fmt.Errorf("want 1 cluster PATCH for the storage change, got %d: %v", len(p), p)
					}
					if cfg, _ := p[0]["config"].(map[string]any); cfg["storage"] != float64(20) {
						return fmt.Errorf("PATCH did not carry storage 20: %v", p[0])
					}
					return nil
				},
			},
		},
	})
}

// config is compared by content, so the same settings written in a different
// key order or layout plan nothing.
func TestDatabaseClusterConfigReorderedPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	reordered := providerCfg(baseURL) + `
resource "laravel_cloud_database_cluster" "main" {
  name          = "conn-db"
  type          = "laravel_mysql"
  version       = "8.4"
  region        = "us-east-2"
  force_destroy = true

  config = "{\"suspend_seconds\": 0, \"retention_days\": 7, \"uses_scheduled_snapshots\": false, \"is_public\": false, \"storage\": 10, \"size\": \"mysql-flex-512mb\"}"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: connectionCluster(baseURL, "10", "force_destroy = true")},
			{Config: reordered, PlanOnly: true},
		},
	})
}

// expectAttributeUnchanged fails when the plan changes the named top-level
// attribute of a resource, including planning it unknown.
type expectAttributeUnchanged struct {
	addr, attr string
}

func (c expectAttributeUnchanged) CheckPlan(_ context.Context, req plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
	for _, rc := range req.Plan.ResourceChanges {
		if rc.Address != c.addr || rc.Change == nil {
			continue
		}
		before, _ := rc.Change.Before.(map[string]any)
		after, _ := rc.Change.After.(map[string]any)
		unknown, _ := rc.Change.AfterUnknown.(map[string]any)
		if u, ok := unknown[c.attr]; ok && u != false {
			resp.Error = fmt.Errorf("%s.%s is planned unknown", c.addr, c.attr)
			return
		}
		if !reflect.DeepEqual(before[c.attr], after[c.attr]) {
			resp.Error = fmt.Errorf("%s.%s changes: %v -> %v", c.addr, c.attr, before[c.attr], after[c.attr])
		}
		return
	}
	resp.Error = fmt.Errorf("%s is not in the plan", c.addr)
}
