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

// TestInstanceAutoScalingAppliesPlan is an auto-scaled instance with no replica
// counts in config. The API reports 1 up to the plan's replica limit, which the
// provider used to plan as null, so every create failed with "inconsistent
// result after apply". It must plan, apply, and then re-plan empty.
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
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("min_replicas"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("max_replicas"), knownvalue.Int64Exact(fakeAutoMaxReplicas)),
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
// custom to automatic scaling and on to none.
//
// min_replicas and max_replicas are Optional+Computed with UseStateForUnknown,
// so removing them from config carries the prior values forward. The platform
// replaces them when the scaling type changes, so the plan must leave them
// unknown, read them back, and never send them for "auto", which the API
// rejects.
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
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("min_replicas"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("max_replicas"), knownvalue.Int64Exact(fakeAutoMaxReplicas)),
				},
			},
			{
				Config: instanceScalingConfig(baseURL, `scaling_type = "auto"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: instanceScalingConfig(baseURL, `scaling_type = "none"`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("min_replicas"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("max_replicas"), knownvalue.Int64Exact(1)),
				},
			},
		},
	})
}

// TestInstanceScalingThresholdsPlan covers changing and clearing the CPU and
// memory thresholds. Update never sent them, so a change failed with
// "inconsistent result after apply", and a removed threshold stayed set. When
// scaling stops the API clears them, and setting one then is rejected.
func TestInstanceScalingThresholdsPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	const addr = "laravel_cloud_instance.scaled"
	cpu := tfjsonpath.New("scaling_cpu_threshold_percentage")
	memory := tfjsonpath.New("scaling_memory_threshold_percentage")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: instanceScalingConfig(baseURL, "scaling_type = \"auto\"\n  scaling_cpu_threshold_percentage = 70"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, cpu, knownvalue.Int64Exact(70)),
					statecheck.ExpectKnownValue(addr, memory, knownvalue.Null()),
				},
			},
			{
				Config: instanceScalingConfig(baseURL,
					"scaling_type = \"auto\"\n  scaling_cpu_threshold_percentage = 80\n  scaling_memory_threshold_percentage = 60"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, cpu, knownvalue.Int64Exact(80)),
					statecheck.ExpectKnownValue(addr, memory, knownvalue.Int64Exact(60)),
				},
			},
			{
				Config: instanceScalingConfig(baseURL, "scaling_type = \"custom\"\n  min_replicas = 2\n  scaling_memory_threshold_percentage = 60"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, cpu, knownvalue.Null()),
					statecheck.ExpectKnownValue(addr, memory, knownvalue.Int64Exact(60)),
				},
			},
			{
				Config:      instanceScalingConfig(baseURL, "scaling_type = \"none\"\n  scaling_memory_threshold_percentage = 60"),
				ExpectError: regexp.MustCompile(`scaling_memory_threshold_percentage cannot be used without scaling`),
			},
			{
				Config: instanceScalingConfig(baseURL, `scaling_type = "none"`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, memory, knownvalue.Null()),
				},
			},
		},
	})
}
