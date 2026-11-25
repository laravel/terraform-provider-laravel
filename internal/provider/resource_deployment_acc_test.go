package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// While one of an environment's resources is provisioning, the API
// queues a deployment and answers 202 with a queued deployment (qdepl-...).
// The provider stored that id, GET /deployments/qdepl-... is a 404, so the
// deployment dropped out of state and the next apply deployed again.
//
// Resizing the environment's cache and deploying in one apply queues the
// deployment: the cache is updating when it is requested.
func TestAccDeployment_queued(t *testing.T) {
	name := testAccName()
	const addr = "laravel_cloud_deployment.test"
	var cacheID, envID string
	var cacheCreated time.Time

	waitForCache := func() {
		ctx := context.Background()
		for start := time.Now(); ; time.Sleep(10 * time.Second) {
			cache, err := testAccClient().GetCache(ctx, cacheID)
			if err == nil && cache.Attributes.Status == "available" {
				break
			}
			if time.Since(start) > 15*time.Minute {
				t.Fatalf("cache %s is not available after 15 minutes (err: %v)", cacheID, err)
			}
		}
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDeploymentQueuedConfig(name, "valkey-flex-250mb", false, false),
				Check: func(s *terraform.State) error {
					cacheID = s.RootModule().Resources["laravel_cloud_cache.test"].Primary.ID
					cacheCreated = time.Now()
					return nil
				},
			},
			{
				// The API refuses to attach a cache that is still being
				// created, and for about a minute after an application is
				// created, creating an environment can fail with "The selected
				// branch is no longer available".
				PreConfig: func() {
					waitForCache()
					time.Sleep(time.Until(cacheCreated.Add(75 * time.Second)))
				},
				Config: testAccDeploymentQueuedConfig(name, "valkey-flex-250mb", true, false),
				Check: func(s *terraform.State) error {
					envID = s.RootModule().Resources["laravel_cloud_environment.test"].Primary.ID
					return nil
				},
			},
			{
				// The framework's plan after apply must be empty: on the old
				// code the queued id reads as a 404 and plans a new deployment.
				PreConfig: waitForCache,
				Config:    testAccDeploymentQueuedConfig(name, "valkey-flex-1gb", true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(addr, "id", regexp.MustCompile(`^depl-`)),
					resource.TestCheckResourceAttrPair(addr, "environment_id", "laravel_cloud_environment.test", "id"),
					func(*terraform.State) error {
						deployments, err := testAccClient().ListDeployments(context.Background(), envID)
						if err != nil {
							return err
						}
						if len(deployments) != 1 {
							return fmt.Errorf("want 1 deployment of environment %s, Cloud has %d", envID, len(deployments))
						}
						return nil
					},
				),
			},
		},
	})
}

func testAccDeploymentQueuedConfig(name, cacheSize string, withEnvironment, deploy bool) string {
	config := fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}

resource "laravel_cloud_cache" "test" {
  name                 = %[1]q
  type                 = "laravel_valkey"
  region               = "us-east-2"
  size                 = %[4]q
  auto_upgrade_enabled = false
  is_public            = false
}
`, name, testAccRepository(), testAccBranch(), cacheSize)
	if withEnvironment {
		config += fmt.Sprintf(`
# Not "production": the API refuses to delete an application's production
# environment.
resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "staging"
  branch         = %q
  cache_id       = laravel_cloud_cache.test.id
}
`, testAccBranch())
	}
	if deploy {
		config += `
resource "laravel_cloud_deployment" "test" {
  environment_id = laravel_cloud_environment.test.id
  # Deploy after the resize has been sent, while the cache is updating.
  depends_on = [laravel_cloud_cache.test]
}
`
	}
	return config
}
