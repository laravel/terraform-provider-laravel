package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// testAccDedicatedEdgeNetwork returns the ID of the organization's first
// available dedicated edge network, the only kind that takes custom rules, and
// skips the test when there is none.
func testAccDedicatedEdgeNetwork(t *testing.T) string {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC must be set for acceptance tests")
	}
	testAccPreCheck(t)
	networks, err := testAccClient().ListEdgeNetworks(context.Background(), map[string]string{"status": "available"})
	if err != nil {
		t.Fatalf("listing edge networks: %v", err)
	}
	for _, n := range networks {
		if n.Attributes.TenancyType == "dedicated" {
			return n.ID
		}
	}
	t.Skip("the organization has no dedicated edge network")
	return ""
}

// testAccEdgeRuleConfig is two firewall rules and a rate limit, all of which
// match only a path no request uses, so they change nothing on the edge
// network's live sites. Rule b is placed before rule a, and the three are
// created at once, which the ruleset takes one at a time.
func testAccEdgeRuleConfig(edgeNetworkID, name, aExtra string, cActive bool) string {
	return fmt.Sprintf(`
locals {
  never = [{ criterias = [{ criteria = "value", field = "uri_path", operator = "equals", value = "/%[2]s-never" }] }]
}

resource "laravel_cloud_edge_network_rule" "a" {
  edge_network_id = %[1]q
  ruleset         = "firewall"
  name            = "%[2]s-a"
  settings        = jsonencode({ action = "log", criteria_groups = local.never })
  %[3]s
}

resource "laravel_cloud_edge_network_rule" "b" {
  edge_network_id = %[1]q
  ruleset         = "firewall"
  name            = "%[2]s-b"
  settings        = jsonencode({ action = "log", criteria_groups = local.never })
  position        = { before = laravel_cloud_edge_network_rule.a.id }
}

resource "laravel_cloud_edge_network_rule" "c" {
  edge_network_id = %[1]q
  ruleset         = "rate_limit"
  name            = "%[2]s-c"
  is_active       = %[4]t
  settings = jsonencode({
    requests_per_period = 10
    period              = 60
    action              = "block"
    action_duration     = 60
    throttles           = false
    criteria_groups     = local.never
  })
}
`, edgeNetworkID, name, aExtra, cActive)
}

// TestAccEdgeNetworkRule_basic creates rules on a dedicated edge network,
// places one before another, changes and moves one in place, and imports it.
func TestAccEdgeNetworkRule_basic(t *testing.T) {
	edgeNetworkID := testAccDedicatedEdgeNetwork(t)
	name := testAccName()
	const a, b, c = "laravel_cloud_edge_network_rule.a", "laravel_cloud_edge_network_rule.b", "laravel_cloud_edge_network_rule.c"

	order := func(addrs ...string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			rules, err := testAccClient().ListEdgeNetworkRules(context.Background(), edgeNetworkID, "firewall")
			if err != nil {
				return err
			}
			var ours []string
			want := map[string]bool{}
			for _, addr := range addrs {
				want[s.RootModule().Resources[addr].Primary.ID] = true
			}
			for _, r := range rules {
				if want[r.ID] {
					ours = append(ours, r.ID)
				}
			}
			for i, addr := range addrs {
				if i >= len(ours) || ours[i] != s.RootModule().Resources[addr].Primary.ID {
					return fmt.Errorf("firewall rules are in the order %v, want %v", ours, addrs)
				}
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			for _, rs := range s.RootModule().Resources {
				if rs.Type != "laravel_cloud_edge_network_rule" {
					continue
				}
				_, err := testAccClient().GetEdgeNetworkRule(context.Background(), edgeNetworkID,
					rs.Primary.Attributes["ruleset"], rs.Primary.ID)
				if !client.IsNotFound(err) {
					return fmt.Errorf("rule %s still exists (err %v)", rs.Primary.ID, err)
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccEdgeRuleConfig(edgeNetworkID, name, "", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					order(b, a),
					resource.TestCheckResourceAttr(a, "is_active", "true"),
					resource.TestCheckResourceAttr(c, "is_active", "false"),
				),
			},
			{
				Config: testAccEdgeRuleConfig(edgeNetworkID, name, `description = "tfacc"
  position = { index = 1 }`, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(a, plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction(b, plancheck.ResourceActionNoop),
					plancheck.ExpectResourceAction(c, plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					order(a, b),
					resource.TestCheckResourceAttr(a, "description", "tfacc"),
					resource.TestCheckResourceAttr(c, "is_active", "true"),
				),
			},
			{
				ResourceName: a,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return edgeNetworkID + ":firewall:" + s.RootModule().Resources[a].Primary.ID, nil
				},
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"position"},
			},
		},
	})
}
