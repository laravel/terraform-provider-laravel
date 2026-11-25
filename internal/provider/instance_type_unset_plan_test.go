package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestInstanceImportWithoutTypePlan: type defaulted to "service" even for an
// instance that already existed, so importing an environment's app instance
// without setting type planned to replace it. Unset, an existing instance now
// keeps its type.
func TestInstanceImportWithoutTypePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	const addr = "laravel_cloud_instance.app"
	config := providerCfg(baseURL) + appEnvStack + `
resource "laravel_cloud_instance" "app" {
  environment_id = laravel_cloud_environment.env.id
  name           = "App"
  size           = "flex-512mb"
  scaling_type   = "none"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: providerCfg(baseURL) + appEnvStack},
			{
				// Validation only sees the configuration, which without a type
				// describes a service: an app-only setting asks for the type.
				Config: strings.Replace(config, `scaling_type   = "none"`, "scaling_type   = \"none\"\n  uses_octane    = true", 1),
				ExpectError: regexp.MustCompile(
					`with\s+type\s+unset,\s+this\s+configuration\s+is\s+for\s+a\s+service\.\s+Set\s+type\s+=\s+"app"`),
			},
			{
				Config:       config,
				ResourceName: addr,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return "instance-app-" + s.RootModule().Resources["laravel_cloud_environment.env"].Primary.ID, nil
				},
				ImportStatePersist: true,
			},
			{
				Config:           config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("type"), knownvalue.StringExact("app")),
				},
			},
		},
	})
}

// TestInstanceTypeRemovedPlan: removing type from an existing instance's
// configuration keeps its type instead of replacing it with a service, which
// for a managed queue destroyed the queue. A new instance without a type is
// still a service. (Validation still reads an unset type as service, so the
// queue's settings here are ones a service accepts too.)
func TestInstanceTypeRemovedPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := func(queueType string) string {
		return providerCfg(baseURL) + appEnvStack + fmt.Sprintf(`
resource "laravel_cloud_instance" "q" {
  environment_id = laravel_cloud_environment.env.id
  name           = "jobs"
  size           = "mq.flex.256mb"
  scaling_type   = "custom"
  min_replicas   = 1
  max_replicas   = 2
  %s
}

resource "laravel_cloud_instance" "svc" {
  environment_id = laravel_cloud_environment.env.id
  name           = "svc"
  size           = "flex-512mb"
  scaling_type   = "custom"
  min_replicas   = 1
  max_replicas   = 1
}
`, queueType)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(`type = "managed_queue"`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("laravel_cloud_instance.svc", tfjsonpath.New("type"), knownvalue.StringExact("service")),
				},
			},
			{
				Config:           config(""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("laravel_cloud_instance.q", tfjsonpath.New("type"), knownvalue.StringExact("managed_queue")),
				},
			},
		},
	})
}
