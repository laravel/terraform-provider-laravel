package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestUserAgentPlan: every request the provider sends names the provider
// release and the Terraform version in its User-Agent, so Laravel Cloud's
// request logs can tell which release sent a request. Without it, every
// request showed up as Go's default "Go-http-client".
func TestUserAgentPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	want := regexp.MustCompile(`^Terraform/\d+\.\d+\.\d+\S* \(\+https://www\.terraform\.io\) terraform-provider-laravel/test$`)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: basicAppConfig(baseURL)},
		},
	})

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.userAgents) == 0 {
		t.Fatal("the fake saw no requests")
	}
	for ua, n := range f.userAgents {
		if !want.MatchString(ua) {
			t.Errorf("%d request(s) sent User-Agent %q, want one matching %s", n, ua, want)
		}
	}
}

func TestUserAgent(t *testing.T) {
	for _, tc := range []struct{ terraform, provider, want string }{
		{"1.17.1", "1.7.0", "Terraform/1.17.1 (+https://www.terraform.io) terraform-provider-laravel/1.7.0"},
		// Terraform does not always report its version.
		{"", "1.7.0", "terraform-provider-laravel/1.7.0"},
	} {
		if got := userAgent(tc.terraform, tc.provider); got != tc.want {
			t.Errorf("userAgent(%q, %q) = %q, want %q", tc.terraform, tc.provider, got, tc.want)
		}
	}
}
