package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestStorageBucketDeletePlan: destroying a bucket deleted all its access keys
// before it tried the bucket, so a bucket delete that then failed, as for a
// token that may not delete it, left the bucket without keys. The bucket is
// now tried first, and once its keys are gone, a bucket busy with another
// change is waited out instead of left behind.
func TestStorageBucketDeletePlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	provider := fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %q
}
`, baseURL)
	bucket := provider + `
resource "laravel_cloud_storage_bucket" "test" {
  name           = "delete-bucket"
  key_name       = "key1"
  key_permission = "read_write"
}
`
	counts := func() (buckets, keys int) {
		f.mu.Lock()
		defer f.mu.Unlock()
		return len(f.objects["storage_bucket"]), len(f.objects["storage_bucket_key"])
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: bucket},
			{
				PreConfig: func() {
					f.mu.Lock()
					defer f.mu.Unlock()
					f.forbidDeletes["storage_bucket"] = 1
				},
				Config:      provider,
				ExpectError: regexp.MustCompile(`unauthorized`),
			},
			{
				PreConfig: func() {
					if buckets, keys := counts(); buckets != 1 || keys != 1 {
						t.Errorf("a refused bucket delete left %d buckets and %d keys, want 1 and 1", buckets, keys)
					}
					f.mu.Lock()
					defer f.mu.Unlock()
					for id := range f.objects["storage_bucket"] {
						f.busy[id] = 2
					}
				},
				Config: provider,
				Check: func(*terraform.State) error {
					if buckets, keys := counts(); buckets != 0 || keys != 0 {
						return fmt.Errorf("destroy left %d buckets and %d keys", buckets, keys)
					}
					return nil
				},
			},
		},
	})
}
