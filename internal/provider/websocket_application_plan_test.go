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

// TestWebsocketApplicationCredentialsSurviveReadPlan guards against silent
// credential loss.
//
// key and secret are merged into the response only when the application is
// created; every later GET omits them. The mapper wrote both unconditionally,
// so the first refresh after create replaced the stored credentials with empty
// strings. Both attributes are Computed, so there was no apply error and no
// diff -- anything referencing .secret simply started getting "".
func TestWebsocketApplicationCredentialsSurviveReadPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := websocketAppConfig(baseURL, `allowed_origins = ["https://example.com"]`)

	const addr = "laravel_cloud_websocket_application.app"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("key"),
						knownvalue.StringExact("ws-key-0001")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("secret"),
						knownvalue.StringExact("ws-secret-0001")),
				},
			},
			// This step refreshes before planning, which is where the
			// credentials used to be wiped.
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("key"),
						knownvalue.StringExact("ws-key-0001")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("secret"),
						knownvalue.StringExact("ws-secret-0001")),
				},
			},
		},
	})
}

// TestWebsocketApplicationOmittedOriginsPlan covers a config that does not set
// allowed_origins at all. The API always returns a list there -- empty when
// nothing is restricted -- so with the attribute Optional but not Computed,
// Terraform saw config null against state [] and failed the apply with
// "provider produced inconsistent result after apply".
func TestWebsocketApplicationOmittedOriginsPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := websocketAppConfig(baseURL, "")

	const addr = "laravel_cloud_websocket_application.app"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("allowed_origins"),
						knownvalue.ListSizeExact(0)),
				},
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestWebsocketApplicationClearOriginsPlan covers narrowing the origin list to
// nothing. `[]string` plus omitempty dropped an empty list from the PATCH body
// entirely, so the API kept the previous origins and the restriction could
// never be removed.
func TestWebsocketApplicationClearOriginsPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	const addr = "laravel_cloud_websocket_application.app"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: websocketAppConfig(baseURL, `allowed_origins = ["https://example.com"]`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("allowed_origins"),
						knownvalue.ListSizeExact(1)),
				},
			},
			{
				Config: websocketAppConfig(baseURL, `allowed_origins = []`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("allowed_origins"),
						knownvalue.ListSizeExact(0)),
				},
			},
			{
				Config: websocketAppConfig(baseURL, `allowed_origins = []`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func websocketAppConfig(baseURL, extra string) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_websocket_server" "server" {
  name            = "reverb"
  type            = "reverb"
  region          = "us-east-1"
  max_connections = 1000
}

resource "laravel_cloud_websocket_application" "app" {
  server_id = laravel_cloud_websocket_server.server.id
  name      = "chat"
  %[2]s
}
`, baseURL, extra)
}
