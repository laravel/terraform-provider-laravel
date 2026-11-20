package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func applicationConfig(baseURL, extra string) string {
	return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_application" "app" {
  name                         = "monorepo-app"
  repository                   = "acme/monorepo"
  region                       = "us-east-2"
  source_control_provider_type = "github"
%s
}
`, extra)
}

// TestApplicationRootDirectoryPlan covers root_directory: sent on create, read
// back, replaced when changed -- and adopted rather than replaced when it was
// set outside Terraform and the configuration does not mention it, which is
// what an existing application looks like after upgrading the provider.
func TestApplicationRootDirectoryPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_application.app"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: applicationConfig(baseURL, `  root_directory = "apps/web"`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("root_directory"), knownvalue.StringExact("apps/web")),
				},
			},
			{
				Config:           applicationConfig(baseURL, `  root_directory = "apps/web"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: applicationConfig(baseURL, `  root_directory = "apps/api"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionDestroyBeforeCreate),
				}},
			},
			{
				// Unset in config: the reported directory is kept, not replaced.
				Config:           applicationConfig(baseURL, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("root_directory"), knownvalue.StringExact("apps/api")),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"source_control_provider_type"},
			},
		},
	})
}

// TestApplicationStarterKitPlan sends starter_kit on create only. The API never
// reports it, so the configured value is what state keeps.
func TestApplicationStarterKitPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_application.app"
	config := applicationConfig(baseURL, `  starter_kit = "laravel-react"`)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("starter_kit"), knownvalue.StringExact("laravel-react")),
				},
				Check: func(*terraform.State) error {
					f.mu.Lock()
					defer f.mu.Unlock()
					last := f.appCreates[len(f.appCreates)-1]
					if last.StarterKit == nil || *last.StarterKit != "laravel-react" {
						return fmt.Errorf("starter_kit was not sent on create: %+v", last)
					}
					return nil
				},
			},
			{
				Config:           config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// TestApplicationStarterKitConflictPlan rejects a starter kit combined with a root
// directory, which the API refuses.
func TestApplicationStarterKitConflictPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      applicationConfig(baseURL, "  starter_kit = \"laravel\"\n  root_directory = \"apps/web\""),
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
		},
	})
}

func bucketConfig(baseURL, extra string) string {
	return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_storage_bucket" "bucket" {
  name     = "assets"
  key_name = "primary"
%s
}
`, extra)
}

// TestStorageBucketLocationHintPlan reads the platform's default when unset
// and forces a new bucket when the hint changes.
func TestStorageBucketLocationHintPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_storage_bucket.bucket"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: bucketConfig(baseURL, ""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("location_hint"), knownvalue.StringExact("automatic")),
				},
			},
			{
				Config: bucketConfig(baseURL, `  location_hint = "weur"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionDestroyBeforeCreate),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("location_hint"), knownvalue.StringExact("weur")),
				},
			},
			{
				Config:           bucketConfig(baseURL, `  location_hint = "weur"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// TestDatabaseClusterWithoutDefaultDatabasePlan creates a cluster without the
// database the platform would otherwise put in it, which also means it can be
// destroyed without force_destroy.
func TestDatabaseClusterWithoutDefaultDatabasePlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_database_cluster.cluster"
	config := providerCfg(baseURL) + `
resource "laravel_cloud_database_cluster" "cluster" {
  name                    = "empty-db"
  type                    = "laravel_mysql_84"
  region                  = "us-east-1"
  config                  = jsonencode({ size = "db-1vcpu-1gb" })
  create_default_database = false
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("create_default_database"), knownvalue.Bool(false)),
				},
				Check: func(s *terraform.State) error {
					id := s.RootModule().Resources[addr].Primary.ID
					f.mu.Lock()
					defer f.mu.Unlock()
					if dbs := f.children["database:"+id]; len(dbs) != 0 {
						return fmt.Errorf("the cluster got a default database: %v", dbs)
					}
					return nil
				},
			},
			{
				Config:           config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

func cacheConfig(baseURL, extra string) string {
	return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_cache" "cache" {
  name                 = "sessions"
  type                 = "aws_elasticache_valkey"
  region               = "us-east-2"
  size                 = "1gb"
  auto_upgrade_enabled = false
%s
}
`, extra)
}

