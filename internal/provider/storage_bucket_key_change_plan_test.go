package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/laravel/terraform-provider-laravel/internal/client"
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

// TestStorageBucketInitialKeyByIDPlan: Laravel Cloud allows several keys with
// one name, so the initial key must be changed by its ID. Going by name, the
// second permission change deleted another key that shared the initial key's
// name, once the first had put the new initial key after it in the listing.
func TestStorageBucketInitialKeyByIDPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	config := func(permission string) string {
		return storageBucketKeyChangeConfig(baseURL, "main", permission) + `
resource "laravel_cloud_storage_bucket_key" "other" {
  bucket_id  = laravel_cloud_storage_bucket.test.id
  name       = "main"
  permission = "read_only"
}
`
	}
	var otherID string
	otherSurvives := func(s *terraform.State) error {
		if otherID == "" {
			otherID = s.RootModule().Resources["laravel_cloud_storage_bucket_key.other"].Primary.ID
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if _, ok := f.objects["storage_bucket_key"][otherID]; !ok {
			return fmt.Errorf("changing the initial key deleted key %s, which only shares its name", otherID)
		}
		if n := len(f.objects["storage_bucket_key"]); n != 2 {
			return fmt.Errorf("the bucket has %d keys, want the initial key and the other one", n)
		}
		return nil
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config("read_write"), Check: otherSurvives},
			{Config: config("read_only"), Check: otherSurvives},
			{Config: config("read_write"), Check: otherSurvives},
		},
	})
}

// TestStorageBucketInitialKeyOldKeyKeptPlan: when the old initial key cannot
// be deleted after its replacement was created, the apply must record the new
// key and warn. Failing it left state on the old key, so every retry created
// one more key.
func TestStorageBucketInitialKeyOldKeyKeptPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	keyCount := func(want int) resource.TestCheckFunc {
		return func(*terraform.State) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			if n := len(f.objects["storage_bucket_key"]); n != want {
				return fmt.Errorf("the bucket has %d keys, want %d", n, want)
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: storageBucketKeyChangeConfig(baseURL, "main", "read_write"), Check: keyCount(1)},
			{
				PreConfig: func() {
					f.mu.Lock()
					defer f.mu.Unlock()
					f.refuseBucketKeyDeletes = 1
				},
				Config: storageBucketKeyChangeConfig(baseURL, "main", "read_only"),
				// The new key and the old one it could not delete.
				Check: keyCount(2),
			},
			{
				Config:           storageBucketKeyChangeConfig(baseURL, "main", "read_only"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				Check:            keyCount(2),
			},
		},
	})
}

// TestStorageBucketInitialKeyByName: state recorded before key_id existed
// finds the initial key by name, but only when exactly one key has it.
func TestStorageBucketInitialKeyByName(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	c := client.NewClient("test-token", client.WithBaseURL(baseURL))
	r := &StorageBucketResource{client: c}
	ctx := context.Background()

	bucket, err := c.CreateStorageBucket(ctx, client.CreateStorageBucketRequest{
		Name: "by-name", Visibility: "private", Jurisdiction: "default", KeyName: "main", KeyPermission: "read_write",
	})
	if err != nil {
		t.Fatalf("creating the bucket: %v", err)
	}
	if _, err := r.initialKeyByName(ctx, bucket.ID, "main"); err != nil {
		t.Fatalf("one key named main: %v", err)
	}
	if _, err := r.initialKeyByName(ctx, bucket.ID, "missing"); err == nil {
		t.Error("no key named missing, but a key was found")
	}
	if _, err := c.CreateStorageBucketKey(ctx, bucket.ID, client.CreateStorageBucketKeyRequest{Name: "main", Permission: "read_only"}); err != nil {
		t.Fatalf("creating a second key named main: %v", err)
	}
	if id, err := r.initialKeyByName(ctx, bucket.ID, "main"); err == nil {
		t.Errorf("two keys named main, but %s was picked", id)
	}
}

// TestStorageBucketImportKeyPermissionPlan: after an import, state has no
// key_name or key_permission yet. A configuration asking for read_only used
// to be recorded as read_only while the key could still write. The first
// apply must find the key, see its real permission, and replace it.
func TestStorageBucketImportKeyPermissionPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	bucket, err := client.NewClient("test-token", client.WithBaseURL(baseURL)).CreateStorageBucket(context.Background(), client.CreateStorageBucketRequest{
		Name: "key-change-bucket", Visibility: "private", Jurisdiction: "default", KeyName: "main", KeyPermission: "read_write",
	})
	if err != nil {
		t.Fatalf("creating the bucket: %v", err)
	}
	const b = "laravel_cloud_storage_bucket.test"
	config := storageBucketKeyChangeConfig(baseURL, "main", "read_only")
	onlyReadOnlyKey := func(*terraform.State) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		var permissions []any
		for _, attrs := range f.objects["storage_bucket_key"] {
			permissions = append(permissions, attrs["permission"])
		}
		if len(permissions) != 1 || permissions[0] != "read_only" {
			return fmt.Errorf("the bucket's keys have permissions %v, want one read_only key", permissions)
		}
		return nil
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:             config,
				ResourceName:       b,
				ImportState:        true,
				ImportStateId:      bucket.ID,
				ImportStatePersist: true,
			},
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					onlyReadOnlyKey,
					resource.TestCheckResourceAttr(b, "key_permission", "read_only"),
				),
			},
			{
				Config:           config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}
