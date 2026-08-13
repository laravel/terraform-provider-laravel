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
