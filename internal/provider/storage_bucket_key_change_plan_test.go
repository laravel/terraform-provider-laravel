package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestStorageBucketKeyChangePlan is the fast counterpart of
// TestAccStorageBucket_renameKeyKeepsBucket: changing the initial
// key's name or permission updates the bucket in place and changes its key,
// instead of replacing the bucket and destroying its contents.
func TestStorageBucketKeyChangePlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)

	keys := func() []map[string]any {
		f.mu.Lock()
		defer f.mu.Unlock()
		var out []map[string]any
		for _, attrs := range f.objects["storage_bucket_key"] {
			out = append(out, cloneMap(attrs))
		}
		return out
	}
	wantOneKey := func(name, permission string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			got := keys()
			if len(got) != 1 || got[0]["name"] != name || got[0]["permission"] != permission {
				return fmt.Errorf("want one %s key %q, got %v", permission, name, got)
			}
			return nil
		}
	}
	inPlace := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
		plancheck.ExpectResourceAction("laravel_cloud_storage_bucket.test", plancheck.ResourceActionUpdate),
	}}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: storageBucketKeyChangeConfig(baseURL, "key1", "read_write"),
				Check:  wantOneKey("key1", "read_write"),
			},
			{
				Config:           storageBucketKeyChangeConfig(baseURL, "key2", "read_write"),
				ConfigPlanChecks: inPlace,
				Check:            wantOneKey("key2", "read_write"),
			},
			{
				Config:           storageBucketKeyChangeConfig(baseURL, "key3", "read_only"),
				ConfigPlanChecks: inPlace,
				Check:            wantOneKey("key3", "read_only"),
			},
		},
	})
}

func storageBucketKeyChangeConfig(baseURL, keyName, permission string) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_storage_bucket" "test" {
  name           = "key-change-bucket"
  key_name       = %[2]q
  key_permission = %[3]q
}
`, baseURL, keyName, permission)
}
