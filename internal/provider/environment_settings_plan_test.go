package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

const testNightwatchToken = "nw_0123456789abcdef0123456789abcdef012345678"

func environmentSettingsConfig(baseURL, settings string) string {
	return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_application" "app" {
  name                         = "settings-app"
  repository                   = "laravel/laravel"
  region                       = "us-east-2"
  source_control_provider_type = "github"
}

resource "laravel_cloud_environment" "env" {
  application_id  = laravel_cloud_application.app.id
  name            = "production"
  branch          = "main"
  edge_network_id = "edgenet-1"
%s
}
`, settings)
}

// lastEnvUpdate returns the most recent PATCH /environments body as sent, so a
// test can tell an omitted field from an explicit null.
func lastEnvUpdate(f *fakeCloud) (map[string]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.envUpdateBodies) == 0 {
		return nil, fmt.Errorf("no environment update was sent")
	}
	return f.envUpdateBodies[len(f.envUpdateBodies)-1], nil
}

// TestEnvironmentSettingsPlan drives the newer environment settings through
// create, an empty re-plan, clearing each clearable one, a change made in the
// dashboard, and an import.
func TestEnvironmentSettingsPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_environment.env"

	initial := environmentSettingsConfig(baseURL, fmt.Sprintf(`
  hibernation_wake_up_interval             = 60
  nightwatch_token                         = %q
  response_headers_frame                   = "sameorigin"
  response_headers_robots_tag              = "noindex, nofollow"
  response_headers_hsts_max_age            = 31536000
  response_headers_hsts_include_subdomains = true
  firewall_block_path                      = false

  filesystem_keys = [{
    key_id          = "key-1"
    disk            = "s3"
    is_default_disk = true
  }]
`, testNightwatchToken))

	// Every clearable setting cleared; the rest left to the platform.
	cleared := environmentSettingsConfig(baseURL, `
  hibernation_wake_up_interval             = 0
  response_headers_hsts_max_age            = 0
  response_headers_hsts_include_subdomains = false
`)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: initial,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("hibernation_wake_up_interval"), knownvalue.Int64Exact(60)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("response_headers_frame"), knownvalue.StringExact("sameorigin")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("response_headers_robots_tag"), knownvalue.StringExact("noindex, nofollow")),
					// Left unset: the platform default is adopted, not reset.
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("response_headers_content_type"), knownvalue.StringExact("nosniff")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("response_headers_hsts_preload"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("firewall_block_path"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("firewall_browser_integrity_check"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("serving_mode"), knownvalue.StringExact("process")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("serving_root"), knownvalue.Null()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("start_command"), knownvalue.Null()),
				},
				Check: func(*terraform.State) error {
					body, err := lastEnvUpdate(f)
					if err != nil {
						return err
					}
					// HSTS goes out whole, with the unset preload filled from the API.
					hsts, _ := body["response_headers_hsts"].(map[string]any)
					if hsts == nil || hsts["max_age"] != float64(31536000) || hsts["include_subdomains"] != true || hsts["preload"] != false {
						return fmt.Errorf("response_headers_hsts = %v", body["response_headers_hsts"])
					}
					if body["nightwatch_token"] != testNightwatchToken {
						return fmt.Errorf("nightwatch_token = %v", body["nightwatch_token"])
					}
					keys, _ := body["filesystem_keys"].([]any)
					if len(keys) != 1 {
						return fmt.Errorf("filesystem_keys = %v", body["filesystem_keys"])
					}
					// Settings the configuration left alone must not be sent.
					for _, k := range []string{"response_headers_content_type", "firewall_browser_integrity_check", "start_command", "serving"} {
						if _, ok := body[k]; ok {
							return fmt.Errorf("%s was sent although it is not configured: %v", k, body[k])
						}
					}
					return nil
				},
			},
			{
				Config:           initial,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: cleared,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("hibernation_wake_up_interval"), knownvalue.Int64Exact(0)),
					// Removed from config, but readable: the last value stays.
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("response_headers_frame"), knownvalue.StringExact("sameorigin")),
				},
				Check: func(*terraform.State) error {
					body, err := lastEnvUpdate(f)
					if err != nil {
						return err
					}
					for _, k := range []string{"hibernation_wake_up_interval", "nightwatch_token"} {
						if v, ok := body[k]; !ok || v != nil {
							return fmt.Errorf("clearing %s must send null, sent %v", k, body)
						}
					}
					if keys, ok := body["filesystem_keys"].([]any); !ok || len(keys) != 0 {
						return fmt.Errorf("removing filesystem_keys must send [], sent %v", body["filesystem_keys"])
					}
					hsts, _ := body["response_headers_hsts"].(map[string]any)
					if hsts == nil || hsts["max_age"] != float64(0) || hsts["include_subdomains"] != false {
						return fmt.Errorf("response_headers_hsts = %v", body["response_headers_hsts"])
					}
					return nil
				},
			},
			{
				Config:           cleared,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				// A header changed in the dashboard is adopted on refresh, not
				// reset, because the configuration does not set it.
				PreConfig: func() {
					f.mu.Lock()
					defer f.mu.Unlock()
					for _, env := range f.envs {
						env.Attributes.NetworkSettings.ResponseHeaders.ContentType = ptr("none")
					}
				},
				Config:           cleared,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("response_headers_content_type"), knownvalue.StringExact("none")),
				},
			},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
				// Never reported by the API, so an import cannot recover them;
				// the wake-up interval of 0 reads back as null (off).
				ImportStateVerifyIgnore: []string{"edge_network_id", "php_version", "hibernation_wake_up_interval"},
			},
		},
	})
}

// TestEnvironmentOmittedSettingsPlan covers settings the published response
// schema leaves out (robots_tag, browser_integrity_check). If the API does
// not report them, a refresh must keep the configured value rather than read
// the absence as null and plan to send them again forever.
func TestEnvironmentOmittedSettingsPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_environment.env"
	config := environmentSettingsConfig(baseURL, `
  response_headers_robots_tag      = "noindex, nofollow"
  firewall_browser_integrity_check = false
