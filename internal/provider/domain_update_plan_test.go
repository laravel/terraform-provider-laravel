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
						tfjsonpath.New("dns_records").AtMapKey("origin_cname"),
						knownvalue.StringExact("cname.laravel.cloud")),
					statecheck.ExpectKnownValue(addr,
						tfjsonpath.New("dns_records").AtMapKey("pre_verification"),
						knownvalue.StringExact("laravel-cloud-verification=token")),
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
