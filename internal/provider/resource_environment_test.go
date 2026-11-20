package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

func TestAccEnvironmentResource_basic(t *testing.T) {
	appName := testAccName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read. laravel/laravel's default branch, 13.x, is not
			// production-like, so the application's default environment is
			// named "13 x" and this "production" environment is a new one.
			// TestAccEnvironmentResource_adoptDefault covers taking over the
			// default environment.
			{
				Config: testAccEnvironmentConfig(appName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "name", "production"),
					resource.TestCheckResourceAttrSet("laravel_cloud_environment.test", "id"),
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "branch", testAccBranch()),
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "php_version", "8.4:1"),
				),
			},
			// Import
			{
				ResourceName:      "laravel_cloud_environment.test",
				ImportState:       true,
				ImportStateVerify: true,
				// application_id is not in the GET response and branch is
				// write-only (the API accepts it but never reports it back), so
				// an imported environment cannot know them and leaves them null.
				ImportStateVerifyIgnore: []string{"application_id", "branch"},
			},
		},
	})
}

// TestAccEnvironmentResource_adoptDefault takes over the environment Laravel
// Cloud creates with the application and then destroys it. The API
// refuses to delete an application's default environment, so destroy must
// leave it to the application's own deletion.
func TestAccEnvironmentResource_adoptDefault(t *testing.T) {
	appName := testAccName()
	// Laravel Cloud names an application's default environment "production"
	// when the repository's default branch is production-like (main, master,
	// prod, live or production), so a "production" environment takes it over.
	repository := testAccFixture(t, "LARAVEL_CLOUD_TEST_MAIN_BRANCH_REPOSITORY")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "production"
  branch         = "main"
}
`, appName, repository),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "name", "production"),
					testAccCheckDefaultEnvironment("laravel_cloud_application.test", "laravel_cloud_environment.test"),
				),
			},
		},
	})
}

// testAccCheckDefaultEnvironment checks that the environment is the
// application's default one, so the test really covers taking it over.
func testAccCheckDefaultEnvironment(appResource, envResource string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		appID := s.RootModule().Resources[appResource].Primary.ID
		envID := s.RootModule().Resources[envResource].Primary.ID
		app, err := testAccClient().GetApplication(context.Background(), appID)
		if err != nil {
			return err
		}
		if got := app.Relationships.DefaultEnvironment.RelatedID(); got != envID {
			return fmt.Errorf("application %s has default environment %q, want the managed environment %q", appID, got, envID)
		}
		return nil
	}
}

func TestAccEnvironmentResource_update(t *testing.T) {
	appName := testAccName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with initial settings
			{
				Config: testAccEnvironmentConfig(appName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "timeout", "30"),
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "uses_octane", "true"),
				),
			},
			// Update timeout
			{
				Config: testAccEnvironmentConfigUpdated(appName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "timeout", "45"),
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "uses_octane", "false"),
				),
			},
		},
	})
}

// TestAccEnvironmentResource_vanityDomain sets a vanity domain on a real
// environment. The API takes a single label and reports the full
// hostname, so a configured label must still plan empty after apply. It sets
// the domain only once: the API allows one change per environment every
// 30 minutes.
func TestAccEnvironmentResource_vanityDomain(t *testing.T) {
	appName := testAccName()
	vanity := testAccName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEnvironmentVanityConfig(appName, vanity),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "vanity_domain", vanity),
					resource.TestMatchResourceAttr("laravel_cloud_environment.test", "vanity_hostname",
						regexp.MustCompile(`^`+regexp.QuoteMeta(vanity)+`\.[a-z0-9.-]+$`)),
				),
			},
			{
				ResourceName:      "laravel_cloud_environment.test",
				ImportState:       true,
				ImportStateVerify: true,
				// php_version is not configured here; an import rebuilds it
				// from the reported major version.
				ImportStateVerifyIgnore: []string{"application_id", "branch", "php_version"},
			},
		},
	})
}

// TestAccEnvironmentResource_readBack covers color, timeout and
// shutdown_timeout, which the API reports. An unset shutdown_timeout
// takes the API's value, and a change made outside Terraform shows in the next
// plan and is put back.
func TestAccEnvironmentResource_readBack(t *testing.T) {
	appName := testAccName()
	var envID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEnvironmentReadBackConfig(appName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "color", "red"),
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "timeout", "30"),
					// Unset, so it is the platform default rather than null.
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "shutdown_timeout", "30"),
					func(s *terraform.State) error {
						envID = s.RootModule().Resources["laravel_cloud_environment.test"].Primary.ID
						return nil
					},
				),
			},
			// The dashboard changes color and timeout: the plan must notice.
			{
				PreConfig: func() {
					color, timeout := "green", 45
					if _, err := testAccClient().UpdateEnvironment(context.Background(), envID,
						client.UpdateEnvironmentRequest{Color: &color, Timeout: &timeout}); err != nil {
						t.Fatalf("changing the environment outside Terraform: %v", err)
					}
				},
				Config:             testAccEnvironmentReadBackConfig(appName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccEnvironmentReadBackConfig(appName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "color", "red"),
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "timeout", "30"),
				),
			},
			{
				ResourceName:            "laravel_cloud_environment.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"application_id", "branch", "php_version"},
			},
		},
	})
}

// TestAccEnvironmentResource_hibernationRefused moves the app instance to a
// Pro size, which cannot hibernate, under an environment with sleep_timeout.
// The API answers a sleep_timeout update with 200 but turns
// hibernation off, so the plan must show sleep_timeout again and applying it
// must fail; removing it must converge.
func TestAccEnvironmentResource_hibernationRefused(t *testing.T) {
	appName := testAccName()
	var envID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEnvironmentSleepConfig(appName, "sleep_timeout = 10"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_environment.test", "sleep_timeout", "10"),
					func(s *terraform.State) error {
						envID = s.RootModule().Resources["laravel_cloud_environment.test"].Primary.ID
						return nil
					},
				),
			},
			{
				PreConfig:   func() { testAccMoveAppInstanceToPro(t, envID) },
				Config:      testAccEnvironmentSleepConfig(appName, "sleep_timeout = 10"),
				ExpectError: regexp.MustCompile(`turned hibernation off instead of applying sleep_timeout`),
			},
			{
				Config: testAccEnvironmentSleepConfig(appName, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func testAccEnvironmentSleepConfig(appName, sleep string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "staging"
  branch         = %[3]q
  %[4]s
}
`, appName, testAccRepository(), testAccBranch(), sleep)
}

