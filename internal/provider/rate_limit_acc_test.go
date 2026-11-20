package provider

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccProvider_pacedReads checks against the live API that
// max_requests_per_minute spaces requests out. It reads the
// organization 15 times at 30 requests a minute: after a burst of 3, the
// other 12 wait 2 seconds each, so the plan alone takes at least 24 seconds.
//
// It creates nothing and keeps well under the organization's 120 requests a
// minute, so it is safe to run while other tests share the organization. It
// does not burst past the limit to show a 429 being retried: that would
// throttle every other client of the organization.
func TestAccProvider_pacedReads(t *testing.T) {
	var start time.Time
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() { start = time.Now() },
				Config: `
provider "laravel" {
  max_requests_per_minute = 30
}

data "laravel_cloud_organization" "paced" {
  count = 15
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.laravel_cloud_organization.paced.14", "name"),
					func(*terraform.State) error {
						if elapsed := time.Since(start); elapsed < 20*time.Second {
							return fmt.Errorf("15 reads at 30 a minute took %v; they were not paced", elapsed)
						}
						return nil
					},
				),
			},
		},
	})
}
