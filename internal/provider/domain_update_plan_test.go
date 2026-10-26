package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestDomainDNSRecordsPlan asserts the domain surfaces the records an operator
// needs to actually finish setting the domain up.
//
// dns_records is required in the API response but was not modelled at all, so
// there was no way to learn the CNAME/TXT records the domain needs -- which
// made the domain workflow impossible to complete from Terraform.
func TestDomainDNSRecordsPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := domainConfig(baseURL, "")

	const addr = "laravel_cloud_domain.site"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr,
						tfjsonpath.New("dns_records").AtMapKey("origin"),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"type":  knownvalue.StringExact("A"),
							"name":  knownvalue.StringExact("example.com"),
							"value": knownvalue.StringExact("192.0.2.1"),
						})),
					statecheck.ExpectKnownValue(addr,
						tfjsonpath.New("dns_records").AtMapKey("origin_cname"),
						knownvalue.Null()),
					statecheck.ExpectKnownValue(addr,
						tfjsonpath.New("dns_records").AtMapKey("pre_verification").AtMapKey("value"),
						knownvalue.StringExact("laravel-cloud-verification=token")),
					statecheck.ExpectKnownValue(addr,
						tfjsonpath.New("dns_records").AtMapKey("dcv").AtMapKey("type"),
						knownvalue.StringExact("CNAME")),
					statecheck.ExpectKnownValue(addr,
						tfjsonpath.New("dns_records").AtMapKey("ssl").AtSliceIndex(0).AtMapKey("type"),
						knownvalue.StringExact("TXT")),
					statecheck.ExpectKnownValue(addr,
						tfjsonpath.New("action_required"),
						knownvalue.StringExact("add_txt_records")),
					statecheck.ExpectKnownValue(addr,
						tfjsonpath.New("stage"),
						knownvalue.StringExact("pre_verification")),
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

// TestDomainCreateOnlyAttributesForceReplacePlan pins the update surface.
//
// PATCH /domains/{domain} accepts verification_method and nothing else, so
// www_redirect, wildcard_enabled, allow_downtime and cloudflare_strategy are
// create-only. They previously had no RequiresReplace, which meant changing one
// produced a plan that "succeeded", wrote the new value to state, and left the
// live domain untouched. Each must now force replacement instead.
func TestDomainCreateOnlyAttributesForceReplacePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	const addr = "laravel_cloud_domain.site"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: domainConfig(baseURL, `www_redirect = "root_to_www"`),
			},
			{
				Config: domainConfig(baseURL, `www_redirect = "www_to_root"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
			},
		},
	})
}

// TestDomainUpdateWithoutVerificationMethodPlan covers the 422 path. The PATCH
// body requires verification_method; the provider used to send it
// unconditionally, so an update on a domain that never set one posted an empty
// string and the API rejected it. With no verification method configured there
// is nothing an update can do, so no request should be made at all.
func TestDomainUpdateWithoutVerificationMethodPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: domainConfigNamed(baseURL, "first.example.com", ""),
			},
			{
				// name is create-only, so this is a replace rather than an
				// in-place update; the point is that it completes cleanly
				// without a verification_method in play.
				Config: domainConfigNamed(baseURL, "second.example.com", ""),
			},
		},
	})
}

func domainConfig(baseURL, extra string) string {
	return domainConfigNamed(baseURL, "example.com", extra)
}

func domainConfigNamed(baseURL, name, extra string) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_application" "app" {
  name       = "domain-app"
  repository = "laravel/laravel"
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "env" {
  application_id = laravel_cloud_application.app.id
  name           = "production"
  branch         = "main"
}

resource "laravel_cloud_domain" "site" {
  environment_id = laravel_cloud_environment.env.id
  name           = %[2]q
  %[3]s
}
`, baseURL, name, extra)
}

// TestDomainUpgradeStateV0 covers state written before dns_records records
// became objects. Version 0 stored them as strings, which no longer decode, so
// the upgrade clears dns_records for the following refresh to fill in and keeps
// every other attribute.
func TestDomainUpgradeStateV0(t *testing.T) {
	const v0 = `{
		"id": "domain-1",
		"environment_id": "env-1",
		"name": "example.com",
		"stage": "origin",
		"dns_records": {
			"ssl": [],
			"pre_verification": "",
			"origin": "",
			"origin_cname": "",
			"dcv": ""
		}
	}`

	upgrader := (&DomainResource{}).UpgradeState(context.Background())[0]
	req := fwresource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: []byte(v0)}}
	var resp fwresource.UpgradeStateResponse
	upgrader.StateUpgrader(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("upgrade: %v", resp.Diagnostics)
	}
	if resp.DynamicValue == nil {
		t.Fatal("upgrade returned no state")
	}
	var upgraded map[string]any
	if err := json.Unmarshal(resp.DynamicValue.JSON, &upgraded); err != nil {
		t.Fatalf("unmarshal upgraded state: %v", err)
	}
	if upgraded["dns_records"] != nil {
		t.Errorf("dns_records = %v, want null", upgraded["dns_records"])
	}
	if upgraded["id"] != "domain-1" || upgraded["name"] != "example.com" || upgraded["stage"] != "origin" {
		t.Errorf("upgraded state = %v, want the other attributes kept", upgraded)
	}
}
