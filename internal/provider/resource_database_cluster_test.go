package provider

// The database cluster acceptance test is commented out on purpose.
//
// Provisioning a cluster routinely takes twenty minutes or more, and neither
// the databases inside it nor the cluster itself can be deleted until it
// finishes. A create-then-destroy cycle therefore either blocks the run for a
// very long time or gives up and leaves a billable database behind -- which is
// where the leftover clusters in the test organization came from.
//
// The resource's behaviour stays covered by the plan tests in
// database_conformance_plan_test.go, which exercise the same code paths
// against the in-memory fake in under a second: the create/update requests,
// the retired-versus-current type round-trip, the version field, and the
// config reconciliation.
//
// Uncomment to run it against a real API, and expect it to be slow.
//
// func TestAccDatabaseClusterResource_basic(t *testing.T) {
// 	rName := fmt.Sprintf("tf-test-%s", acctest.RandString(8))
//
// 	resource.Test(t, resource.TestCase{
// 		PreCheck:                 func() { testAccPreCheck(t) },
// 		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
// 		Steps: []resource.TestStep{
// 			// Create and Read
// 			{
// 				Config: testAccDatabaseClusterConfig(rName),
// 				Check: resource.ComposeAggregateTestCheckFunc(
// 					resource.TestCheckResourceAttr("laravel_cloud_database_cluster.test", "name", rName),
// 					resource.TestCheckResourceAttrSet("laravel_cloud_database_cluster.test", "id"),
// 					resource.TestCheckResourceAttr("laravel_cloud_database_cluster.test", "type", "laravel_mysql"),
// 					resource.TestCheckResourceAttr("laravel_cloud_database_cluster.test", "region", "us-east-2"),
// 					resource.TestCheckResourceAttrSet("laravel_cloud_database_cluster.test", "status"),
// 				),
// 			},
// 			// Import
// 			{
// 				ResourceName:      "laravel_cloud_database_cluster.test",
// 				ImportState:       true,
// 				ImportStateVerify: true,
// 				// version is a create-only request field: the API accepts it and
// 				// never reports it back, so an imported cluster cannot recover
// 				// it. config is ignored because an imported cluster has no
// 				// configured JSON to preserve, so it takes the API's full
// 				// effective configuration including defaults the original
// 				// config never set.
// 				ImportStateVerifyIgnore: []string{"version", "config"},
// 			},
// 		},
// 	})
// }
//
// func testAccDatabaseClusterConfig(name string) string {
// 	return fmt.Sprintf(`
// resource "laravel_cloud_database_cluster" "test" {
//   name   = %[1]q
//   type    = "laravel_mysql"
//   version = "8.4"
//   region  = "us-east-2"
//   config = jsonencode({
//     size                     = "mysql-flex-512mb"
//     storage                  = 10
//     is_public                = false
//     uses_scheduled_snapshots = false
//     retention_days           = 0
//   })
// }
// `, name)
// }
