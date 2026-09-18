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

// TestInstanceAutoScalingRejectsReplicasPlan pins the scaling_type/replica
// rule. The API documents min_replicas and max_replicas as "only applicable to
// the custom scaling type, and rejected when used with auto".
//
// Previously both were Required in the schema, so there was no way to express
// an auto-scaled instance at all -- every such config was rejected by the API
// at apply time. They are now optional, and setting them alongside "auto" is
// caught during validation instead of surfacing as an opaque 422.
func TestInstanceAutoScalingRejectsReplicasPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      instanceScalingConfig(baseURL, `scaling_type = "auto"`+"\n  min_replicas = 1"),
				ExpectError: regexp.MustCompile(`min_replicas cannot be used with automatic scaling`),
			},
			{
				Config:      instanceScalingConfig(baseURL, `scaling_type = "auto"`+"\n  max_replicas = 4"),
				ExpectError: regexp.MustCompile(`max_replicas cannot be used with automatic scaling`),
			},
			{
				Config:      instanceScalingConfig(baseURL, `scaling_type = "custom"`),
				ExpectError: regexp.MustCompile(`min_replicas is required for custom scaling`),
			},
		},
	})
}

// TestInstanceAutoScalingAppliesPlan is the positive case the old schema made
// impossible: an auto-scaled instance with no replica counts at all. It must
// plan, apply, and then re-plan empty.
func TestInstanceAutoScalingAppliesPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := instanceScalingConfig(baseURL, `scaling_type = "auto"`)

	const addr = "laravel_cloud_instance.scaled"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("scaling_type"), knownvalue.StringExact("auto")),
				},
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

func instanceScalingConfig(baseURL, scaling string) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_application" "app" {
  name       = "scaling-app"
  repository = "laravel/laravel"
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "env" {
  application_id = laravel_cloud_application.app.id
  name           = "production"
  branch         = "main"
}

resource "laravel_cloud_instance" "scaled" {
  environment_id = laravel_cloud_environment.env.id
  name           = "web"
  size           = "flex.c-1vcpu-256mb"
  %[2]s
}
`, baseURL, scaling)
}

// TestInstanceSwitchToAutoScalingPlan covers moving an existing instance from
// custom to automatic scaling.
//
// min_replicas and max_replicas are Optional+Computed with UseStateForUnknown,
// so removing them from config does not plan a change -- the prior values carry
// forward. That left an instance with scaling_type "auto" and replica counts
// still set: the exact combination the validator rejects and the API 422s, with
// no way out, because removing them from config did nothing and setting them
// failed validation. The plan must clear them instead.
func TestInstanceSwitchToAutoScalingPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	const addr = "laravel_cloud_instance.scaled"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: instanceScalingConfig(baseURL,
					"scaling_type = \"custom\"\n  min_replicas = 2\n  max_replicas = 5"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("min_replicas"), knownvalue.Int64Exact(2)),
				},
			},
			{
				Config: instanceScalingConfig(baseURL, `scaling_type = "auto"`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("scaling_type"), knownvalue.StringExact("auto")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("min_replicas"), knownvalue.Null()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("max_replicas"), knownvalue.Null()),
				},
			},
			{
				Config: instanceScalingConfig(baseURL, `scaling_type = "auto"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
