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

// testAccFixture returns the value of an environment variable naming a fixture
// that only some test organizations have, such as a repository with a
// particular layout. The test is skipped when it is not set.
func testAccFixture(t *testing.T, name string) string {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		t.Skipf("%s is not set", name)
	}
	return v
}

// testAccBranch returns the branch acceptance tests should deploy.
func testAccBranch() string {
	if v := os.Getenv("LARAVEL_CLOUD_TEST_BRANCH"); v != "" {
		return v
	}
	return defaultTestBranch
}
