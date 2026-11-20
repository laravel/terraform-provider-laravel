package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// A WebSocket server and two applications on it, created in one apply
// and destroyed in one apply.
//
// The API refuses to change a server while it is busy -- still being created,
// or updating after an application on it was created or deleted -- with a 422
// the provider has to recognise and retry. The two sibling applications are
// created in parallel, so the second one always finds the server busy with the
// first.
func TestAccWebsocketApplication_createWithServer(t *testing.T) {
	name := testAccName()
	var serverID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			_, err := testAccClient().GetWebsocketServer(context.Background(), serverID)
			if client.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("websocket server %s still exists after destroy (err: %v)", serverID, err)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccWebsocketServerWithAppsConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_websocket_server.test", "status", "available"),
					resource.TestCheckResourceAttrSet("laravel_cloud_websocket_application.one", "key"),
					resource.TestCheckResourceAttrSet("laravel_cloud_websocket_application.two", "key"),
					func(s *terraform.State) error {
						serverID = s.RootModule().Resources["laravel_cloud_websocket_server.test"].Primary.ID
						return nil
					},
				),
			},
		},
	})
}

func testAccWebsocketServerWithAppsConfig(name string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_websocket_server" "test" {
  name            = %[1]q
  type            = "reverb"
  region          = "us-east-2"
  max_connections = 100
}

resource "laravel_cloud_websocket_application" "one" {
  server_id = laravel_cloud_websocket_server.test.id
  name      = "one"
}

resource "laravel_cloud_websocket_application" "two" {
  server_id = laravel_cloud_websocket_server.test.id
  name      = "two"
}
`, name)
}
