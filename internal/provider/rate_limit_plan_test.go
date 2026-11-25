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
// bare 302, which the provider cannot safely retry. The error must say it may
// be a rate limit rather than leave the user to guess.
func TestRateLimitedWritePlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	f.throttleWrites = 1

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      basicAppConfig(baseURL),
				ExpectError: regexp.MustCompile(`(?s)302\s+redirect.*may\s+be\s+a\s+rate\s+limit`),
			},
		},
	})
}