// testAccMoveAppInstanceToPro moves the environment's app instance to a Pro
// size outside Terraform. The API allows that only with hibernation off, which
// takes hibernation_timeout: null -- a null the client's instance request
// cannot send -- so this is a raw request.
func testAccMoveAppInstanceToPro(t *testing.T, envID string) {
	t.Helper()
	ctx := context.Background()
	instances, err := testAccClient().ListInstances(ctx, envID)
	if err != nil {
		t.Fatalf("listing instances: %v", err)
	}
	var appInstance string
	for _, inst := range instances {
		if inst.Attributes.InstanceType == "app" {
			appInstance = inst.ID
		}
	}
	if appInstance == "" {
		t.Fatalf("environment %s has no app instance", envID)
	}
	base := os.Getenv("LARAVEL_CLOUD_BASE_URL")
	if base == "" {
		base = "https://cloud.laravel.com/api"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, base+"/instances/"+appInstance,
		strings.NewReader(`{"size":"pro.m-1vcpu-4gb","hibernation_timeout":null}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("LARAVEL_CLOUD_API_TOKEN"))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("resizing the app instance: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("resizing the app instance: %s: %s", res.Status, body)
	}
}

// TestAccEnvironmentResource_attachmentReadBack covers an attached cache
// detached outside Terraform. The API reports the attachment as an
// included relationship, so the next plan must put it back.
func TestAccEnvironmentResource_attachmentReadBack(t *testing.T) {
	appName := testAccName()
	cacheName := testAccName()
	var envID, cacheID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEnvironmentCacheConfig(appName, cacheName, false),
				Check: func(s *terraform.State) error {
					envID = s.RootModule().Resources["laravel_cloud_environment.test"].Primary.ID
					cacheID = s.RootModule().Resources["laravel_cloud_cache.test"].Primary.ID
					return nil
				},
			},
			// The API refuses to attach a cache that is still provisioning,
			// which a new one is for a while after Create returns.
			{
				PreConfig: func() { testAccWaitForCacheAvailable(t, cacheID) },
				Config:    testAccEnvironmentCacheConfig(appName, cacheName, true),
				Check:     func(*terraform.State) error { return testAccCheckAttachedCache(envID, cacheID) },
			},
			{
				PreConfig: func() {
					if _, err := testAccClient().UpdateEnvironment(context.Background(), envID,
						client.UpdateEnvironmentRequest{CacheID: client.DetachID()}); err != nil {
						t.Fatalf("detaching the cache outside Terraform: %v", err)
					}
				},
				Config:             testAccEnvironmentCacheConfig(appName, cacheName, true),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccEnvironmentCacheConfig(appName, cacheName, true),
				Check:  func(*terraform.State) error { return testAccCheckAttachedCache(envID, cacheID) },
			},
		},
	})
}

func testAccWaitForCacheAvailable(t *testing.T, id string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Minute)
	for {
		cache, err := testAccClient().GetCache(context.Background(), id)
		if err == nil && cache.Attributes.Status == "available" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("cache %s is not available after 15 minutes (last error: %v)", id, err)
		}
		time.Sleep(10 * time.Second)
	}
}

// testAccCheckAttachedCache checks the cache the API reports as attached.
func testAccCheckAttachedCache(envID, want string) error {
	env, err := testAccClient().GetEnvironment(context.Background(), envID)
	if err != nil {
		return err
	}
	if env.Relationships.Cache == nil || env.Relationships.Cache.RelatedID() != want {
		return fmt.Errorf("environment %s reports cache %+v, want %q", envID, env.Relationships.Cache, want)
	}
	return nil
}

func testAccEnvironmentCacheConfig(appName, cacheName string, attach bool) string {
	cacheID := ""
	if attach {
		cacheID = "cache_id = laravel_cloud_cache.test.id"
	}
	return fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}

resource "laravel_cloud_cache" "test" {
  name                 = %[4]q
  type                 = "laravel_valkey"
  region               = "us-east-2"
  size                 = "valkey-pro.250mb"
  auto_upgrade_enabled = true
  is_public            = false
}

resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "staging"
  branch         = %[3]q
  %[5]s
}
`, appName, testAccRepository(), testAccBranch(), cacheName, cacheID)
}

func testAccEnvironmentReadBackConfig(appName string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "staging"
  branch         = %[3]q
  color          = "red"
  timeout        = 30
}
`, appName, testAccRepository(), testAccBranch())
}

func testAccEnvironmentVanityConfig(appName, vanity string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "production"
  branch         = %[3]q
  vanity_domain  = %[4]q
}
`, appName, testAccRepository(), testAccBranch(), vanity)
}

func testAccEnvironmentConfig(appName string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "production"
  branch         = %[3]q
  php_version    = "8.4:1"
  node_version   = "22"

  uses_push_to_deploy = true
  uses_octane         = true
  timeout             = 30
}
`, appName, testAccRepository(), testAccBranch())
}

func testAccEnvironmentConfigUpdated(appName string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "production"
  branch         = %[3]q
  php_version    = "8.4:1"
  node_version   = "22"

  uses_push_to_deploy = true
  uses_octane         = false
  timeout             = 45
}
`, appName, testAccRepository(), testAccBranch())
}
