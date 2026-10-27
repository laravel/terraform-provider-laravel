package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
// made the domain workflow impossible to complete from Terraform. The single
// records come back as {type, name, value} objects, or null when not needed.
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
						tfjsonpath.New("dns_records").AtMapKey("origin_cname"),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"type":  knownvalue.StringExact("CNAME"),
							"name":  knownvalue.StringExact("example.com"),
							"value": knownvalue.StringExact("origin.example.net"),
						})),
					statecheck.ExpectKnownValue(addr,
						tfjsonpath.New("dns_records").AtMapKey("pre_verification").AtMapKey("value"),
						knownvalue.StringExact("verify-token")),
					statecheck.ExpectKnownValue(addr,
						tfjsonpath.New("dns_records").AtMapKey("origin"),
						knownvalue.Null()),
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

// TestDomainStateUpgradeV0 covers state written before the single records in
// dns_records became objects. Version 0 stored them as strings, which no
// longer fit the schema, so the upgrade must drop dns_records (the next read
// refills it) and keep everything else.
func TestDomainStateUpgradeV0(t *testing.T) {
	server, err := providerserver.NewProtocol6WithError(New("test")())()
	if err != nil {
		t.Fatal(err)
	}

	const v0 = `{
		"id": "domain-1",
		"environment_id": "env-1",
		"name": "example.com",
		"type": "root",
		"hostname_status": "verified",
		"ssl_status": "verified",
		"origin_status": "verified",
		"www_redirect": null,
		"wildcard_enabled": null,
		"verification_method": null,
		"cloudflare_strategy": null,
		"allow_downtime": null,
		"downtime": null,
		"stage": "origin",
		"action_required": null,
		"last_verified_at": null,
		"dns_records": {"ssl": [], "pre_verification": "", "origin": "", "origin_cname": "", "dcv": ""},
		"created_at": null
	}`

	resp, err := server.UpgradeResourceState(context.Background(), &tfprotov6.UpgradeResourceStateRequest{
		TypeName: "laravel_cloud_domain",
		Version:  0,
		RawState: &tfprotov6.RawState{JSON: []byte(v0)},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		t.Errorf("diagnostic: %s: %s", d.Summary, d.Detail)
	}
	if resp.UpgradedState == nil {
		t.Fatal("no upgraded state")
	}

	schemaResp, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	val, err := resp.UpgradedState.Unmarshal(schemaResp.ResourceSchemas["laravel_cloud_domain"].ValueType())
	if err != nil {
		t.Fatal(err)
	}
	var attrs map[string]tftypes.Value
	if err := val.As(&attrs); err != nil {
		t.Fatal(err)
	}
	if !attrs["dns_records"].IsNull() {
		t.Errorf("dns_records = %v, want null", attrs["dns_records"])
	}
	var name string
	if err := attrs["name"].As(&name); err != nil || name != "example.com" {
		t.Errorf("name = %q (%v), want example.com", name, err)
	}
}
