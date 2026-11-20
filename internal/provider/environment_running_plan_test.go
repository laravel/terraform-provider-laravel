package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestEnvironmentRunningPlan: running stops and starts an
// environment in place. A started environment reports itself running only
// once the deployment that starts it succeeds, so the apply must wait for
// that, or the next plan would try to start it again.
func TestEnvironmentRunningPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	const env = "laravel_cloud_environment.env"
	config := func(running string) string {
		return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_application" "app" {
  name                         = "stack-app"
  repository                   = "laravel/laravel"
  region                       = "us-east-2"
  source_control_provider_type = "github"
}

resource "laravel_cloud_environment" "env" {
  application_id = laravel_cloud_application.app.id
  name           = "staging"
  branch         = "main"
  %s
}

# Only a deployed environment can be stopped.
resource "laravel_cloud_deployment" "deploy" {
  environment_id = laravel_cloud_environment.env.id
}
`, running)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(env, tfjsonpath.New("running"), knownvalue.Bool(true)),
				},
			},
			{
				Config: config("running = false"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(env, plancheck.ResourceActionUpdate),
					plancheck.ExpectUnknownValue(env, tfjsonpath.New("status")),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(env, tfjsonpath.New("running"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(env, tfjsonpath.New("status"), knownvalue.StringExact("stopped")),
				},
			},
			{
				Config:           config("running = false"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: config("running = true"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(env, tfjsonpath.New("running"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(env, tfjsonpath.New("status"), knownvalue.StringExact("running")),
				},
			},
			{
				// Leaving running out leaves the environment as it is.
				Config:           config(""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}
