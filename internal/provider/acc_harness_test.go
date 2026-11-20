package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// Acceptance tests run against a real organization that also holds resources
// nobody should touch. Everything a test creates is named with this prefix, and
// the sweepers below delete only names that start with it, so a sweep can never
// reach anything else in the account.
const testAccPrefix = "tfacc-"

// testAccName returns a unique name for a test resource. Names use only
// lowercase letters, digits and dashes, which every resource type accepts.
func testAccName() string {
	return testAccPrefix + acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
}

// testAccClient returns an API client for checks and out-of-band changes
// that a test makes behind Terraform's back, such as simulating a change the
// backend makes on its own.
func testAccClient() *client.Client {
	var opts []client.ClientOption
	if u := os.Getenv("LARAVEL_CLOUD_BASE_URL"); u != "" {
		opts = append(opts, client.WithBaseURL(u))
	}
	return client.NewClient(os.Getenv("LARAVEL_CLOUD_API_TOKEN"), opts...)
}

// TestMain runs the sweepers when go test is given -sweep, and the tests
// otherwise:
//
//	go test ./internal/provider -v -sweep=all
func TestMain(m *testing.M) {
	resource.TestMain(m)
}

func isTestName(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), testAccPrefix)
}

// sweepEach deletes every item whose name has the test prefix and reports
// all failures together, so one stuck resource does not hide the others.
func sweepEach[T any](items []T, name func(T) string, del func(T) error) error {
	var errs []string
	for _, it := range items {
		if !isTestName(name(it)) {
			continue
		}
		if err := del(it); err != nil && !client.IsNotFound(err) {
			errs = append(errs, fmt.Sprintf("%s: %v", name(it), err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("sweep failed for %d resource(s):\n%s", len(errs), strings.Join(errs, "\n"))
	}
	return nil
}

func init() {
	ctx := context.Background()

	// Applications go first: deleting one deletes its environments,
	// instances and domains, and releases the resources attached to them.
	resource.AddTestSweepers("laravel_cloud_application", &resource.Sweeper{
		Name: "laravel_cloud_application",
		F: func(string) error {
			c := testAccClient()
			apps, err := c.ListApplications(ctx)
			if err != nil {
				return err
			}
			return sweepEach(apps,
				func(a client.ApplicationData) string { return a.Attributes.Name },
				func(a client.ApplicationData) error { return c.DeleteApplication(ctx, a.ID) })
		},
	})

	resource.AddTestSweepers("laravel_cloud_websocket_server", &resource.Sweeper{
		Name:         "laravel_cloud_websocket_server",
		Dependencies: []string{"laravel_cloud_application"},
		F: func(string) error {
			c := testAccClient()
			servers, err := c.ListWebsocketServers(ctx)
			if err != nil {
				return err
			}
			return sweepEach(servers,
				func(s client.WebsocketServerData) string { return s.Attributes.Name },
				func(s client.WebsocketServerData) error { return c.DeleteWebsocketServer(ctx, s.ID) })
		},
	})

	resource.AddTestSweepers("laravel_cloud_cache", &resource.Sweeper{
		Name:         "laravel_cloud_cache",
		Dependencies: []string{"laravel_cloud_application"},
		F: func(string) error {
			c := testAccClient()
			caches, err := c.ListCaches(ctx)
			if err != nil {
				return err
			}
			return sweepEach(caches,
				func(x client.CacheData) string { return x.Attributes.Name },
				func(x client.CacheData) error { return c.DeleteCache(ctx, x.ID) })
		},
	})

	resource.AddTestSweepers("laravel_cloud_storage_bucket", &resource.Sweeper{
		Name:         "laravel_cloud_storage_bucket",
		Dependencies: []string{"laravel_cloud_application"},
		F: func(string) error {
			c := testAccClient()
			buckets, err := c.ListStorageBuckets(ctx)
			if err != nil {
				return err
			}
			return sweepEach(buckets,
				func(b client.StorageBucketData) string { return b.Attributes.Name },
				// The API refuses to delete a bucket that still has keys.
				func(b client.StorageBucketData) error {
					keys, err := c.ListStorageBucketKeys(ctx, b.ID)
					if err != nil {
						return err
					}
					for _, k := range keys {
						if err := c.DeleteStorageBucketKey(ctx, k.ID); err != nil && !client.IsNotFound(err) {
							return err
						}
					}
					return c.DeleteStorageBucket(ctx, b.ID)
				})
		},
	})

	// Database users are named with the prefix in underscores, since
	// usernames may not contain dashes. They are looked for in every cluster,
	// not only test clusters, because a test may add one to any cluster; only
	// prefixed users are deleted. Serverless Postgres has no database users,
	// and the superuser cannot be deleted.
	resource.AddTestSweepers("laravel_cloud_database_user", &resource.Sweeper{
		Name:         "laravel_cloud_database_user",
		Dependencies: []string{"laravel_cloud_application"},
		F: func(string) error {
			c := testAccClient()
			clusters, err := c.ListDatabaseClusters(ctx)
			if err != nil {
				return err
			}
			var errs []string
			for _, d := range clusters {
				if strings.HasPrefix(d.Attributes.DBType, "neon_serverless_postgres") {
					continue
				}
				users, err := c.ListDatabaseUsers(ctx, d.ID)
				if err != nil {
					if !client.IsNotFound(err) {
						errs = append(errs, fmt.Sprintf("%s: %v", d.Attributes.Name, err))
					}
					continue
				}
				err = sweepEach(users,
					func(u client.DatabaseUserData) string {
						if u.Attributes.Type == "superuser" {
							return ""
						}
						return strings.ReplaceAll(u.Attributes.Username, "_", "-")
					},
					func(u client.DatabaseUserData) error { return c.DeleteDatabaseUser(ctx, d.ID, u.ID) })
				if err != nil {
					errs = append(errs, err.Error())
				}
			}
			if len(errs) > 0 {
				return fmt.Errorf("%s", strings.Join(errs, "\n"))
			}
			return nil
		},
	})

	// A cluster cannot be deleted while it still holds databases, so its
	// databases go first. A cluster that is still busy fails here and is
	// picked up by the next sweep.
	resource.AddTestSweepers("laravel_cloud_database_cluster", &resource.Sweeper{
		Name:         "laravel_cloud_database_cluster",
		Dependencies: []string{"laravel_cloud_application", "laravel_cloud_database_user"},
		F: func(string) error {
			c := testAccClient()
			clusters, err := c.ListDatabaseClusters(ctx)
			if err != nil {
				return err
			}
			return sweepEach(clusters,
				func(d client.DatabaseClusterData) string { return d.Attributes.Name },
				func(d client.DatabaseClusterData) error {
					dbs, err := c.ListDatabases(ctx, d.ID)
					if err != nil {
						return err
					}
					for _, db := range dbs {
						if err := c.DeleteDatabase(ctx, d.ID, db.ID); err != nil && !client.IsNotFound(err) {
							return err
						}
					}
					return c.DeleteDatabaseCluster(ctx, d.ID)
				})
		},
	})

	// Secret keys are environment variable names, so the prefix is
	// upper-cased with an underscore.
	resource.AddTestSweepers("laravel_cloud_secret", &resource.Sweeper{
		Name:         "laravel_cloud_secret",
		Dependencies: []string{"laravel_cloud_application"},
		F: func(string) error {
			c := testAccClient()
			secrets, err := c.ListSecrets(ctx)
			if err != nil {
				return err
			}
			return sweepEach(secrets,
				func(s client.SecretData) string {
					return strings.ReplaceAll(strings.ToLower(s.Attributes.Key), "_", "-")
				},
				func(s client.SecretData) error { return c.DeleteSecret(ctx, s.ID) })
		},
	})
}
