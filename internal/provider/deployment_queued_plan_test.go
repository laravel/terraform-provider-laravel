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

// While an environment's resources are provisioning, the API queues a
// deployment: 202 with a queued deployment (qdepl-...) that GET /deployments
// does not know. Storing that id made the next read a 404, so the deployment
// dropped out of state and the next apply deployed again. Create now waits for
// the queued deployment to start and stores the deployment it started, and
// Read follows a queued id that an older provider stored.
func TestDeploymentQueuedPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	f.queueDeployments = 1
	addr := "laravel_cloud_deployment.deploy"
	config := providerCfg(baseURL) + appEnvStack + `
resource "laravel_cloud_deployment" "deploy" {
  environment_id = laravel_cloud_environment.env.id
}
`
	deployID := regexp.MustCompile(`^deploy-\d+$`)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("id"), knownvalue.StringRegexp(deployID)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("status"), knownvalue.StringExact("pending")),
				},
			},
			{
				Config:           config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				Check: func(*terraform.State) error {
					if n := len(f.objects["deployment"]); n != 1 {
						return fmt.Errorf("want 1 deployment, the API has %d", n)
					}
					return nil
				},
			},
			{
				// A queued deployment id, as provider 1.4.0 stored it, reads
				// as the deployment it started.
				ResourceName: addr,
				ImportState:  true,
				ImportStateIdFunc: func(*terraform.State) (string, error) {
					f.mu.Lock()
					defer f.mu.Unlock()
					for _, envID := range f.parents["deployment"] {
						id := f.addQueuedDeployment(envID)
						// Long since started.
						f.queued[id].pendingReads = 0
						return id, nil
					}
					return "", fmt.Errorf("no environment")
				},
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("want 1 imported state, got %d", len(states))
					}
					s := states[0]
					if !deployID.MatchString(s.ID) || s.Attributes["environment_id"] == "" {
						return fmt.Errorf("imported id %q, environment_id %q: want the started deployment and its environment", s.ID, s.Attributes["environment_id"])
					}
					return nil
				},
			},
		},
	})
}
