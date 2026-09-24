package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestStorageBucketCORSClearingPlan proves that emptying or removing the CORS
// origins actually reaches the bucket.
//
// cors_settings used to be a *CorsSettings whose every list carried omitempty,
// so an emptied list was dropped from the request body exactly when it mattered.
// Because the API merges a cors_settings body into the bucket's current rules,
// sending nothing means "leave it alone": Terraform recorded the origins as
// gone while the bucket went on serving them, and since the provider
// deliberately does not read cors_settings back, no refresh ever noticed.
//
// The assertions run against the fake's stored attributes rather than Terraform
// state, because state reflects the configuration either way -- only the
// request body distinguishes a real change from a silent no-op.
//
// The bucket keeps a non-empty allowed_methods throughout: the API rejects an
// empty method list outright, so "no CORS" is expressed as "no origins".
func TestStorageBucketCORSClearingPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)

	bucketAttrs := func() map[string]any {
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, attrs := range f.objects["storage_bucket"] {
			return cloneMap(attrs)
		}
		return nil
	}
	corsList := func(attrs map[string]any, key string) []any {
		cors, _ := attrs["cors_settings"].(map[string]any)
		list, _ := cors[key].([]any)
		return list
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: storageBucketCORSConfig(baseURL, `
  cors_settings = {
    allowed_origins = ["https://example.com"]
    allowed_methods = ["GET"]
    expose_headers  = ["x-trace"]
  }
`),
				Check: func(*terraform.State) error {
					attrs := bucketAttrs()
					if got := corsList(attrs, "allowed_origins"); len(got) != 1 {
						return fmt.Errorf("after create, allowed_origins = %v, want one entry", got)
					}
					return nil
				},
			},
			{
				// Emptying a list in place must reach the bucket. expose_headers
				// is the load-bearing one: it is the field omitempty silently
				// dropped, and the merge means an absent field changes nothing.
				Config: storageBucketCORSConfig(baseURL, `
  cors_settings = {
    allowed_origins = ["https://example.com"]
    allowed_methods = ["GET"]
    expose_headers  = []
  }
`),
				Check: func(*terraform.State) error {
					attrs := bucketAttrs()
					if got := corsList(attrs, "expose_headers"); len(got) != 0 {
						return fmt.Errorf("expose_headers = %v after being emptied, want []", got)
					}
					if got := corsList(attrs, "allowed_methods"); len(got) != 1 {
						return fmt.Errorf("allowed_methods = %v, want the merge to leave it alone", got)
					}
					return nil
				},
			},
			{
				// The whole block is gone. The API cannot take CORS off a
				// bucket, so the provider empties the origin list instead --
				// the one state it accepts that means "allow nothing".
				Config: storageBucketCORSConfig(baseURL, ""),
				Check: func(*terraform.State) error {
					attrs := bucketAttrs()
					if got := corsList(attrs, "allowed_origins"); len(got) != 0 {
						return fmt.Errorf("allowed_origins = %v after the block was removed, want []", got)
					}
					return nil
				},
			},
		},
	})
}

// TestStorageBucketEmptyMethodsRejectedPlan keeps the API's one CORS invariant
// at plan time: allowed_methods cannot be empty, so a config that empties it
// can never apply and should not reach the API to find that out.
func TestStorageBucketEmptyMethodsRejectedPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: storageBucketCORSConfig(baseURL, `
  cors_settings = {
    allowed_origins = ["https://example.com"]
    allowed_methods = []
  }
`),
				ExpectError: regexp.MustCompile(`allowed_methods cannot be empty`),
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
