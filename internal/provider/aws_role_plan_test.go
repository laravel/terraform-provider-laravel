package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const (
	awsRoleA = "arn:aws:iam::123456789012:role/app-a"
	awsRoleB = "arn:aws:iam::123456789012:role/app-b"
	awsRoleC = "arn:aws:iam::123456789012:role/set-elsewhere"
)

func awsRoleConfig(baseURL, cluster, role string) string {
	env := ""
	if cluster != "" {
		env += fmt.Sprintf("  cluster_id   = %q\n", cluster)
	}
	if role != "" {
		env += fmt.Sprintf("  aws_role_arn = %q\n", role)
	}
	return providerCfg(baseURL) + `
resource "laravel_cloud_application" "app" {
  name       = "role-app"
  repository = "acme/role-app"
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "worker" {
  application_id = laravel_cloud_application.app.id
  name           = "worker"
  branch         = "main"
` + env + `}
`
}

// fakeWorkerRole is the AWS role the fake holds for the worker environment,
// and setFakeWorkerRole changes it as the dashboard would.
func fakeWorkerRole(f *fakeCloud) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.envs {
		if e.Attributes.Name == "worker" {
			return ptrValue(e.Attributes.AwsRoleArn.Value)
		}
	}
	return ""
}

func setFakeWorkerRole(f *fakeCloud, role *string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.envs {
		if e.Attributes.Name == "worker" {
			e.Attributes.AwsRoleArn.Value = role
		}
	}
}

func workerRoleIs(f *fakeCloud, want string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if got := fakeWorkerRole(f); got != want {
			return fmt.Errorf("worker AWS role on the platform = %q, want %q", got, want)
		}
		return nil
	}
}

// TestEnvironmentAwsRolePlan: environment create does not take the role, so
// it follows in the update after it. A role changed or removed outside
// Terraform shows in the next plan, and removing it from the configuration
// removes it from the environment.
func TestEnvironmentAwsRolePlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	f.byoiam = true
	const addr = "laravel_cloud_environment.worker"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: awsRoleConfig(baseURL, "dc-1", awsRoleA),
				Check: resource.ComposeAggregateTestCheckFunc(
					workerRoleIs(f, awsRoleA),
					func(s *terraform.State) error {
						id := s.RootModule().Resources[addr].Primary.ID
						return resource.TestCheckResourceAttr(addr, "aws_trust_policy", fakeTrustPolicy(id))(s)
					},
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("aws_role_arn"), knownvalue.StringExact(awsRoleA)),
				},
			},
			{
				Config:           awsRoleConfig(baseURL, "dc-1", awsRoleA),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: awsRoleConfig(baseURL, "dc-1", awsRoleB),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
				Check: workerRoleIs(f, awsRoleB),
			},
			{
				// Changed in the dashboard: the plan sets it back.
				PreConfig: func() { setFakeWorkerRole(f, ptr(awsRoleC)) },
				Config:    awsRoleConfig(baseURL, "dc-1", awsRoleB),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
				Check: workerRoleIs(f, awsRoleB),
			},
			{
				// Removed in the dashboard: the plan sets it again.
				PreConfig: func() { setFakeWorkerRole(f, nil) },
				Config:    awsRoleConfig(baseURL, "dc-1", awsRoleB),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
				Check: workerRoleIs(f, awsRoleB),
			},
			{
				// Not reported, as once the organization loses bring your own
				// IAM role: the recorded role stands.
				PreConfig: func() {
					f.mu.Lock()
					f.byoiam = false
					f.mu.Unlock()
				},
				Config:           awsRoleConfig(baseURL, "dc-1", awsRoleB),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("aws_role_arn"), knownvalue.StringExact(awsRoleB)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("aws_trust_policy"), knownvalue.Null()),
				},
			},
			{
				PreConfig: func() {
					f.mu.Lock()
					f.byoiam = true
					f.mu.Unlock()
				},
				Config: awsRoleConfig(baseURL, "dc-1", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
				Check: workerRoleIs(f, ""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("aws_role_arn"), knownvalue.Null()),
				},
			},
			{
				// Left unset, a role set in the dashboard is not managed, so
				// nothing plans to remove it.
				PreConfig:        func() { setFakeWorkerRole(f, ptr(awsRoleC)) },
				Config:           awsRoleConfig(baseURL, "dc-1", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				Check:            workerRoleIs(f, awsRoleC),
			},
			{
				// An import does not adopt the role either; the trust policy is
				// read back.
				Config:                  awsRoleConfig(baseURL, "dc-1", ""),
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"php_version"},
			},
		},
	})
}

// TestEnvironmentAwsRoleRefusedPlan: Laravel Cloud refuses a role without
// bring your own IAM role, and on shared compute. On create the environment
// exists by then, so the apply warns, and the next plan sends the role again
// rather than recording one the platform never took.
func TestEnvironmentAwsRoleRefusedPlan(t *testing.T) {
	const addr = "laravel_cloud_environment.worker"

	t.Run("not enabled", func(t *testing.T) {
		f, baseURL := newFakeCloud(t)
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					Config:             awsRoleConfig(baseURL, "dc-1", awsRoleA),
					Check:              workerRoleIs(f, ""),
					ExpectNonEmptyPlan: true,
				},
				{
					Config:      awsRoleConfig(baseURL, "dc-1", awsRoleA),
					ExpectError: regexp.MustCompile(`needs\s+bring\s+your\s+own\s+IAM\s+role`),
				},
			},
		})
	})

	t.Run("shared compute", func(t *testing.T) {
		f, baseURL := newFakeCloud(t)
		f.byoiam = true
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					Config: awsRoleConfig(baseURL, "", ""),
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue(addr, tfjsonpath.New("aws_trust_policy"), knownvalue.Null()),
					},
				},
				{
					Config:      awsRoleConfig(baseURL, "", awsRoleA),
					ExpectError: regexp.MustCompile(`only\s+for\s+an\s+environment\s+on\s+a\s+dedicated\s+cluster`),
				},
			},
		})
	})

	t.Run("not an ARN", func(t *testing.T) {
		_, baseURL := newFakeCloud(t)
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					Config:      awsRoleConfig(baseURL, "dc-1", "arn:aws:iam::123456789012:user/app"),
					PlanOnly:    true,
					ExpectError: regexp.MustCompile(`must\s+be\s+an\s+IAM\s+role\s+ARN`),
				},
			},
		})
	})
}
