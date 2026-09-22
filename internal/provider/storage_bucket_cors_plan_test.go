package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestStorageBucketCORSClearingPlan proves that removing the cors_settings
// block actually takes the rules off the bucket.
//
// The PATCH body carried cors_settings as a *CorsSettings with omitempty, which
// can only say "absent" or "an object" -- never "clear this". Removing the
// block from a configuration therefore sent an empty body: state recorded the
// rules as gone while the bucket went on serving them, and because the provider
// deliberately does not read cors_settings back, nothing ever noticed. The
// rules could not be removed through Terraform at all.
//
// cors_settings is asserted on the fake's stored attributes rather than on
// Terraform state precisely because state is not evidence here -- it reflects
// the configuration either way. Only the request body distinguishes the two.
func TestStorageBucketCORSClearingPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)

	storedCORS := func() (any, bool) {
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, attrs := range f.objects["storage_bucket"] {
			v, ok := attrs["cors_settings"]
			return v, ok
		}
		return nil, false
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: storageBucketCORSConfig(baseURL, `
  cors_settings = {
    allowed_origins = ["https://example.com"]
    allowed_methods = ["GET"]
  }
`),
				Check: func(*terraform.State) error {
					if v, ok := storedCORS(); !ok || v == nil {
						return fmt.Errorf("bucket has no cors_settings after create, got %v", v)
					}
					return nil
				},
			},
			{
				// The block is gone: the bucket must end up with no rules.
				Config: storageBucketCORSConfig(baseURL, ""),
				Check: func(*terraform.State) error {
					v, ok := storedCORS()
					if !ok {
						return fmt.Errorf("bucket was never patched to clear cors_settings")
					}
					if v != nil {
						return fmt.Errorf("bucket still has cors_settings %v after the block was removed", v)
					}
					return nil
				},
			},
		},
	})
}

func storageBucketCORSConfig(baseURL, cors string) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_storage_bucket" "assets" {
  name     = "cors-bucket"
  key_name = "cors-key"
%[2]s}
`, baseURL, cors)
}
