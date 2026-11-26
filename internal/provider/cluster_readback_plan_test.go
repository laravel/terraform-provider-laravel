package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func clusterApplicationConfig(baseURL, appExtra, envExtra string) string {
	return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_application" "app" {
  name                         = "cluster-app"
  repository                   = "acme/cluster-app"
  region                       = "us-east-2"
  source_control_provider_type = "github"
%s
}

resource "laravel_cloud_environment" "staging" {
  application_id = laravel_cloud_application.app.id
  name           = "staging"
  branch         = "staging"
%s
}
`, appExtra, envExtra)
}

// TestEnvironmentClusterIDPlan: an environment's cluster_id is read back, so it
// survives an import and is kept when the configuration leaves it out. An
// environment cannot move, so a different cluster replaces it.
func TestEnvironmentClusterIDPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	const addr = "laravel_cloud_environment.staging"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: clusterApplicationConfig(baseURL, "", `  cluster_id = "dc-1"`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("cluster_id"), knownvalue.StringExact("dc-1")),
				},
			},
			{
				Config:           clusterApplicationConfig(baseURL, "", `  cluster_id = "dc-1"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				// Unset: the environment stays where it runs.
				Config:           clusterApplicationConfig(baseURL, "", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("cluster_id"), knownvalue.StringExact("dc-1")),
				},
			},
			{
				Config:            clusterApplicationConfig(baseURL, "", `  cluster_id = "dc-1"`),
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
				// Not reported by the API in the form it is sent.
				ImportStateVerifyIgnore: []string{"php_version"},
			},
			{
				Config: clusterApplicationConfig(baseURL, "", `  cluster_id = "dc-2"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionReplace),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("cluster_id"), knownvalue.StringExact("dc-2")),
				},
			},
		},
	})
}

// TestEnvironmentDefaultClusterChangePlan: a different cluster replaces an
// environment, which Laravel Cloud cannot do for an application's default
// environment: the delete would only drop it from state and the create would
// find it again where it was. So the plan fails, from shared compute and from
// a cluster alike, while any other environment is still replaced. Taking over
// an existing environment on another cluster fails the apply the same way.
func TestEnvironmentDefaultClusterChangePlan(t *testing.T) {
	config := func(baseURL, app, production, staging string) string {
		return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_application" "app" {
  name       = "default-cluster-app"
  repository = "example/app"
  region     = "us-east-2"
  %s
}

resource "laravel_cloud_environment" "production" {
  application_id = laravel_cloud_application.app.id
  name           = "production"
  branch         = "main"
  %s
}

resource "laravel_cloud_environment" "staging" {
  application_id = laravel_cloud_application.app.id
  name           = "staging"
  branch         = "main"
  %s
}
`, app, production, staging)
	}
	refused := regexp.MustCompile(`cluster_id cannot be changed on the default environment`)

	t.Run("shared compute", func(t *testing.T) {
		_, baseURL := newFakeCloud(t)
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{Config: config(baseURL, "", "", "")},
				{
					Config:      config(baseURL, "", `cluster_id = "dc-1"`, ""),
					ExpectError: refused,
				},
				{
					Config: config(baseURL, "", "", `cluster_id = "dc-1"`),
					ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("laravel_cloud_environment.staging", plancheck.ResourceActionDestroyBeforeCreate),
						plancheck.ExpectResourceAction("laravel_cloud_environment.production", plancheck.ResourceActionNoop),
					}},
				},
			},
		})
	})

	t.Run("dedicated cluster", func(t *testing.T) {
		_, baseURL := newFakeCloud(t)
		const app = `cluster_id = "dc-1"`
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{Config: config(baseURL, app, `cluster_id = "dc-1"`, "")},
				{
					Config:      config(baseURL, app, `cluster_id = "dc-2"`, ""),
					ExpectError: refused,
				},
				{
					// Known only after apply, it replaces the environment even
					// though it turns out to be the same cluster.
					Config: config(baseURL, app, `cluster_id = "${substr(laravel_cloud_environment.preview.id, 0, 0)}dc-1"`, "") + `
resource "laravel_cloud_environment" "preview" {
  application_id = laravel_cloud_application.app.id
  name           = "preview"
  branch         = "preview"
}
`,
					ExpectError: refused,
				},
			},
		})
	})

	t.Run("adoption", func(t *testing.T) {
		_, baseURL := newFakeCloud(t)
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					Config:      config(baseURL, `cluster_id = "dc-1"`, `cluster_id = "dc-2"`, ""),
					ExpectError: regexp.MustCompile(`cluster_id cannot be applied to the existing environment`),
				},
			},
		})
	})
}

// TestEnvironmentSharedComputeUpdatePlan: on shared compute cluster_id is null,
// so an unrelated change plans it unknown. That must update the environment in
// place, not replace it.
func TestEnvironmentSharedComputeUpdatePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	const addr = "laravel_cloud_environment.staging"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: clusterApplicationConfig(baseURL, "", `  color = "blue"`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("cluster_id"), knownvalue.Null()),
				},
			},
			{
				Config: clusterApplicationConfig(baseURL, "", `  color = "green"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("cluster_id"), knownvalue.Null()),
				},
			},
		},
	})
}

// TestApplicationClusterIDPlan: an application's cluster_id only places its
// default environment, and the API doesn't report it. An import reads it from
// the default environment, so a matching configuration plans nothing, and a
// later change only records the new value rather than replacing the
// application with all its environments.
func TestApplicationClusterIDPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	const addr = "laravel_cloud_application.app"
	app := func(cluster string) string {
		return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_application" "app" {
  name                         = "cluster-app"
  repository                   = "acme/cluster-app"
  region                       = "us-east-2"
  source_control_provider_type = "github"
  cluster_id                   = %q
}
`, cluster)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: app("dc-1"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("cluster_id"), knownvalue.StringExact("dc-1")),
				},
			},
			{
				// An import records cluster_id and source_control_provider_type
				// as the managed state has them, so a matching configuration
				// has nothing to add.
				Config:            app("dc-1"),
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: app("dc-2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("cluster_id"), knownvalue.StringExact("dc-2")),
				},
			},
			{
				Config:           app("dc-2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}
