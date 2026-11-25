package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

func internalNetworkingConfig(baseURL, access string) string {
	return providerCfg(baseURL) + appEnvStack + fmt.Sprintf(`
resource "laravel_cloud_environment" "api" {
  application_id = laravel_cloud_application.app.id
  name           = "api"
  branch         = "main"
}

resource "laravel_cloud_environment" "worker" {
  application_id = laravel_cloud_application.app.id
  name           = "worker"
  branch         = "main"
}

resource "laravel_cloud_environment_internal_networking" "env" {
  environment_id = laravel_cloud_environment.env.id
  %s
}
`, access)
}

// TestEnvironmentInternalNetworkingPlan turns internal networking on for all
// environments, narrows it to specific ones in place, imports it, notices it
// being turned off outside Terraform, and turns it off on destroy.
func TestEnvironmentInternalNetworkingPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	const addr = "laravel_cloud_environment_internal_networking.env"

	networking := func(want *client.EnvironmentInternalNetworking) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			id := s.RootModule().Resources[addr].Primary.ID
			f.mu.Lock()
			defer f.mu.Unlock()
			got := f.envs[id].Attributes.InternalNetworking
			if got == nil || got.Enabled != want.Enabled || *got.AccessLevel != *want.AccessLevel || len(got.EnvironmentIDs) != len(want.EnvironmentIDs) {
				return fmt.Errorf("internal networking of %s = %+v, want %+v", id, got, want)
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: internalNetworkingConfig(baseURL, `access_level = "all"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					networking(&client.EnvironmentInternalNetworking{Enabled: true, AccessLevel: ptr("all")}),
					resource.TestCheckResourceAttrPair(addr, "id", "laravel_cloud_environment.env", "id"),
					resource.TestMatchResourceAttr(addr, "hostname", regexp.MustCompile(`\.private\.laravel\.cloud$`)),
					resource.TestCheckNoResourceAttr(addr, "environment_ids"),
				),
			},
			{
				Config: internalNetworkingConfig(baseURL, `access_level = "specific"
  environment_ids = [laravel_cloud_environment.api.id, laravel_cloud_environment.worker.id]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					networking(&client.EnvironmentInternalNetworking{Enabled: true, AccessLevel: ptr("specific"), EnvironmentIDs: []string{"a", "b"}}),
					resource.TestCheckResourceAttr(addr, "environment_ids.#", "2"),
					func(*terraform.State) error {
						f.mu.Lock()
						defer f.mu.Unlock()
						if f.internalNetworkingWrites["PATCH"] != 1 {
							return fmt.Errorf("PATCH sent %d times, want 1", f.internalNetworkingWrites["PATCH"])
						}
						return nil
					},
				),
			},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Turned off outside Terraform: the next plan turns it on.
				PreConfig: func() {
					f.mu.Lock()
					defer f.mu.Unlock()
					for _, env := range f.envs {
						if env.Attributes.InternalNetworking != nil {
							env.Attributes.InternalNetworking = &client.EnvironmentInternalNetworking{EnvironmentIDs: []string{}}
						}
					}
				},
				Config: internalNetworkingConfig(baseURL, `access_level = "specific"
  environment_ids = [laravel_cloud_environment.api.id, laravel_cloud_environment.worker.id]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{create(addr)}},
				Check:            networking(&client.EnvironmentInternalNetworking{Enabled: true, AccessLevel: ptr("specific"), EnvironmentIDs: []string{"a", "b"}}),
			},
			{
				// Removing the resource turns internal networking off.
				Config: providerCfg(baseURL) + appEnvStack,
				Check: func(*terraform.State) error {
					f.mu.Lock()
					defer f.mu.Unlock()
					for id, env := range f.envs {
						if env.Attributes.InternalNetworking != nil && env.Attributes.InternalNetworking.Enabled {
							return fmt.Errorf("internal networking is still on for %s", id)
						}
					}
					return nil
				},
			},
		},
	})
}

func TestEnvironmentInternalNetworkingValidationPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      internalNetworkingConfig(baseURL, `access_level = "specific"`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Missing environment_ids`),
			},
			{
				Config: internalNetworkingConfig(baseURL, `access_level = "all"
  environment_ids = [laravel_cloud_environment.api.id]`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Unexpected environment_ids`),
			},
		},
	})
}
