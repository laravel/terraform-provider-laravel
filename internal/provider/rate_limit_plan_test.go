package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestRateLimitedReadsPlan: when Laravel Cloud's rate limiter refuses reads
// with a 429, the provider waits as told and retries them, so the apply
// succeeds instead of failing on the first refused read.
func TestRateLimitedReadsPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	config := basicAppConfig(baseURL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					f.mu.Lock()
					f.throttleReads = 4
					f.mu.Unlock()
				},
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("laravel_cloud_environment.production",
						tfjsonpath.New("php_major_version"), knownvalue.StringExact("8.4")),
				},
			},
		},
	})

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.throttleReads != 0 {
		t.Fatalf("the fake still had %d throttled reads to give; the test did not exercise them", f.throttleReads)
	}
}

// TestRateLimitedWritePlan: Laravel Cloud answers a throttled write with a
// 429. A write is sent only once, so the apply fails, and the error must show
// the 429 so the user can tell it was the rate limit.
func TestRateLimitedWritePlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	f.throttleWrites = 1

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      basicAppConfig(baseURL),
				ExpectError: regexp.MustCompile(`(?s)status\s+429.*Too\s+Many\s+Attempts`),
			},
		},
	})
}

// TestRedirectedWritePlan: Laravel Cloud answers some failed writes, such as
// an application create whose repository it cannot look up through the
// source control provider, with a bare 302 to the app root instead of a JSON
// error. The error must point at that connection, not at a rate limit, which
// Laravel Cloud answers with a 429.
func TestRedirectedWritePlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	f.redirectWrites = 1

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      basicAppConfig(baseURL),
				ExpectError: regexp.MustCompile(`(?s)302\s+redirect.*source\s+control\s+provider`),
			},
		},
	})
}