`)
	omit := func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, env := range f.envs {
			env.Attributes.NetworkSettings.ResponseHeaders.RobotsTag = nil
			env.Attributes.NetworkSettings.Firewall.BrowserIntegrityCheck = nil
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig:        omit,
				Config:           config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("response_headers_robots_tag"), knownvalue.StringExact("noindex, nofollow")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("firewall_browser_integrity_check"), knownvalue.Bool(false)),
				},
			},
		},
	})
}

// TestEnvironmentUnsentWriteOnlyPlan covers a create whose follow-up update
// fails. The API never reports nightwatch_token or filesystem_keys, so unless
// the provider remembers they were not sent, state claims them and no later
// plan would ever send them.
func TestEnvironmentUnsentWriteOnlyPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_environment.env"
	config := environmentSettingsConfig(baseURL, fmt.Sprintf(`
  nightwatch_token = %q
  filesystem_keys  = [{ key_id = "key-1", disk = "s3" }]
`, testNightwatchToken))

	f.mu.Lock()
	f.failEnvUpdates = 1
	f.mu.Unlock()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// The environment is created; the settings update fails and
				// is reported as a warning, and the refresh after it already
				// plans to send the two settings again.
				Config:             config,
				ExpectNonEmptyPlan: true,
				Check: func(*terraform.State) error {
					f.mu.Lock()
					defer f.mu.Unlock()
					if len(f.envUpdateBodies) != 0 {
						return fmt.Errorf("expected the follow-up update to fail, got %v", f.envUpdateBodies)
					}
					return nil
				},
			},
			{
				// The next plan must send both settings.
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
				Check: func(*terraform.State) error {
					body, err := lastEnvUpdate(f)
					if err != nil {
						return err
					}
					if body["nightwatch_token"] != testNightwatchToken {
						return fmt.Errorf("nightwatch_token was not resent: %v", body)
					}
					if keys, _ := body["filesystem_keys"].([]any); len(keys) != 1 {
						return fmt.Errorf("filesystem_keys were not resent: %v", body)
					}
					return nil
				},
			},
			{
				// Once applied, the marker is gone and nothing is planned.
				Config:           config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// TestEnvironmentWebsocketApplicationPlan attaches a WebSocket application to
// an environment and detaches it again by removing the attribute, which must
// send null rather than omit the field.
func TestEnvironmentWebsocketApplicationPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	stack := providerCfg(baseURL) + appEnvStack + serverStack + `
resource "laravel_cloud_websocket_application" "wsapp" {
  server_id = laravel_cloud_websocket_server.server.id
  name      = "chat"
}
`
	// appEnvStack's environment, with the attachment swapped in.
	attached := strings.Replace(stack, `  branch         = "main"
}`, `  branch         = "main"
  websocket_application_id = laravel_cloud_websocket_application.wsapp.id
}`, 1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: attached,
				Check: func(s *terraform.State) error {
					body, err := lastEnvUpdate(f)
					if err != nil {
						return err
					}
					want := s.RootModule().Resources["laravel_cloud_websocket_application.wsapp"].Primary.ID
					if body["websocket_application_id"] != want {
						return fmt.Errorf("websocket_application_id = %v, want %s", body["websocket_application_id"], want)
					}
					return nil
				},
			},
			{
				Config:           attached,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: stack,
				Check: func(*terraform.State) error {
					body, err := lastEnvUpdate(f)
					if err != nil {
						return err
					}
					if v, ok := body["websocket_application_id"]; !ok || v != nil {
						return fmt.Errorf("detaching must send websocket_application_id: null, sent %v", body)
					}
					return nil
				},
			},
		},
	})
}

// TestEnvironmentServingRootPlan changes the directory a static environment
// serves, which goes out with the environment's current, fixed mode.
func TestEnvironmentServingRootPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_environment.env"
	config := func(root string) string {
		return environmentSettingsConfig(baseURL, fmt.Sprintf(`  serving_root = %q`, root))
	}

	// The platform decides the mode on create; make every new environment a
	// static site, as it does for frameworks that build to static files.
	makeStatic := func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, env := range f.envs {
			env.Attributes.Serving = &client.EnvironmentServing{Mode: "static", Root: ptr(".")}
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Without a configured root the environment is created as is.
				Config: environmentSettingsConfig(baseURL, ""),
			},
			{
				PreConfig: makeStatic,
				Config:    config("dist"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("serving_mode"), knownvalue.StringExact("static")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("serving_root"), knownvalue.StringExact("dist")),
				},
				Check: func(*terraform.State) error {
					body, err := lastEnvUpdate(f)
					if err != nil {
						return err
					}
					serving, _ := body["serving"].(map[string]any)
					if serving == nil || serving["mode"] != "static" || serving["root"] != "dist" {
						return fmt.Errorf("serving = %v", body["serving"])
					}
					return nil
				},
			},
			{
				Config:           config("dist"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}
