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

// Renaming a bucket's initial key must rename the key in place, not
// replace the bucket and destroy everything stored in it.
//
// The plan check runs before apply, so on a provider with the bug the test
// fails without ever replacing the bucket.
func TestAccStorageBucket_renameKeyKeepsBucket(t *testing.T) {
	name := testAccName()
	var bucketID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		// Every bucket gets an access key on create, and the API refuses to
		// delete a bucket that still has one, so this also proves destroy works.
		CheckDestroy: func(*terraform.State) error {
			_, err := testAccClient().GetStorageBucket(context.Background(), bucketID)
			if client.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("bucket %s still exists after destroy (err: %v)", bucketID, err)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccStorageBucketKeyConfig(name, name+"-key1"),
				Check: func(s *terraform.State) error {
					bucketID = s.RootModule().Resources["laravel_cloud_storage_bucket.test"].Primary.ID
					return nil
				},
			},
			{
				Config: testAccStorageBucketKeyConfig(name, name+"-key2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("laravel_cloud_storage_bucket.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_storage_bucket.test", "key_name", name+"-key2"),
					func(s *terraform.State) error {
						if got := s.RootModule().Resources["laravel_cloud_storage_bucket.test"].Primary.ID; got != bucketID {
							return fmt.Errorf("bucket was replaced: id %s became %s", bucketID, got)
						}
						keys, err := testAccClient().ListStorageBucketKeys(context.Background(), bucketID)
						if err != nil {
							return err
						}
						for _, k := range keys {
							if k.Attributes.Name == name+"-key2" {
								return nil
							}
						}
						return fmt.Errorf("no key named %s on bucket %s", name+"-key2", bucketID)
					},
				),
			},
			// A permission change swaps the key but keeps the bucket.
			{
				Config: testAccStorageBucketKeyConfigPerm(name, name+"-key3", "read_only"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("laravel_cloud_storage_bucket.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: func(s *terraform.State) error {
					if got := s.RootModule().Resources["laravel_cloud_storage_bucket.test"].Primary.ID; got != bucketID {
						return fmt.Errorf("bucket was replaced: id %s became %s", bucketID, got)
					}
					keys, err := testAccClient().ListStorageBucketKeys(context.Background(), bucketID)
					if err != nil {
						return err
					}
					if len(keys) != 1 || keys[0].Attributes.Name != name+"-key3" || keys[0].Attributes.Permission != "read_only" {
						return fmt.Errorf("want one read_only key %s-key3, got %+v", name, keys)
					}
					return nil
				},
			},
		},
	})
}

func testAccStorageBucketKeyConfig(name, keyName string) string {
	return testAccStorageBucketKeyConfigPerm(name, keyName, "read_write")
}

func testAccStorageBucketKeyConfigPerm(name, keyName, permission string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_storage_bucket" "test" {
  name           = %[1]q
  key_name       = %[2]q
  key_permission = %[3]q
}
`, name, keyName, permission)
}
