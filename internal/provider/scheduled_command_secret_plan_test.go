package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func scheduledCommandConfig(baseURL, nameLine, schedule string) string {
	return providerCfg(baseURL) + appEnvStack + instanceStack + fmt.Sprintf(`
resource "laravel_cloud_scheduled_command" "sched" {
  instance_id = laravel_cloud_instance.instance.id
  %s
  command     = "php artisan reports:send"
  schedule    = %q
}
`, nameLine, schedule)
}

// TestScheduledCommandPlan creates a scheduled command, re-plans it empty,
// changes its schedule in place, clears its name (which must reach the API as
// null, not be omitted) and imports it.
func TestScheduledCommandPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_scheduled_command.sched"
	named := scheduledCommandConfig(baseURL, `name = "Nightly reports"`, "0 3 * * *")
	rescheduled := scheduledCommandConfig(baseURL, `name = "Nightly reports"`, "@hourly")
	unnamed := scheduledCommandConfig(baseURL, ``, "@hourly")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:           named,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{create(addr)}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("name"), knownvalue.StringExact("Nightly reports")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("schedule"), knownvalue.StringExact("0 3 * * *")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("created_at"), knownvalue.NotNull()),
				},
			},
			{
				Config:           named,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: rescheduled,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("schedule"), knownvalue.StringExact("@hourly")),
				},
			},
			{
				Config: unnamed,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("name"), knownvalue.Null()),
				},
				Check: func(*terraform.State) error {
					f.mu.Lock()
					defer f.mu.Unlock()
					patches := f.patches["scheduled_command"]
					last := patches[len(patches)-1]
					if v, ok := last["name"]; !ok || v != nil {
						return fmt.Errorf("clearing name must send name: null, sent %v", last)
					}
					return nil
				},
			},
			{
				Config:           unnamed,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func secretConfig(baseURL, value, notesLine string) string {
	return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_secret" "s" {
  key   = "STRIPE_SECRET"
  value = %q
  %s
}
`, value, notesLine)
}

// TestSecretPlan drives a secret through create, an empty re-plan, a value
// rotation and clearing its notes. The fake opens every submitted value with
// its private key and rejects one it cannot open, as the API does, so these
// steps only pass if the provider sealed the value to the published key.
func TestSecretPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_secret.s"

	storedValue := func(want string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			id := s.RootModule().Resources[addr].Primary.ID
			f.mu.Lock()
			defer f.mu.Unlock()
			if got := f.secrets.values[id]; got != want {
				return fmt.Errorf("the API opened %q, want %q", got, want)
			}
			return nil
		}
	}

	initial := secretConfig(baseURL, "sk_live_one", `notes = "Stripe live key"`)
	rotated := secretConfig(baseURL, "sk_live_two", `notes = "Stripe live key"`)
	noNotes := secretConfig(baseURL, "sk_live_two", ``)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:           initial,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{create(addr)}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("key"), knownvalue.StringExact("STRIPE_SECRET")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("notes"), knownvalue.StringExact("Stripe live key")),
				},
				Check: storedValue("sk_live_one"),
			},
			{
				Config:           initial,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: rotated,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
				Check: storedValue("sk_live_two"),
			},
			{
				Config: noNotes,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("notes"), knownvalue.Null()),
				},
				Check: func(*terraform.State) error {
					f.mu.Lock()
					defer f.mu.Unlock()
					patches := f.patches["secret"]
					last := patches[len(patches)-1]
					if v, ok := last["notes"]; !ok || v != nil {
						return fmt.Errorf("clearing notes must send notes: null, sent %v", last)
					}
					// The value did not change, so it must not be re-sealed and re-sent.
					if _, ok := last["value"]; ok {
						return fmt.Errorf("an unchanged value was re-sent: %v", last)
					}
					return nil
				},
			},
			{
				Config:           noNotes,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				// The API never returns the value, so an import cannot recover it.
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"value"},
			},
		},
	})
}
