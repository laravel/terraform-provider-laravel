package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// Removing slack_channel from the configuration turns Slack
// notifications off. It used to be Optional+Computed, so the old channel stayed
// in state and nothing was sent.
func TestApplicationSlackChannelClearedPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	app := func(extra string) string {
		return providerCfg(baseURL) + `
resource "laravel_cloud_application" "app" {
  name       = "slack-app"
  repository = "acme/site"
  region     = "us-east-2"
  ` + extra + `
}
`
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: app(`slack_channel = "deploys"`),
				Check:  resource.TestCheckResourceAttr("laravel_cloud_application.app", "slack_channel", "deploys"),
			},
			{
				Config: app(""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("laravel_cloud_application.app", "slack_channel"),
					func(*terraform.State) error {
						f.mu.Lock()
						defer f.mu.Unlock()
						for _, a := range f.apps {
							if a.Attributes.SlackChannel != nil {
								return fmt.Errorf("Cloud still has slack_channel %q", *a.Attributes.SlackChannel)
							}
						}
						last := f.appUpdates[len(f.appUpdates)-1]
						if last.SlackChannel == nil || !last.SlackChannel.Null {
							return fmt.Errorf("last update did not send slack_channel: null, sent %+v", last.SlackChannel)
						}
						return nil
					},
				),
			},
		},
	})
}

// A static site: serving and build_command go out with the application create,
// the default environment is static, and the read-only details are recorded.
// A second environment is then made the default.
func TestApplicationStaticSiteAndDefaultEnvironmentPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	cfg := func(defaultEnv string) string {
		return providerCfg(baseURL) + `
resource "laravel_cloud_application" "app" {
  name                   = "static-app"
  repository             = "acme/site"
  region                 = "us-east-2"
  serving_mode           = "static"
  serving_root           = "dist"
  build_command          = "npm run build"
  ` + defaultEnv + `
}

resource "laravel_cloud_environment" "staging" {
  application_id = laravel_cloud_application.app.id
  name           = "staging"
  branch         = "main"
  serving_mode   = "process"
  build_command  = "composer install"
}
`
	}
	const app = "laravel_cloud_application.app"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg(""),
				Check: resource.ComposeAggregateTestCheckFunc(
					// The fake numbers objects in order: app-1, its default
					// environment env-2, then staging env-3.
					resource.TestCheckResourceAttr(app, "default_environment_id", "env-2"),
					resource.TestCheckResourceAttr(app, "framework", "laravel"),
					resource.TestCheckResourceAttr(app, "language", "php"),
					resource.TestCheckResourceAttr(app, "repository_default_branch", "main"),
					resource.TestCheckResourceAttr("laravel_cloud_environment.staging", "serving_mode", "process"),
					resource.TestCheckResourceAttr("laravel_cloud_environment.staging", "display_name", "staging"),
					resource.TestCheckResourceAttr("laravel_cloud_environment.staging", "created_from_automation", "false"),
					resource.TestCheckNoResourceAttr("laravel_cloud_environment.staging", "pull_request"),
					func(*terraform.State) error {
						f.mu.Lock()
						defer f.mu.Unlock()
						req := f.appCreates[0]
						if req.Serving == nil || req.Serving.Mode != "static" || req.Serving.Root.Value != "dist" ||
							req.BuildCommand == nil || *req.BuildCommand != "npm run build" {
							return fmt.Errorf("create sent serving %+v, build_command %v", req.Serving, req.BuildCommand)
						}
						def := f.envs["env-2"].Attributes
						if def.Serving.Mode != "static" || def.BuildCommand == nil || *def.BuildCommand != "npm run build" {
							return fmt.Errorf("default environment serving %+v, build_command %v", def.Serving, def.BuildCommand)
						}
						// Without serving_mode, a new environment of an
						// application with a static one would be static too.
						if mode := f.envs["env-3"].Attributes.Serving.Mode; mode != "process" {
							return fmt.Errorf("staging serving mode = %q, want process", mode)
						}
						return nil
					},
				),
			},
			{
				Config: cfg(`default_environment_id = "env-3"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(app, "default_environment_id", "env-3"),
					func(*terraform.State) error {
						f.mu.Lock()
						defer f.mu.Unlock()
						if got := f.apps["app-1"].Relationships.DefaultEnvironment.RelatedID(); got != "env-3" {
							return fmt.Errorf("Cloud's default environment = %q, want env-3", got)
						}
						return nil
					},
				),
			},
			// Leaving it unset keeps the current default.
			{Config: cfg(""), PlanOnly: true},
		},
	})
}

// post_deploy_command is read back, cleared with null, and fails the apply
// when the organization does not have post-deploy commands, because Laravel
// Cloud then ignores it and still answers 200.
func TestEnvironmentPostDeployCommandPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	cfg := func(extra string) string {
		return providerCfg(baseURL) + `
resource "laravel_cloud_application" "app" {
  name       = "pdc-app"
  repository = "acme/site"
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "staging" {
  application_id = laravel_cloud_application.app.id
  name           = "staging"
  branch         = "main"
  ` + extra + `
}
`
	}
	const env = "laravel_cloud_environment.staging"
	f.postDeploy = true
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg(`post_deploy_command = "php artisan queue:restart"`),
				Check:  resource.TestCheckResourceAttr(env, "post_deploy_command", "php artisan queue:restart"),
			},
			// Changed in the dashboard: the plan puts it back.
			{
				PreConfig: func() {
					f.mu.Lock()
					defer f.mu.Unlock()
					f.envs["env-3"].Attributes.PostDeployCommand.Value = ptr("php artisan about")
				},
				Config: cfg(`post_deploy_command = "php artisan queue:restart"`),
				Check: func(*terraform.State) error {
					f.mu.Lock()
					defer f.mu.Unlock()
					if got := f.envs["env-3"].Attributes.PostDeployCommand.Value; got == nil || *got != "php artisan queue:restart" {
						return fmt.Errorf("Cloud's post_deploy_command = %v after apply", got)
					}
					return nil
				},
			},
			// Without the feature, Cloud ignores it and the apply says so.
			{
				PreConfig: func() {
					f.mu.Lock()
					defer f.mu.Unlock()
					f.envs["env-3"].Attributes.PostDeployCommand = client.ReportedString{}
				},
				Config:      cfg(`post_deploy_command = "php artisan queue:restart"`),
				ExpectError: regexp.MustCompile(`Laravel Cloud did not apply post_deploy_command`),
			},
		},
	})
}