// TestCacheElastiCacheIsPublicPlan covers is_public for ElastiCache:
// the API stores false whatever create sends and refuses to change it, so
// leaving it out applies and re-plans cleanly, true is refused at plan time
// instead of drifting, and types other than ElastiCache still require it.
func TestCacheElastiCacheIsPublicPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_cache.cache"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      cacheConfig(baseURL, `  is_public = true`),
				ExpectError: regexp.MustCompile(`is_public must be false for ElastiCache caches`),
			},
			{
				Config:      strings.Replace(valkeyFlexConfig(baseURL, "sessions", ""), "  is_public            = false\n", "", 1),
				ExpectError: regexp.MustCompile(`is_public is required for laravel_valkey caches`),
			},
			{
				Config: cacheConfig(baseURL, ""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("is_public"), knownvalue.Bool(false)),
				},
			},
			{
				Config:           cacheConfig(baseURL, `  is_public = false`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// TestCacheClusterIDPlan covers cluster_id: sent on create, stable on re-plan
// although the API never reports it back, and replaced when changed.
func TestCacheClusterIDPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_cache.cache"

	sentClusterID := func(want string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			for _, attrs := range f.objects["cache"] {
				if attrs["cluster_id"] == want {
					return nil
				}
			}
			return fmt.Errorf("no cache was created with cluster_id %q", want)
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cacheConfig(baseURL, `  cluster_id = "dc-1"`),
				Check:  sentClusterID("dc-1"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("cluster_id"), knownvalue.StringExact("dc-1")),
				},
			},
			{
				Config:           cacheConfig(baseURL, `  cluster_id = "dc-1"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: cacheConfig(baseURL, `  cluster_id = "dc-2"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionDestroyBeforeCreate),
				}},
				Check: sentClusterID("dc-2"),
			},
		},
	})
}

// TestApplicationRootDirectoryClearedPlan covers Laravel Cloud clearing
// root_directory when the repository changes to one without that directory.
// Unset in the configuration, the cleared value is read back; still
// configured, the apply fails and the next plan refuses to replace the
// application, which would delete its environments.
func TestApplicationRootDirectoryClearedPlan(t *testing.T) {
	addr := "laravel_cloud_application.app"
	withRepo := func(baseURL, repo, extra string) string {
		return strings.Replace(applicationConfig(baseURL, extra), `"acme/monorepo"`, fmt.Sprintf("%q", repo), 1)
	}

	t.Run("unset", func(t *testing.T) {
		_, baseURL := newFakeCloud(t)
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{Config: applicationConfig(baseURL, `  root_directory = "apps/web"`)},
				{
					Config: withRepo(baseURL, "acme/site", ""),
					ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
						plancheck.ExpectUnknownValue(addr, tfjsonpath.New("root_directory")),
					}},
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue(addr, tfjsonpath.New("root_directory"), knownvalue.Null()),
					},
				},
			},
		})
	})

	t.Run("configured", func(t *testing.T) {
		_, baseURL := newFakeCloud(t)
		cleared := regexp.MustCompile(`Laravel Cloud cleared root_directory`)
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{Config: applicationConfig(baseURL, `  root_directory = "apps/web"`)},
				{
					Config:      withRepo(baseURL, "acme/site", `  root_directory = "apps/web"`),
					ExpectError: cleared,
				},
				{
					// Planning again fails too, instead of replacing.
					Config:      withRepo(baseURL, "acme/site", `  root_directory = "apps/web"`),
					PlanOnly:    true,
					ExpectError: cleared,
				},
				{
					Config:           withRepo(baseURL, "acme/site", ""),
					ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue(addr, tfjsonpath.New("root_directory"), knownvalue.Null()),
					},
				},
				{
					// Any other directory is a new application, as before.
					Config: withRepo(baseURL, "acme/site", `  root_directory = "apps/api"`),
					ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionDestroyBeforeCreate),
					}},
				},
			},
		})
	})
}

// TestSlugPlan sends a configured slug for an application and an environment,
// both on create, which takes none, and on update.
func TestSlugPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	config := func(app, env string) string {
		return applicationConfig(baseURL, fmt.Sprintf("  slug = %q", app)) + fmt.Sprintf(`
resource "laravel_cloud_environment" "env" {
  application_id = laravel_cloud_application.app.id
  name           = "staging"
  branch         = "main"
  slug           = %q
}
`, env)
	}
	check := func(app, env string) []statecheck.StateCheck {
		return []statecheck.StateCheck{
			statecheck.ExpectKnownValue("laravel_cloud_application.app", tfjsonpath.New("slug"), knownvalue.StringExact(app)),
			statecheck.ExpectKnownValue("laravel_cloud_environment.env", tfjsonpath.New("slug"), knownvalue.StringExact(env)),
		}
	}
	remote := func(app, env string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			for _, a := range f.apps {
				if a.Attributes.Slug != app {
					return fmt.Errorf("API application slug = %q, want %q", a.Attributes.Slug, app)
				}
			}
			for _, e := range f.envs {
				if e.Attributes.Name == "staging" && e.Attributes.Slug != env {
					return fmt.Errorf("API environment slug = %q, want %q", e.Attributes.Slug, env)
				}
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config("acme-web", "stage"), ConfigStateChecks: check("acme-web", "stage"), Check: remote("acme-web", "stage")},
			{Config: config("acme-www", "staging-2"), ConfigStateChecks: check("acme-www", "staging-2"), Check: remote("acme-www", "staging-2")},
		},
	})
}
