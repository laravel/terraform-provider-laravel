package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories returns provider factories for acceptance tests.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"laravel": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck validates the required environment variables for acceptance
// tests are set. Call this in every acceptance test's PreCheck function.
func testAccPreCheck(t *testing.T) {
	t.Helper()

	if v := os.Getenv("LARAVEL_CLOUD_API_TOKEN"); v == "" {
		t.Skip("LARAVEL_CLOUD_API_TOKEN must be set for acceptance tests")
	}
}

// Acceptance tests need a source repository that the account under test can
// actually reach, on a branch that still exists. Hardcoding either one rots:
// laravel/laravel's default branch moved from master to main to 13.x, and every
// environment test began failing with "The selected branch is no longer
// available" -- a 422 that looks like a provider bug but is only a stale
// fixture. Both are overridable so a fork, a private repo or a different branch
// can be used without editing the tests.
const (
	defaultTestRepository = "laravel/laravel"
	defaultTestBranch     = "13.x"
)

// testAccRepository returns the repository acceptance tests should build from.
func testAccRepository() string {
	if v := os.Getenv("LARAVEL_CLOUD_TEST_REPOSITORY"); v != "" {
		return v
	}
	return defaultTestRepository
}

// testAccBranch returns the branch acceptance tests should deploy.
func testAccBranch() string {
	if v := os.Getenv("LARAVEL_CLOUD_TEST_BRANCH"); v != "" {
		return v
	}
	return defaultTestBranch
}

// testAccPreCheckSlowResource skips acceptance tests for resources whose
// provisioning takes long enough to dominate a test run.
//
// A database cluster routinely takes twenty minutes or more to become
// available, and nothing in it -- not its schemas, not the cluster itself --
// can be deleted until it does. A create-then-destroy cycle therefore either
// blocks the run for a very long time or gives up and leaves a billable
// database behind. These tests are opt-in rather than part of the default
// acceptance run; the provider's behaviour is covered by the plan tests, which
// exercise the same code paths against the in-memory fake in under a second.
func testAccPreCheckSlowResource(t *testing.T) {
	t.Helper()

	if os.Getenv("LARAVEL_CLOUD_ACC_SLOW") == "" {
		t.Skip("set LARAVEL_CLOUD_ACC_SLOW=1 to run acceptance tests that provision slow resources (e.g. database clusters)")
	}
}
