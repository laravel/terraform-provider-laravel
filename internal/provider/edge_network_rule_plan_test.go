package provider

import (
	"fmt"
	"regexp"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// edgeRuleConfig is three rules: two firewall rules, the second placed
// before the first, and a rate limit that leaves counting_key to the API's
// default. aExtra is added to rule "a".
func edgeRuleConfig(baseURL, aExtra, aPath string, cActive bool) string {
	return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_edge_network_rule" "a" {
  edge_network_id = %[1]q
  ruleset         = "firewall"
  name            = "log probes"
  settings = jsonencode({
    action = "log"
    criteria_groups = [{ criterias = [{ criteria = "value", field = "uri_path", operator = "equals", value = %[2]q }] }]
  })
  %[3]s
}

resource "laravel_cloud_edge_network_rule" "b" {
  edge_network_id = %[1]q
  ruleset         = "firewall"
  name            = "block admin"
  settings = jsonencode({
    action = "block"
    criteria_groups = [{ criterias = [{ criteria = "value", field = "uri_path", operator = "equals", value = "/admin" }] }]
  })
  position = { before = laravel_cloud_edge_network_rule.a.id }
}

resource "laravel_cloud_edge_network_rule" "c" {
  edge_network_id = %[1]q
  ruleset         = "rate_limit"
  name            = "throttle login"
  is_active       = %[4]t
  settings = jsonencode({
    requests_per_period = 10
    period              = 60
    action              = "block"
    action_duration     = 60
    throttles           = false
    criteria_groups     = [{ criterias = [{ criteria = "value", field = "uri_path", operator = "equals", value = "/login" }] }]
  })
}
`, fakeEdgeNetworkID, aPath, aExtra, cActive)
}

// TestEdgeNetworkRulePlan creates rules while the ruleset is busy, places one
// before another, keeps the settings the configuration wrote although the API
// adds nulls and defaults, changes and moves a rule in place, switches one off,
// imports one, and destroys a rule that was already deleted outside Terraform.
func TestEdgeNetworkRulePlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	f.mu.Lock()
	f.edgeRules.busy = 3
	f.mu.Unlock()

	order := func(ruleset string, addrs ...string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			var want []string
			for _, a := range addrs {
				want = append(want, s.RootModule().Resources[a].Primary.ID)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if got := f.edgeRules.order[ruleset]; !slices.Equal(got, want) {
				return fmt.Errorf("%s order = %v, want %v", ruleset, got, want)
			}
			return nil
		}
	}
	const a, b, c = "laravel_cloud_edge_network_rule.a", "laravel_cloud_edge_network_rule.b", "laravel_cloud_edge_network_rule.c"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: edgeRuleConfig(baseURL, "", "/probe", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					order("firewall", b, a),
					resource.TestCheckResourceAttr(a, "is_active", "true"),
					resource.TestCheckResourceAttr(c, "is_active", "true"),
					resource.TestCheckNoResourceAttr(a, "description"),
					func(*terraform.State) error {
						f.mu.Lock()
						defer f.mu.Unlock()
						if f.edgeRules.busy != 0 {
							return fmt.Errorf("%d busy answers were never retried", f.edgeRules.busy)
						}
						return nil
					},
				),
			},
			{
				// Changing a rule in place and moving it first, and switching
				// another off.
				Config: edgeRuleConfig(baseURL, `description = "changed"
  position = { index = 1 }`, "/probe-2", false),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(a, plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction(b, plancheck.ResourceActionNoop),
					plancheck.ExpectResourceAction(c, plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					order("firewall", a, b),
					resource.TestCheckResourceAttr(a, "description", "changed"),
					resource.TestCheckResourceAttr(a, "settings",
						`{"action":"log","criteria_groups":[{"criterias":[{"criteria":"value","field":"uri_path","operator":"equals","value":"/probe-2"}]}]}`),
					resource.TestCheckResourceAttr(c, "is_active", "false"),
					func(*terraform.State) error {
						f.mu.Lock()
						defer f.mu.Unlock()
						for _, rule := range f.edgeRules.rules {
							if rule.ruleset == "rate_limit" && rule.isActive {
								return fmt.Errorf("the rate limit rule is active")
							}
						}
						return nil
					},
				),
			},
			{
				// The imported settings are the configuration's, without the
				// API's nulls.
				ResourceName: a,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return fakeEdgeNetworkID + ":firewall:" + s.RootModule().Resources[a].Primary.ID, nil
				},
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"position"},
			},
			{
				// Deleted outside Terraform: the next plan creates it again,
				// and the destroy at the end of the test, which does not
				// refresh, gets a 302 for it and must still succeed.
				PreConfig: func() {
					f.mu.Lock()
					defer f.mu.Unlock()
					for id, rule := range f.edgeRules.rules {
						if rule.name == "block admin" {
							delete(f.edgeRules.rules, id)
							f.edgeRules.order["firewall"] = slices.DeleteFunc(f.edgeRules.order["firewall"], func(o string) bool { return o == id })
						}
					}
				},
				Config: edgeRuleConfig(baseURL, `description = "changed"
  position = { index = 1 }`, "/probe-2", false),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestEdgeNetworkRuleValidationPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	rule := func(body string) string {
		return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_edge_network_rule" "r" {
  edge_network_id = %q
  ruleset         = "firewall"
  name            = "r"
  %s
}
`, fakeEdgeNetworkID, body)
	}
	const settings = `settings = jsonencode({ action = "log", criteria_groups = [] })`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      rule(`settings = "[]"`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`must be a JSON object`),
			},
			{
				Config:      rule(settings + "\n  position = { before = \"nru-1\", after = \"nru-2\" }"),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Combination`),
			},
			{
				Config:      rule(settings + "\n  position = {}"),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Combination`),
			},
		},
	})
}

// TestEdgeNetworkRuleCreateInactivePlan: Laravel Cloud creates every rule
// active, so a rule the apply would create, new or replacing another, cannot
// be inactive. The provider used to create it and then deactivate it, so the
// rule applied to traffic in between, and for good if the second request
// failed. The plan must fail before anything is created.
func TestEdgeNetworkRuleCreateInactivePlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	rule := func(ruleset string, active bool) string {
		return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_edge_network_rule" "r" {
  edge_network_id = %[1]q
  ruleset         = %[2]q
  name            = "log probes"
  is_active       = %[3]t
  settings = jsonencode({
    action = "log"
    criteria_groups = [{ criterias = [{ criteria = "value", field = "uri_path", operator = "equals", value = "/probe" }] }]
  })
}
`, fakeEdgeNetworkID, ruleset, active)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      rule("firewall", false),
				ExpectError: regexp.MustCompile(`A new edge network rule cannot start inactive`),
			},
			{
				PreConfig: func() {
					f.mu.Lock()
					defer f.mu.Unlock()
					if n := len(f.edgeRules.rules); n != 0 {
						t.Errorf("the refused plan still created %d rules", n)
					}
				},
				Config: rule("firewall", true),
			},
			{
				// A new ruleset replaces the rule, which creates a new one.
				Config:      rule("rate_limit", false),
				ExpectError: regexp.MustCompile(`A new edge network rule cannot start inactive`),
			},
			{
				// Switching an existing rule off is fine.
				Config: rule("firewall", false),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("laravel_cloud_edge_network_rule.r", plancheck.ResourceActionUpdate),
				}},
			},
		},
	})
}
