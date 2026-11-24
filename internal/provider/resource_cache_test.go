package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCacheResource_basic(t *testing.T) {
	rName := testAccName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccCacheConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_cache.test", "name", rName),
					resource.TestCheckResourceAttrSet("laravel_cloud_cache.test", "id"),
					resource.TestCheckResourceAttr("laravel_cloud_cache.test", "type", "laravel_valkey"),
					resource.TestCheckResourceAttr("laravel_cloud_cache.test", "region", "us-east-2"),
					resource.TestCheckResourceAttr("laravel_cloud_cache.test", "size", "valkey-pro.250mb"),
				),
			},
			// Import
			{
				ResourceName:      "laravel_cloud_cache.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccCacheResource_elasticacheIsPublic: is_public was
// required, but the API always stores false for ElastiCache and refuses to
// change it, so true drifted on every plan and then failed to apply. It only
// plans, so no ElastiCache cache is created: true is refused at plan time,
// and leaving it out plans a create.
func TestAccCacheResource_elasticacheIsPublic(t *testing.T) {
	config := func(extra string) string {
		return fmt.Sprintf(`
resource "laravel_cloud_cache" "test" {
  name                 = %q
  type                 = "aws_elasticache_valkey"
  region               = "us-east-2"
  size                 = "elasticache.cache.t4g.micro"
  auto_upgrade_enabled = false
%s
}
`, testAccName(), extra)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      config(`  is_public = true`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`is_public must be false for ElastiCache caches`),
			},
			{
				Config:             config(""),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccCacheConfig(name string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_cache" "test" {
  name                 = %[1]q
  type                 = "laravel_valkey"
  region               = "us-east-2"
  size                 = "valkey-pro.250mb"
  auto_upgrade_enabled = true
  is_public            = false
}
`, name)
}

// TestAccCacheResource_hibernation: a Valkey Flex cache created
// through the API never scales to zero unless uses_hibernation is sent, which
// the provider could not do. Scale to zero is set on create, its timeout
// changed and then turned off in place, and eviction_policy, which the API
// defaults to allkeys-lru, is read back.
func TestAccCacheResource_hibernation(t *testing.T) {
	rName := testAccName()
	addr := "laravel_cloud_cache.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCacheHibernationConfig(rName, "uses_hibernation = true\n  hibernation_timeout = 5"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "uses_hibernation", "true"),
					resource.TestCheckResourceAttr(addr, "hibernation_timeout", "5"),
					resource.TestCheckResourceAttr(addr, "eviction_policy", "allkeys-lru"),
				),
			},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// The first update usually lands while the cache is still creating.
			{
				Config: testAccCacheHibernationConfig(rName, "uses_hibernation = true\n  hibernation_timeout = 10"),
				Check:  resource.TestCheckResourceAttr(addr, "hibernation_timeout", "10"),
			},
			{
				Config: testAccCacheHibernationConfig(rName, "uses_hibernation = false"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "uses_hibernation", "false"),
					resource.TestCheckNoResourceAttr(addr, "hibernation_timeout"),
				),
			},
		},
	})
}

func testAccCacheHibernationConfig(name, hibernation string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_cache" "test" {
  name                 = %[1]q
  type                 = "laravel_valkey"
  region               = "us-east-2"
  size                 = "valkey-flex-250mb"
  auto_upgrade_enabled = false
  is_public            = false
  %[2]s
}
`, name, hibernation)
}

// TestCacheBusyAttemptsFor: an ElastiCache cache gets the long provisioning
// budget for both Create and Delete, so a destroy soon after a create does
// not give up while the cache is still provisioning.
func TestCacheBusyAttemptsFor(t *testing.T) {
	cases := map[string]int{
		"laravel_valkey":         cacheBusyAttempts,
		"aws_elasticache_valkey": elastiCacheCreateAttempts,
		"aws_elasticache_redis":  elastiCacheCreateAttempts,
	}
	for cacheType, want := range cases {
		if got := cacheBusyAttemptsFor(cacheType); got != want {
			t.Errorf("cacheBusyAttemptsFor(%q) = %d, want %d", cacheType, got, want)
		}
	}
}
