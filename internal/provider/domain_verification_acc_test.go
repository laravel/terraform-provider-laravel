package provider

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// wait_for_verification asks Cloud to check the domain's records once
// a minute until it verifies. Nothing creates this domain's records, so it
// cannot verify: the apply waits out verification_timeout, keeps the domain
// with its pending statuses, and the next plan is empty.
func TestAccDomain_waitForVerification(t *testing.T) {
	name := testAccName()
	// Cloud answers a domain under example.com with a 302 instead of creating
	// it, so the test needs a real zone. Nothing verifies records under it.
	zone := testAccFixture(t, "LARAVEL_CLOUD_TEST_DOMAIN_ZONE")
	const addr = "laravel_cloud_domain.test"
	var started time.Time

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() { started = time.Now() },
				Config:    testAccDomainWaitConfig(name, zone),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "hostname_status", "pending"),
					resource.TestCheckResourceAttrSet(addr, "action_required"),
					// One check right away and one a minute later.
					func(*terraform.State) error {
						if d := time.Since(started); d < time.Minute {
							return fmt.Errorf("apply took %s; it should have waited a minute for the domain to verify", d)
						}
						return nil
					},
				),
			},
			{
				Config:           testAccDomainWaitConfig(name, zone),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

func testAccDomainWaitConfig(name, zone string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}

# Not "production": the API refuses to delete an application's production
# environment.
resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "staging"
  branch         = %[3]q
}

resource "laravel_cloud_domain" "test" {
  environment_id        = laravel_cloud_environment.test.id
  name                  = "%[1]s.%[4]s"
  wait_for_verification = true
  verification_timeout  = 1
}
`, name, testAccRepository(), testAccBranch(), zone)
}
