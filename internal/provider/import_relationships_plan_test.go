package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The three tests below all guard the same defect.
//
// An instance, domain and background process each belong to a parent, and each
// is imported by its own id alone -- the parent id is not in the import string.
// The provider never parsed the JSON:API `relationships` block, so after import
// environment_id / instance_id stayed empty. Since all three carry
// RequiresReplace, the very next plan proposed destroying the resource that had
// just been imported.
//
// ImportStateVerify compares the imported state against the state produced by
// the normal create path, so an empty parent id fails these outright.

func TestInstanceImportRecoversEnvironmentPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := providerCfg(baseURL) + appEnvStack + `
resource "laravel_cloud_instance" "instance" {
  environment_id = laravel_cloud_environment.env.id
  name           = "web"
  size           = "flex.c-1vcpu-256mb"
  scaling_type   = "custom"
  min_replicas   = 1
  max_replicas   = 1
}
`
	runImportRecoversParent(t, config, "laravel_cloud_instance.instance")
}

func TestDomainImportRecoversEnvironmentPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := providerCfg(baseURL) + appEnvStack + `
resource "laravel_cloud_domain" "domain" {
  environment_id = laravel_cloud_environment.env.id
  name           = "example.com"
}
`
	runImportRecoversParent(t, config, "laravel_cloud_domain.domain")
}

func TestBackgroundProcessImportRecoversInstancePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := providerCfg(baseURL) + appEnvStack + `
resource "laravel_cloud_instance" "instance" {
  environment_id = laravel_cloud_environment.env.id
  name           = "worker-host"
  size           = "flex.c-1vcpu-256mb"
  scaling_type   = "custom"
  min_replicas   = 1
  max_replicas   = 1
}

resource "laravel_cloud_background_process" "worker" {
  instance_id = laravel_cloud_instance.instance.id
  type        = "worker"
  processes   = 1
}
`
	runImportRecoversParent(t, config, "laravel_cloud_background_process.worker")
}

// runImportRecoversParent applies the config, then imports the named resource
// by its bare id and requires the resulting state to match -- which it only can
// if the parent id was read back from the relationship.
func runImportRecoversParent(t *testing.T, config, addr string) {
	t.Helper()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				Config:            config,
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// The tests below guard the same defect on the resources the original sweep
// missed. Each is imported by its own id alone, carries a RequiresReplace
// attribute that the API does not report back, and so proposed destroying the
// just-imported resource on the next plan.

func TestEnvironmentImportRecoversApplicationPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := providerCfg(baseURL) + appEnvStack
	runImportRecoversParent(t, config, "laravel_cloud_environment.env")
}

func TestDatabaseSnapshotImportRecoversClusterPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := providerCfg(baseURL) + `
resource "laravel_cloud_database_cluster" "cluster" {
  name    = "snap-cluster"
  type    = "laravel_mysql"
  version = "8.4"
  region  = "us-east-2"

  force_destroy = true

  config = jsonencode({
    size                     = "mysql-flex-512mb"
    storage                  = 10
    is_public                = false
    uses_scheduled_snapshots = false
    retention_days           = 7
    suspend_seconds          = 0
  })
}

resource "laravel_cloud_database_snapshot" "snap" {
  cluster_id = laravel_cloud_database_cluster.cluster.id
  name       = "nightly"
}
`
	runImportRecoversParent(t, config, "laravel_cloud_database_snapshot.snap")
}

func TestCommandImportRecoversEnvironmentPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := providerCfg(baseURL) + appEnvStack + `
resource "laravel_cloud_command" "cmd" {
  environment_id = laravel_cloud_environment.env.id
  command        = "php artisan migrate --force"
}
`
	runImportRecoversParent(t, config, "laravel_cloud_command.cmd")
}

func TestDeploymentImportRecoversEnvironmentPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := providerCfg(baseURL) + appEnvStack + `
resource "laravel_cloud_deployment" "deploy" {
  environment_id = laravel_cloud_environment.env.id
}
`
	runImportRecoversParent(t, config, "laravel_cloud_deployment.deploy")
}
