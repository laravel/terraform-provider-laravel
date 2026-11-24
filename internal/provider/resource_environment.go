package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                = &EnvironmentResource{}
	_ resource.ResourceWithImportState = &EnvironmentResource{}
	_ resource.ResourceWithModifyPlan  = &EnvironmentResource{}
)

type EnvironmentResource struct {
	client *client.Client
}

type EnvironmentResourceModel struct {
	ID                         types.String `tfsdk:"id"`
	ApplicationID              types.String `tfsdk:"application_id"`
	Name                       types.String `tfsdk:"name"`
	Branch                     types.String `tfsdk:"branch"`
	Slug                       types.String `tfsdk:"slug"`
	Status                     types.String `tfsdk:"status"`
	Running                    types.Bool   `tfsdk:"running"`
	Color                      types.String `tfsdk:"color"`
	ClusterID                  types.String `tfsdk:"cluster_id"`
	PHPVersion                 types.String `tfsdk:"php_version"`
	PHPMajorVersion            types.String `tfsdk:"php_major_version"`
	VanityDomain               types.String `tfsdk:"vanity_domain"`
	VanityHostname             types.String `tfsdk:"vanity_hostname"`
	NodeVersion                types.String `tfsdk:"node_version"`
	BuildCommand               types.String `tfsdk:"build_command"`
	DeployCommand              types.String `tfsdk:"deploy_command"`
	UsesPushToDeploy           types.Bool   `tfsdk:"uses_push_to_deploy"`
	UsesDeployHook             types.Bool   `tfsdk:"uses_deploy_hook"`
	DeployHookURL              types.String `tfsdk:"deploy_hook_url"`
	UsesVanityDomain           types.Bool   `tfsdk:"uses_vanity_domain"`
	UsesOctane                 types.Bool   `tfsdk:"uses_octane"`
	UsesPurgeEdgeCacheOnDeploy types.Bool   `tfsdk:"uses_purge_edge_cache_on_deploy"`
	Timeout                    types.Int64  `tfsdk:"timeout"`
	SleepTimeout               types.Int64  `tfsdk:"sleep_timeout"`
	ShutdownTimeout            types.Int64  `tfsdk:"shutdown_timeout"`
	CacheStrategy              types.String `tfsdk:"cache_strategy"`
	DatabaseID                 types.String `tfsdk:"database_id"`
	DatabaseSchemaID           types.String `tfsdk:"database_schema_id"`
	CacheID                    types.String `tfsdk:"cache_id"`
	WebsocketApplicationID     types.String `tfsdk:"websocket_application_id"`
	CreatedAt                  types.String `tfsdk:"created_at"`

	EdgeNetworkID                        types.String         `tfsdk:"edge_network_id"`
	ServingMode                          types.String         `tfsdk:"serving_mode"`
	ServingRoot                          types.String         `tfsdk:"serving_root"`
	StartCommand                         types.String         `tfsdk:"start_command"`
	HibernationWakeUpInterval            types.Int64          `tfsdk:"hibernation_wake_up_interval"`
	NightwatchToken                      types.String         `tfsdk:"nightwatch_token"`
	NightwatchTokenWO                    types.String         `tfsdk:"nightwatch_token_wo"`
	NightwatchTokenWOVersion             types.Int64          `tfsdk:"nightwatch_token_wo_version"`
	FilesystemKeys                       []FilesystemKeyModel `tfsdk:"filesystem_keys"`
	ResponseHeadersFrame                 types.String         `tfsdk:"response_headers_frame"`
	ResponseHeadersContentType           types.String         `tfsdk:"response_headers_content_type"`
	ResponseHeadersRobotsTag             types.String         `tfsdk:"response_headers_robots_tag"`
	ResponseHeadersHSTSMaxAge            types.Int64          `tfsdk:"response_headers_hsts_max_age"`
	ResponseHeadersHSTSIncludeSubdomains types.Bool           `tfsdk:"response_headers_hsts_include_subdomains"`
	ResponseHeadersHSTSPreload           types.Bool           `tfsdk:"response_headers_hsts_preload"`
	FirewallBlockPath                    types.Bool           `tfsdk:"firewall_block_path"`
	FirewallBrowserIntegrityCheck        types.Bool           `tfsdk:"firewall_browser_integrity_check"`

	PostDeployCommand     types.String `tfsdk:"post_deploy_command"`
	DisplayName           types.String `tfsdk:"display_name"`
	Framework             types.String `tfsdk:"framework"`
	Language              types.String `tfsdk:"language"`
	UsesHibernation       types.Bool   `tfsdk:"uses_hibernation"`
	CreatedFromAutomation types.Bool   `tfsdk:"created_from_automation"`
	PullRequest           types.Object `tfsdk:"pull_request"`
}

// FilesystemKeyModel attaches a bucket to the environment through one of its
// access keys.
type FilesystemKeyModel struct {
	KeyID         types.String `tfsdk:"key_id"`
	Disk          types.String `tfsdk:"disk"`
	IsDefaultDisk types.Bool   `tfsdk:"is_default_disk"`
}

func NewEnvironmentResource() resource.Resource {
	return &EnvironmentResource{}
}

func (r *EnvironmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_environment"
}

func (r *EnvironmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Laravel Cloud environment.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Required:    true,
				Description: "Parent application ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Environment name (1-40 characters).",
			},
			"branch": schema.StringAttribute{
				Required:    true,
				Description: "Git branch.",
			},
			"slug": schema.StringAttribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Current environment status.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"running": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Whether the environment is running. Set false to stop it, which cancels a deployment in " +
					"progress, and true to start it again, which deploys the latest commit; Terraform waits for that " +
					"deployment to succeed. Only an environment that has been deployed can be stopped. Leave unset to " +
					"leave it as it is.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"color": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Environment color (blue, green, orange, purple, red, yellow, cyan, gray). " +
					"Leave unset to keep the color Laravel Cloud picked when it created the environment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cluster_id": schema.StringAttribute{
				Optional: true,
				Description: "Dedicated cluster ID. " +
					"The API never reports it back, so the first plan after an import shows it being added; applying that plan only records the value.",
				PlanModifiers: []planmodifier.String{
					requiresReplaceUnlessImported(),
					warnIfNotReported(),
				},
			},
			"php_version": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "PHP version to run, in the API's \"major:minor\" form (such as \"8.4:1\"). " +
					"The API reports only the major version (see php_major_version), so an import " +
					"derives this from it. A version changed outside Terraform is not detected.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"php_major_version": schema.StringAttribute{
				Computed:    true,
				Description: "Major PHP version reported by the API (such as \"8.4\").",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"vanity_domain": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Subdomain label of the environment's vanity domain, such as \"my-app\" " +
					"for my-app.laravel.cloud: 3 to 63 lowercase letters, numbers and dashes, not " +
					"starting or ending with a dash. The full hostname is in vanity_hostname. " +
					"Leave unset to keep the assigned default. The API allows one change every 30 minutes.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(3, 63),
					stringvalidator.RegexMatches(vanityLabelPattern,
						"must be a single subdomain label (lowercase letters, numbers and dashes, "+
							"not starting or ending with a dash), such as \"my-app\", not a full hostname"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"vanity_hostname": schema.StringAttribute{
				Computed:    true,
				Description: "Full vanity domain hostname, such as \"my-app.laravel.cloud\".",
				// ModifyPlan marks it unknown when vanity_domain changes.
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"node_version": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Node.js version.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"build_command": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Custom build command (max 2000 chars). Leave unset to keep the framework's default, or for a " +
					"static site to build nothing.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"deploy_command": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Custom deploy command (max 2000 chars).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"post_deploy_command": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Commands that run after a deployment goes live, one per line, such as " +
					"\"php artisan queue:restart\" (max 2000 chars). A failure does not roll the deployment back. " +
					"Only organizations with post-deploy commands enabled can set it, and not on preview or static " +
					"environments; where Laravel Cloud does not offer it, the plan warns and the apply fails. Null when not reported.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 2000),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					postDeployAvailable{},
				},
			},
			"display_name": schema.StringAttribute{
				Computed:    true,
				Description: "Name shown in the dashboard: the name, or a preview environment's pull request title.",
				PlanModifiers: []planmodifier.String{
					followsAttributes{"name"},
				},
			},
			"framework": schema.StringAttribute{
				Computed:    true,
				Description: "Framework Laravel Cloud detected in the branch (such as laravel, nextjs).",
				PlanModifiers: []planmodifier.String{
					followsAttributes{"branch"},
				},
			},
			"language": schema.StringAttribute{
				Computed:    true,
				Description: "Language of the environment (such as php, javascript).",
				PlanModifiers: []planmodifier.String{
					followsAttributes{"branch"},
				},
			},
			"uses_hibernation": schema.BoolAttribute{
				Computed: true,
				Description: "Whether the environment hibernates when idle. Set it with hibernation_timeout on the app " +
					"instance (laravel_cloud_instance with type = \"app\"), which also turns it off, or the deprecated sleep_timeout here.",
				PlanModifiers: []planmodifier.Bool{
					followsAttributes{"sleep_timeout"},
				},
			},
			"created_from_automation": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the environment is a preview environment created for a pull request.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"pull_request": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "The pull request a preview environment deploys. Null for other environments.",
				Attributes: map[string]schema.Attribute{
					"number":        schema.Int64Attribute{Computed: true, Description: "Pull request number."},
					"url":           schema.StringAttribute{Computed: true, Description: "Pull request URL."},
					"title":         schema.StringAttribute{Computed: true, Description: "Pull request title."},
					"source_branch": schema.StringAttribute{Computed: true, Description: "Branch the pull request merges from."},
					"target_branch": schema.StringAttribute{Computed: true, Description: "Branch the pull request merges into."},
					"status":        schema.StringAttribute{Computed: true, Description: "open, closed or merged."},
					"draft":         schema.BoolAttribute{Computed: true, Description: "Whether the pull request is a draft."},
				},
				PlanModifiers: []planmodifier.Object{
					// Follows nothing: keeps the prior value, like UseStateForUnknown.
					followsAttributes{},
				},
			},
			"uses_push_to_deploy": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"uses_deploy_hook": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"deploy_hook_url": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
				Description: "URL that deploys the environment when it receives a POST request, such as from CI. " +
					"Null while uses_deploy_hook is false, or when the API token lacks permission to edit deploy hooks. " +
					"Anyone with the URL can deploy.",
				// ModifyPlan keeps it unless uses_deploy_hook changes.
			},
			"uses_vanity_domain": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"uses_octane": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"uses_purge_edge_cache_on_deploy": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Whether a deploy purges the edge cache. Write-only: the API " +
					"accepts this but never reports it back, so drift cannot be detected.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"timeout": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Seconds the environment waits for the application to respond before timing out (5-60). " +
					"Applies on the next deployment. Leave unset to keep the platform default.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"sleep_timeout": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Minutes without requests before the environment hibernates (1-60). Only instance " +
					"sizes that can hibernate (Flex) take it: on a Pro size Laravel Cloud turns hibernation off " +
					"instead, and the apply fails. The API reports whether hibernation is on but not this value, " +
					"so hibernation turned off outside Terraform shows in the next plan, and a changed value does not. " +
					"It is the same setting as the app instance's hibernation_timeout (laravel_cloud_instance), so set it in " +
					"only one place. The API cannot turn hibernation off through the environment; use the instance for that. " +
					"Deprecated: use hibernation_timeout on a laravel_cloud_instance with type = \"app\".",
				DeprecationMessage: "Use hibernation_timeout on a laravel_cloud_instance with type = \"app\", which takes over " +
					"the environment's app instance and can also turn hibernation off (0). sleep_timeout will be removed " +
					"in a future major version.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"shutdown_timeout": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Seconds to wait before stopping the previous deployment's instances, so they can finish " +
					"their requests and jobs (1-600). Applies on the next deployment. Leave unset to keep the platform default.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"cache_strategy": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Cache strategy (default, bypass).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"database_id": schema.StringAttribute{
				Optional: true,
				Description: "ID of the database to attach (a laravel_cloud_database, not its cluster). " +
					"Removing it detaches the database. A detach or another database attached outside Terraform " +
					"shows in the next plan; left unset, the attachment is not managed.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("database_schema_id")),
				},
			},
			"database_schema_id": schema.StringAttribute{
				Optional:           true,
				Description:        "Deprecated alias for database_id.",
				DeprecationMessage: "database_schema_id is renamed to database_id. Rename the attribute; the attached database is unchanged.",
			},
			"cache_id": schema.StringAttribute{
				Optional: true,
				Description: "Cache to attach. Removing it detaches the cache. A detach or another cache attached " +
					"outside Terraform shows in the next plan; left unset, the attachment is not managed.",
			},
			"websocket_application_id": schema.StringAttribute{
				Optional: true,
				Description: "WebSocket application (laravel_cloud_websocket_application) to attach. Removing it detaches " +
					"the application. A detach or another application attached outside Terraform shows in the next plan; " +
					"left unset, the attachment is not managed.",
			},
			"created_at": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"edge_network_id": schema.StringAttribute{
				Optional: true,
				Description: "Dedicated edge network to create the environment in (see the laravel_cloud_edge_networks " +
					"data source). Only for organizations with a dedicated edge network. " +
					"The API never reports it back, so the first plan after an import shows it being added; applying that plan only records the value.",
				PlanModifiers: []planmodifier.String{
					requiresReplaceUnlessImported(),
					warnIfNotReported(),
				},
			},
			"serving_mode": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "How the environment is served: \"process\" (an application server) or \"static\" " +
					"(files served directly, for a static site). Leave unset to let Laravel Cloud decide: a new environment " +
					"of an application that already has a static environment is static too. It cannot be changed after " +
					"creation, so changing it forces a new environment. Null when the API does not report it.",
				Validators: []validator.String{
					stringvalidator.OneOf("process", "static"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"serving_root": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Directory a static environment serves, relative to the repository (\".\" for the top level). " +
					"Only static environments accept it.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"start_command": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Command that starts a dynamic application that is not PHP, such as \"gunicorn myproject.wsgi\" " +
					"(max 255 chars). Required by the API for dynamic JavaScript, Python, Ruby, Go, Rust and Java " +
					"applications, and rejected for PHP and static ones.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"hibernation_wake_up_interval": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Minutes between periodic wake-ups of a hibernating environment (1-10080, up to 7 days). " +
					"Set 0 to turn periodic wake-ups off.",
				Validators: []validator.Int64{
					int64validator.Between(0, 10080),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"nightwatch_token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "Nightwatch environment token (44 chars) that connects the environment to Nightwatch. " +
					"Removing it disconnects Nightwatch. The API never reports it back, so a token changed outside " +
					"Terraform is not detected. It is stored in Terraform state; with Terraform 1.11 or later, use " +
					"nightwatch_token_wo instead to keep it out of state.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(44, 44),
					stringvalidator.ConflictsWith(path.MatchRoot("nightwatch_token_wo")),
				},
			},
			"nightwatch_token_wo": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "Nightwatch environment token as a write-only argument (Terraform 1.11 or later): it is " +
					"sent and never stored in plan or state. Because Terraform cannot see it change, it is sent " +
					"only on create and when nightwatch_token_wo_version changes; bump the version to rotate it. " +
					"Removing both disconnects Nightwatch.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(44, 44),
					stringvalidator.AlsoRequires(path.MatchRoot("nightwatch_token_wo_version")),
				},
			},
			"nightwatch_token_wo_version": schema.Int64Attribute{
				Optional:    true,
				Description: "Version of nightwatch_token_wo. Changing it sends the current nightwatch_token_wo.",
				Validators: []validator.Int64{
					int64validator.AlsoRequires(path.MatchRoot("nightwatch_token_wo")),
				},
			},
			"filesystem_keys": schema.ListNestedAttribute{
				Optional: true,
				Description: "Object storage buckets attached to the environment, each through one of its access keys " +
					"and under a disk name. This list replaces every attachment: an empty list detaches all buckets, " +
					"and so does removing the attribute once set. Write-only: the API never reports the " +
					"attachments back, so changes made outside Terraform are not detected.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key_id": schema.StringAttribute{
							Required:    true,
							Description: "ID of the bucket access key (laravel_cloud_storage_bucket_key) to attach the bucket with.",
						},
						"disk": schema.StringAttribute{
							Required:    true,
							Description: "Disk name the bucket is exposed as (1-40 chars: letters, numbers, spaces, dashes and underscores).",
							Validators: []validator.String{
								stringvalidator.LengthBetween(1, 40),
							},
						},
						"is_default_disk": schema.BoolAttribute{
							Optional:    true,
							Computed:    true,
							Default:     booldefault.StaticBool(false),
							Description: "Whether this is the default disk. At most one attachment can be. Defaults to false.",
						},
					},
				},
			},
			"response_headers_frame": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "X-Frame-Options response header: deny, sameorigin or all.",
				Validators: []validator.String{
					stringvalidator.OneOf("deny", "sameorigin", "all"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"response_headers_content_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "X-Content-Type-Options response header: nosniff or none.",
				Validators: []validator.String{
					stringvalidator.OneOf("nosniff", "none"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"response_headers_robots_tag": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "X-Robots-Tag response header: \"index, follow\" or \"noindex, nofollow\".",
				Validators: []validator.String{
					stringvalidator.OneOf("index, follow", "noindex, nofollow"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"response_headers_hsts_max_age": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Strict-Transport-Security max-age in seconds; 0 turns HSTS off. Null when HSTS was never set. " +
					"Required when include_subdomains or preload is true.",
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"response_headers_hsts_include_subdomains": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether HSTS also covers subdomains.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"response_headers_hsts_preload": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Whether the HSTS header asks for preloading. Needs include_subdomains and a max_age of " +
					"at least 31536000.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"firewall_block_path": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the firewall blocks requests for common exploit paths.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"firewall_browser_integrity_check": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the firewall runs a browser integrity check on requests.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *EnvironmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected resource configure type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *EnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID := plan.ApplicationID.ValueString()
	wantName := plan.Name.ValueString()
	// Captured before the API response overwrites plan.VanityDomain below;
	// this is the value the user actually configured, if any.
	configuredVanity := plan.VanityDomain

	// Laravel Cloud auto-creates a default environment when an application
	// is created. Check whether an environment with the requested name
	// already exists and adopt it instead of creating a duplicate.
	var env *client.EnvironmentData
	adopted := false

	// A failure here is fatal rather than a reason to fall through: creating
	// blind is exactly the duplicate this listing exists to prevent, and the
	// extra environment would be live and billing with nothing in state
	// pointing at it. Erroring leaves the apply retryable instead.
	existing, err := r.client.ListEnvironments(ctx, appID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing existing environments",
			"Laravel Cloud creates a default environment alongside an application, so the existing "+
				"environments must be checked before creating one. Otherwise the provider creates a duplicate. "+
				"Listing them failed: "+err.Error(),
		)
		return
	}
	for i := range existing {
		if existing[i].Attributes.Name == wantName {
			env = &existing[i]
			adopted = true
			break
		}
	}
	// If no match was found by name, fall through and create a new one.

	// An adopted environment already lives wherever it was created; the edge
	// network is chosen only on create.
	if adopted && !plan.EdgeNetworkID.IsNull() {
		resp.Diagnostics.AddWarning(
			"edge_network_id not applied to the existing environment",
			fmt.Sprintf("An environment named %q already existed and was adopted instead of created, so edge_network_id "+
				"was not used. The edge network can only be chosen when an environment is created.", wantName),
		)
	}

	// Nothing is created yet, so an adopted environment served differently
	// from the configuration is an error rather than a replacement later.
	if adopted && isSet(plan.ServingMode) && !plan.ServingMode.Equal(servingModeFromAPI(env)) {
		resp.Diagnostics.AddAttributeError(path.Root("serving_mode"), "serving_mode cannot be applied to the existing environment",
			fmt.Sprintf("An environment named %q already exists, served as %q, and the serving mode can only be chosen when "+
				"an environment is created. For the application's default environment, set serving_mode on "+
				"laravel_cloud_application instead.", wantName, servingModeFromAPI(env).ValueString()))
		return
	}

	if env == nil {
		createReq := client.CreateEnvironmentRequest{
			Name:   wantName,
			Branch: plan.Branch.ValueString(),
		}
		if isSet(plan.ServingMode) {
			createReq.Serving = &client.ServingRequest{Mode: plan.ServingMode.ValueString(), Root: *nullableStringRequest(resolvedNull(plan.ServingRoot))}
		}
		if isSet(plan.BuildCommand) {
			createReq.BuildCommand = plan.BuildCommand.ValueStringPointer()
		}
		if !plan.ClusterID.IsNull() {
			v := plan.ClusterID.ValueString()
			createReq.ClusterID = &v
		}
		createReq.EdgeNetworkID = plan.EdgeNetworkID.ValueStringPointer()

		created, err := createEnvironment(ctx, r.client, appID, createReq)
		if err != nil {
			resp.Diagnostics.AddError("Error creating environment", err.Error())
			return
		}
		env = created
	}

	// Set server-generated fields from the API response.
	plan.ID = types.StringValue(env.ID)
	// A configured slug is sent in the follow-up update below: the create
	// endpoint takes none.
	setUnknownStringFromAPI(&plan.Slug, env.Attributes.Slug)
	plan.Status = types.StringValue(env.Attributes.Status)
	plan.PHPMajorVersion = types.StringValue(env.Attributes.PHPMajorVersion)
	setVanityFromAPI(&plan, env)
	plan.CreatedAt = types.StringPointerValue(env.Attributes.CreatedAt)

	// Snapshot the plan BEFORE filling unknowns. We use origPlan to decide
	// which fields the user explicitly set (i.e. not unknown/null) when
	// building the PATCH request. This prevents sending API-default values
	// (like color="", sleep_timeout=0) that would fail validation.
	origPlan := plan

	// For Computed+Optional fields that the user did NOT explicitly set,
	// accept whatever the API returned as the initial value.
	setUnknownStringFromAPI(&plan.NodeVersion, env.Attributes.NodeVersion)
	setUnknownStringPtrFromAPI(&plan.BuildCommand, env.Attributes.BuildCommand)
	setUnknownStringPtrFromAPI(&plan.DeployCommand, env.Attributes.DeployCommand)
	setUnknownBoolFromAPI(&plan.UsesPushToDeploy, env.Attributes.UsesPushToDeploy)
	setUnknownBoolFromAPI(&plan.UsesDeployHook, env.Attributes.UsesDeployHook)
	setUnknownBoolFromAPI(&plan.UsesOctane, env.Attributes.UsesOctane)
	setUnknownStringFromAPI(&plan.CacheStrategy, env.Attributes.NetworkSettings.Cache.Strategy)
	setUnknownStringPtrFromAPI(&plan.Color, env.Attributes.Color)
	setUnknownInt64PtrFromAPI(&plan.Timeout, env.Attributes.Timeout)
	setUnknownInt64PtrFromAPI(&plan.ShutdownTimeout, env.Attributes.ShutdownTimeout)
	setUnknownSettingsFromAPI(&plan, env)
	setUnknownDetailsFromAPI(&plan, env)

	// The remaining Optional+Computed attributes are write-only: the API
	// accepts them on PATCH but never returns them, so there is no value to
	// adopt. Resolve them to null rather than to a fabricated zero value,
	// which would then be re-sent on every apply.
	resolveUnknownBoolToNull(&plan.UsesVanityDomain)
	resolveUnknownBoolToNull(&plan.UsesPurgeEdgeCacheOnDeploy)
	resolveUnknownInt64ToNull(&plan.SleepTimeout)

	// If the user specified values that differ from API defaults, attempt
	// to PATCH the environment so the remote state matches the plan.
	// We pass origPlan so we only PATCH fields the user explicitly set
	// (not fields that were unknown and got filled from the API).
	// A configured vanity domain is applied through its own endpoint.
	//
	// The environment already exists at this point, so a failure here must NOT
	// abort Create: returning early would skip resp.State.Set below and leave a
	// live, billing environment that Terraform has no record of and cannot
	// destroy. The vanity hostname is the only thing lost, and the next apply
	// retries it, so this is a warning -- matching how the settings PATCH below
	// already handles the same situation.
	if !configuredVanity.IsNull() && !configuredVanity.IsUnknown() {
		updated, err := applyVanityDomain(ctx, r.client, env.ID, configuredVanity)
		if err != nil {
			resp.Diagnostics.AddWarning(
				"Could not set the vanity domain after creating the environment",
				"The environment was created, but setting its vanity domain failed: "+err.Error()+
					". The configured value is kept in state; run 'terraform apply' again to retry.",
			)
		} else if updated != nil {
			setVanityFromAPI(&plan, updated)
		}
		plan.VanityDomain = configuredVanity
	}

	patchReq := buildEnvironmentUpdateFromDiff(origPlan, env, adopted)
	if wo, d := nightwatchTokenWO(ctx, req.Config); d.HasError() {
		resp.Diagnostics.Append(d...)
		return
	} else if !wo.IsNull() {
		if patchReq == nil {
			patchReq = &client.UpdateEnvironmentRequest{}
		}
		patchReq.NightwatchToken = nullableStringRequest(wo)
	}
	if patchReq != nil {
		patched, err := updateEnvironmentWhenReady(ctx, r.client, env.ID, *patchReq)
		if err == nil && patched != nil {
			// Read-only values the PATCH changed must come from its response.
			// php_version is sent as "8.4:1" and reported back only as a major
			// version, so the create response still said the platform default
			// (8.5) while the environment was really on 8.4 -- state was wrong
			// the moment apply finished, and an import disagreed with it.
			plan.PHPMajorVersion = types.StringValue(patched.Attributes.PHPMajorVersion)
			plan.Slug = types.StringValue(patched.Attributes.Slug)
			plan.Status = types.StringValue(patched.Attributes.Status)
			setUnknownStringPtrFromAPI(&plan.BuildCommand, patched.Attributes.BuildCommand)
			setUnknownStringPtrFromAPI(&plan.DeployCommand, patched.Attributes.DeployCommand)
			setUnknownStringPtrFromAPI(&plan.DeployHookURL, patched.Attributes.DeployHookURL)
			if patched.Attributes.UsesHibernation != nil {
				plan.UsesHibernation = types.BoolPointerValue(patched.Attributes.UsesHibernation)
			}
			// The configured value stays in state for now; the next refresh
			// records null, so the next plan shows it and the update fails.
			if patchReq.PostDeployCommand != nil && !patched.Attributes.PostDeployCommand.Reported {
				resp.Diagnostics.AddAttributeWarning(path.Root("post_deploy_command"), postDeployIgnoredSummary, postDeployIgnoredDetail)
			}
			// The environment exists, so this is a warning: an error would
			// taint it and the next apply would replace it, to the same end.
			// The next refresh drops sleep_timeout, so the plan shows it again
			// and the update fails with the same message.
			if patchReq.SleepTimeout != nil && hibernationOff(patched) {
				resp.Diagnostics.AddAttributeWarning(path.Root("sleep_timeout"), hibernationRefusedSummary, hibernationRefusedDetail)
			}
		}
		if err != nil {
			// PATCH failed (e.g. dev server returns HTML). Keep the plan
			// values in state so Terraform doesn't error. The next Read
			// will detect drift if the remote values are different.
			resp.Diagnostics.AddWarning(
				"Could not apply all settings after creation",
				"The environment was created, but updating settings failed: "+err.Error()+
					". The planned values are kept in state. Run 'terraform apply' again to retry.",
			)
			// nightwatch_token and filesystem_keys are never reported back,
			// so no refresh could notice they were not applied and the next
			// plan would send nothing. Mark them for Read to drop instead.
			if patchReq.NightwatchToken != nil || patchReq.FilesystemKeys != nil {
				resp.Diagnostics.Append(resp.Private.SetKey(ctx, unsentWriteOnlyKey, []byte("true"))...)
			}
		}
		// Whether PATCH succeeded or failed, we keep the plan values in
		// state. The API may not echo back the values we sent (e.g.
		// php_version returns "" even when we sent "8.4:1"), so using
		// mapEnvironmentToState here would overwrite user values and
		// cause "inconsistent result after apply".
	}

	// An unset php_version stays null rather than recording the platform's
	// version: the API reports only the major version and Read never rewrites
	// php_version, so a value recorded here would go stale on the first change
	// made outside Terraform, and pinning that same version later would plan
	// nothing. A configured one was sent in the PATCH above and stays as planned.
	resolveUnknownStringToNull(&plan.PHPVersion)
	// Still unknown when no follow-up update was needed or it failed.
	setUnknownStringPtrFromAPI(&plan.DeployHookURL, env.Attributes.DeployHookURL)

	// The environment exists, so failing to start or stop it is a warning: an
	// error would taint it and the next apply would replace it. The usual case
	// is running = false on an environment that has never been deployed, which
	// Laravel Cloud cannot stop yet. As with the settings above, the configured
	// value stays in state, which Terraform requires: the next refresh records
	// whether the environment really is running, so the next plan offers the
	// change again, and Update retries it or fails with the API's reason.
	var runningDiags diag.Diagnostics
	if !applyRunning(ctx, r.client, env.ID, &plan, environmentRunning(plan.Status.ValueString()), &runningDiags) {
		resp.Diagnostics.AddAttributeWarning(path.Root("running"), "Could not start or stop the environment after creating it",
			runningDiags.Errors()[0].Detail()+" The configured value is kept in state, so the next plan shows the "+
				"change again and 'terraform apply' retries it. An environment that has never been deployed cannot "+
				"be stopped: deploy it first.")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// The API refuses to attach a database or cache that is creating or updating
// ("The selected cache is still being provisioned. Try again in a few
// seconds."), such as one created or resized earlier in the same apply. These
// bound how long an environment update retries that. The interval is a
// variable so the plan tests need not wait.
const environmentBusyAttempts = 30

var environmentBusyInterval = 10 * time.Second

func updateEnvironmentWhenReady(ctx context.Context, c *client.Client, id string, req client.UpdateEnvironmentRequest) (*client.EnvironmentData, error) {
	var env *client.EnvironmentData
	err := client.RetryOnConflict(ctx, environmentBusyAttempts, environmentBusyInterval, func() error {
		var err error
		env, err = c.UpdateEnvironment(ctx, id, req)
		return err
	}, client.IsBusy)
	return env, err
}

// ---------------------------------------------------------------------------
// Helpers – fill plan fields that Terraform marked as "unknown" with API values
// ---------------------------------------------------------------------------

func setUnknownStringFromAPI(field *types.String, apiVal string) {
	if field.IsUnknown() {
		*field = types.StringValue(apiVal)
	}
}

func setUnknownStringPtrFromAPI(field *types.String, apiVal *string) {
	if field.IsUnknown() {
		*field = types.StringPointerValue(apiVal)
	}
}

func setUnknownBoolFromAPI(field *types.Bool, apiVal bool) {
	if field.IsUnknown() {
		*field = types.BoolValue(apiVal)
	}
}

func setUnknownInt64PtrFromAPI(field *types.Int64, apiVal *int64) {
	if field.IsUnknown() {
		*field = types.Int64PointerValue(apiVal)
	}
}

func (r *EnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := r.client.GetEnvironment(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading environment", err.Error())
		return
	}

	mapEnvironmentToState(env, &state)
	resp.Diagnostics.Append(setPostDeployUnavailable(ctx, resp.Private, env)...)
	// Settings that never reached the API after create are dropped from state,
	// so the next plan shows them being added and sends them.
	if pending, d := req.Private.GetKey(ctx, unsentWriteOnlyKey); d.HasError() {
		resp.Diagnostics.Append(d...)
		return
	} else if len(pending) > 0 {
		state.NightwatchToken = types.StringNull()
		state.NightwatchTokenWOVersion = types.Int64Null()
		state.FilesystemKeys = nil
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// nightwatchTokenWO reads the write-only token, which only the configuration
// carries.
func nightwatchTokenWO(ctx context.Context, config tfsdk.Config) (types.String, diag.Diagnostics) {
	var wo types.String
	d := config.GetAttribute(ctx, path.Root("nightwatch_token_wo"), &wo)
	if wo.IsUnknown() {
		wo = types.StringNull()
	}
	return wo, d
}

// unsentWriteOnlyKey is the private state key that records an environment
// whose nightwatch_token and filesystem_keys failed to reach the API on
// create. An update that succeeds clears it.
const unsentWriteOnlyKey = "unsent_write_only"

func (r *EnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan EnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state EnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The vanity domain has its own PUT route and is not part of the
	// environment update body.
	// On update the resource is already in state, so Terraform keeps the prior
	// state when this errors and nothing is orphaned -- a failure here is fatal.
	if !plan.VanityDomain.Equal(state.VanityDomain) {
		updated, err := applyVanityDomain(ctx, r.client, state.ID.ValueString(), plan.VanityDomain)
		if err != nil {
			resp.Diagnostics.AddError("Error setting vanity domain", err.Error())
			return
		}
		if updated != nil {
			setVanityFromAPI(&plan, updated)
		}
	}
	// Only a vanity domain change leaves the hostname unknown, and it was
	// resolved above; this covers an update that sent nothing.
	if plan.VanityHostname.IsUnknown() {
		plan.VanityHostname = state.VanityHostname
	}

	updateReq := buildEnvironmentUpdateFromStateDiff(plan, state)
	// nightwatch_token_wo never reaches state, so its version stands in for
	// it: a changed version sends the current token, or null when it was
	// removed. It goes after the state diff so that switching from
	// nightwatch_token sends the new token rather than clearing it.
	if !plan.NightwatchTokenWOVersion.Equal(state.NightwatchTokenWOVersion) {
		wo, d := nightwatchTokenWO(ctx, req.Config)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		if !wo.IsNull() || plan.NightwatchToken.IsNull() {
			if updateReq == nil {
				updateReq = &client.UpdateEnvironmentRequest{}
			}
			updateReq.NightwatchToken = nullableStringRequest(wo)
		}
	}
	if updateReq == nil {
		// Nothing else changed — just keep state as-is.
		if plan.DeployHookURL.IsUnknown() {
			plan.DeployHookURL = state.DeployHookURL
		}
		keepUnknownDetails(&plan, state)
		if applyRunning(ctx, r.client, state.ID.ValueString(), &plan, state.Running.ValueBool(), &resp.Diagnostics) {
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		}
		return
	}

	updated, err := updateEnvironmentWhenReady(ctx, r.client, state.ID.ValueString(), *updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating environment", err.Error())
		return
	}
	// Prior state is kept, so the next refresh records what the API did and
	// the next plan offers the same change again.
	if updateReq.SleepTimeout != nil && hibernationOff(updated) {
		resp.Diagnostics.AddAttributeError(path.Root("sleep_timeout"), hibernationRefusedSummary, hibernationRefusedDetail)
		return
	}
	if updateReq.PostDeployCommand != nil && !updated.Attributes.PostDeployCommand.Reported {
		resp.Diagnostics.AddAttributeError(path.Root("post_deploy_command"), postDeployIgnoredSummary, postDeployIgnoredDetail)
		return
	}
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, unsentWriteOnlyKey, nil)...)

	// Keep plan values in state rather than overwriting from the API
	// response. The API may not echo back the values we sent (e.g.
	// php_version returns "" even when we sent "8.4:1"), which would
	// cause "Provider produced inconsistent result after apply".
	plan.ID = state.ID
	plan.ApplicationID = state.ApplicationID
	plan.CreatedAt = state.CreatedAt
	plan.Slug = types.StringValue(updated.Attributes.Slug)
	// Turning the deploy hook on or off creates or drops its URL.
	setUnknownStringPtrFromAPI(&plan.DeployHookURL, updated.Attributes.DeployHookURL)
	setUnknownDetailsFromAPI(&plan, updated)
	// Started after the update, so its deployment has the new settings.
	if !applyRunning(ctx, r.client, state.ID.ValueString(), &plan, state.Running.ValueBool(), &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envID := state.ID.ValueString()

	// The API refuses to delete an application's default environment, which
	// this resource manages when it took over the one created with the
	// application. Deleting the application deletes it, so it is only dropped
	// from state here, before anything below changes it: it keeps serving, with
	// its database, cache and WebSocket application still attached. A
	// database or cache destroyed in the same apply needs no detach first,
	// since the API detaches it from its environments when deleting it. Any
	// failure to tell falls through to the delete, whose error then says why.
	if r.isDefaultEnvironment(ctx, state) {
		resp.Diagnostics.AddWarning(
			"Default environment not deleted",
			fmt.Sprintf("Environment %q is its application's default environment, which Laravel Cloud cannot delete "+
				"on its own. It was removed from Terraform state unchanged, still running with its attachments, and "+
				"is deleted along with the application. To delete it sooner, make another environment the "+
				"application's default first.", state.Name.ValueString()),
		)
		return
	}

	// Detach database, cache and WebSocket application from the environment
	// before deleting so downstream resources (database schema, cluster,
	// WebSocket server) can be cleaned up.
	if !databaseID(state).IsNull() || !state.CacheID.IsNull() || !state.WebsocketApplicationID.IsNull() {
		detach := client.UpdateEnvironmentRequest{}
		if !databaseID(state).IsNull() {
			detach.DatabaseSchemaID = client.DetachID()
		}
		if !state.CacheID.IsNull() {
			detach.CacheID = client.DetachID()
		}
		if !state.WebsocketApplicationID.IsNull() {
			detach.WebsocketApplicationID = client.DetachID()
		}
		// Best-effort detach, but surface a failure as a warning: the
		// subsequent delete will fail opaquely if the resources stay attached.
		if _, err := r.client.UpdateEnvironment(ctx, envID, detach); err != nil {
			resp.Diagnostics.AddWarning(
				"Could not detach environment resources before delete",
				"Detaching the database and cache from the environment failed, so deleting it may also fail: "+err.Error(),
			)
		}
	}

	if err := r.client.DeleteEnvironment(ctx, envID); err != nil {
		resp.Diagnostics.AddError("Error deleting environment", err.Error())
	}
}

// isDefaultEnvironment reports whether the environment is its application's
// default environment. An imported environment has no application_id in
// state, so the application is then looked up from the environment.
func (r *EnvironmentResource) isDefaultEnvironment(ctx context.Context, state EnvironmentResourceModel) bool {
	appID := state.ApplicationID.ValueString()
	if appID == "" {
		env, err := r.client.GetEnvironment(ctx, state.ID.ValueString())
		if err != nil {
			return false
		}
		appID = env.Relationships.Application.RelatedID()
	}
	if appID == "" {
		return false
	}
	app, err := r.client.GetApplication(ctx, appID)
	if err != nil {
		return false
	}
	return app.Relationships.DefaultEnvironment.RelatedID() == state.ID.ValueString()
}

func (r *EnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read never touches php_version, so an import would leave it null and the
	// first plan would propose adding the configured value even when the
	// platform already runs it. Import is the one point where nothing is
	// recorded, so it is seeded here from the major version the API reports.
	env, err := r.client.GetEnvironment(ctx, req.ID)
	if err != nil {
		if client.IsNotFound(err) {
			return // Read reports the missing environment.
		}
		resp.Diagnostics.AddError("Error importing environment", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("php_version"), phpVersionFromMajor(env.Attributes.PHPMajorVersion))...)
}

// buildEnvironmentUpdateFromStateDiff compares the plan against the current
// state and only includes fields that actually changed.
//
// Comparisons use Equal rather than ValueBool/ValueString, which collapse null
// onto the zero value: with the latter, setting a write-only boolean to false
// after it had been null looked like no change at all, so nothing was sent and
// -- since these attributes are never read back -- the divergence could never
// be noticed or corrected. This avoids sending
// server-managed values (color, sleep_timeout, etc.) that the user never set.
func buildEnvironmentUpdateFromStateDiff(plan, state EnvironmentResourceModel) *client.UpdateEnvironmentRequest {
	req := client.UpdateEnvironmentRequest{}
	hasUpdates := false

	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		req.Name = &v
		hasUpdates = true
	}
	if !plan.Branch.Equal(state.Branch) {
		v := plan.Branch.ValueString()
		req.Branch = &v
		hasUpdates = true
	}
	if isSet(plan.Slug) && !plan.Slug.Equal(state.Slug) {
		req.Slug = plan.Slug.ValueStringPointer()
		hasUpdates = true
	}
	if !plan.Color.Equal(state.Color) {
		v := plan.Color.ValueString()
		req.Color = &v
		hasUpdates = true
	}
	if !plan.PHPVersion.Equal(state.PHPVersion) {
		v := plan.PHPVersion.ValueString()
		req.PHPVersion = &v
		hasUpdates = true
	}
	if !plan.NodeVersion.Equal(state.NodeVersion) {
		v := plan.NodeVersion.ValueString()
		req.NodeVersion = &v
		hasUpdates = true
	}
	if !plan.BuildCommand.Equal(state.BuildCommand) {
		v := plan.BuildCommand.ValueString()
		req.BuildCommand = &v
		hasUpdates = true
	}
	if !plan.DeployCommand.Equal(state.DeployCommand) {
		v := plan.DeployCommand.ValueString()
		req.DeployCommand = &v
		hasUpdates = true
	}
	// Unset keeps what Laravel Cloud has, like the other commands.
	if isSet(plan.PostDeployCommand) && !plan.PostDeployCommand.Equal(state.PostDeployCommand) {
		req.PostDeployCommand = nullableStringRequest(plan.PostDeployCommand)
		hasUpdates = true
	}
	if !plan.UsesPushToDeploy.Equal(state.UsesPushToDeploy) {
		v := plan.UsesPushToDeploy.ValueBool()
		req.UsesPushToDeploy = &v
		hasUpdates = true
	}
	if !plan.UsesOctane.Equal(state.UsesOctane) {
		v := plan.UsesOctane.ValueBool()
		req.UsesOctane = &v
		hasUpdates = true
	}
	if !plan.CacheStrategy.Equal(state.CacheStrategy) {
		v := plan.CacheStrategy.ValueString()
		req.CacheStrategy = &v
		hasUpdates = true
	}
	if !plan.Timeout.Equal(state.Timeout) {
		v := int(plan.Timeout.ValueInt64())
		req.Timeout = &v
		hasUpdates = true
	}
	if !plan.SleepTimeout.Equal(state.SleepTimeout) {
		v := int(plan.SleepTimeout.ValueInt64())
		req.SleepTimeout = &v
		hasUpdates = true
	}
	if !plan.ShutdownTimeout.Equal(state.ShutdownTimeout) {
		v := int(plan.ShutdownTimeout.ValueInt64())
		req.ShutdownTimeout = &v
		hasUpdates = true
	}
	if !plan.UsesDeployHook.Equal(state.UsesDeployHook) {
		v := plan.UsesDeployHook.ValueBool()
		req.UsesDeployHook = &v
		hasUpdates = true
	}
	if !plan.UsesVanityDomain.Equal(state.UsesVanityDomain) {
		v := plan.UsesVanityDomain.ValueBool()
		req.UsesVanityDomain = &v
		hasUpdates = true
	}
	if !plan.UsesPurgeEdgeCacheOnDeploy.Equal(state.UsesPurgeEdgeCacheOnDeploy) {
		v := plan.UsesPurgeEdgeCacheOnDeploy.ValueBool()
		req.UsesPurgeEdgeCacheOnDeploy = &v
		hasUpdates = true
	}
	if !databaseID(plan).Equal(databaseID(state)) {
		req.DatabaseSchemaID = attachOrDetach(databaseID(plan))
		hasUpdates = true
	}
	if !plan.CacheID.Equal(state.CacheID) {
		req.CacheID = attachOrDetach(plan.CacheID)
		hasUpdates = true
	}
	if !plan.WebsocketApplicationID.Equal(state.WebsocketApplicationID) {
		req.WebsocketApplicationID = attachOrDetach(plan.WebsocketApplicationID)
		hasUpdates = true
	}
	if addSettingsFromStateDiff(&req, plan, state) {
		hasUpdates = true
	}

	if !hasUpdates {
		return nil
	}
	return &req
}

// ModifyPlan rejects a plan that sets both database_id and its deprecated
// alias database_schema_id. Validation already rejects both in config, so this
// only fires when lifecycle.ignore_changes on database_schema_id carries the
// old value into the plan next to database_id. Accepting that would store
// both, and removing database_id later would fall back to the stale alias and
// re-attach the old database instead of detaching.
func (r *EnvironmentResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Nothing to do when the resource is being destroyed.
	if req.Plan.Raw.IsNull() {
		return
	}

	// vanity_hostname follows vanity_domain, so a new label means a new
	// hostname, which only the API can say.
	if !req.State.Raw.IsNull() {
		var planVanity, stateVanity types.String
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("vanity_domain"), &planVanity)...)
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("vanity_domain"), &stateVanity)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if !planVanity.Equal(stateVanity) {
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("vanity_hostname"), types.StringUnknown())...)
		}

		// deploy_hook_url only changes with uses_deploy_hook. It is often null,
		// which UseStateForUnknown would not carry over, so it is kept here.
		var planHook, stateHook types.Bool
		var stateHookURL types.String
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("uses_deploy_hook"), &planHook)...)
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("uses_deploy_hook"), &stateHook)...)
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("deploy_hook_url"), &stateHookURL)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if planHook.Equal(stateHook) {
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("deploy_hook_url"), stateHookURL)...)
		}
		modifyRunningPlan(ctx, req, resp)
		r.refuseDefaultEnvironmentReplacement(ctx, req, resp)
	}

	var dbID, schemaID types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("database_id"), &dbID)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("database_schema_id"), &schemaID)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// An unknown value may still resolve to null, so leave that case to the
	// ConflictsWith validator, which runs again once values are known.
	if dbID.IsNull() || dbID.IsUnknown() || schemaID.IsNull() || schemaID.IsUnknown() {
		return
	}
	resp.Diagnostics.AddAttributeError(
		path.Root("database_schema_id"),
		"Conflicting database attributes",
		"database_id and its deprecated alias database_schema_id are both set in the plan, "+
			"most likely because database_schema_id is listed in lifecycle.ignore_changes. "+
			"Remove it from ignore_changes (or list database_id there instead).",
	)
}

// refuseDefaultEnvironmentReplacement fails the plan when a serving_mode
// change would replace an application's default environment. Laravel Cloud
// cannot delete that environment, so the replacement's delete would only drop
// it from state, and the create would then find it again, still served the old
// way, and fail: the apply would end with the environment no longer in state.
// The API is asked only when serving_mode changes.
func (r *EnvironmentResource) refuseDefaultEnvironmentReplacement(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var planServing, stateServing types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("serving_mode"), &planServing)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("serving_mode"), &stateServing)...)
	if resp.Diagnostics.HasError() || !isSet(planServing) || !isSet(stateServing) || planServing.Equal(stateServing) || r.client == nil {
		return
	}
	var state EnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || !r.isDefaultEnvironment(ctx, state) {
		return
	}
	resp.Diagnostics.AddAttributeError(path.Root("serving_mode"), "serving_mode cannot be changed on the default environment",
		fmt.Sprintf("Changing serving_mode replaces the environment, but %q is its application's default environment, "+
			"which Laravel Cloud cannot delete, so the replacement would fail after removing it from Terraform state. "+
			"Keep serving_mode = %q, or make another environment the application's default first.",
			state.Name.ValueString(), stateServing.ValueString()))
}

// databaseID returns the attached database from database_id, falling back to
// its deprecated alias database_schema_id. Comparing this rather than either
// attribute means renaming one to the other sends nothing to the API.
func databaseID(m EnvironmentResourceModel) types.String {
	if !m.DatabaseID.IsNull() {
		return m.DatabaseID
	}
	return m.DatabaseSchemaID
}

// attachOrDetach renders an attachment id for the environment PATCH body:
// a JSON string when set, JSON null when the id was removed from config.
func attachOrDetach(v types.String) json.RawMessage {
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return client.DetachID()
	}
	return client.AttachID(v.ValueString())
}

// buildEnvironmentUpdateFromDiff compares plan values against the freshly-created
// (or adopted) environment and only returns a PATCH request when something
// actually differs. When adopted is true the environment was not freshly
// created so write-only fields (branch) are always included in the PATCH.
func buildEnvironmentUpdateFromDiff(plan EnvironmentResourceModel, env *client.EnvironmentData, adopted bool) *client.UpdateEnvironmentRequest {
	req := client.UpdateEnvironmentRequest{}
	hasUpdates := false

	if v := plan.Name.ValueString(); !plan.Name.IsNull() && !plan.Name.IsUnknown() && v != env.Attributes.Name {
		req.Name = &v
		hasUpdates = true
	}
	if isSet(plan.Slug) && plan.Slug.ValueString() != env.Attributes.Slug {
		req.Slug = plan.Slug.ValueStringPointer()
		hasUpdates = true
	}
	// Branch is write-only (not in the API response). On adoption we must
	// always send it so the existing environment picks up the desired branch.
	if adopted && !plan.Branch.IsNull() && !plan.Branch.IsUnknown() {
		v := plan.Branch.ValueString()
		req.Branch = &v
		hasUpdates = true
	}
	// php_version is sent in "major:minor" form but the API only echoes a
	// major version, so it cannot be compared against the response. Send it
	// whenever the user set it; re-sending the same value is harmless.
	if v := plan.PHPVersion.ValueString(); !plan.PHPVersion.IsNull() && !plan.PHPVersion.IsUnknown() {
		req.PHPVersion = &v
		hasUpdates = true
	}
	if v := plan.NodeVersion.ValueString(); !plan.NodeVersion.IsNull() && !plan.NodeVersion.IsUnknown() && v != env.Attributes.NodeVersion {
		req.NodeVersion = &v
		hasUpdates = true
	}
	if isSet(plan.Color) && !plan.Color.Equal(types.StringPointerValue(env.Attributes.Color)) {
		req.Color = plan.Color.ValueStringPointer()
		hasUpdates = true
	}
	if !plan.UsesPushToDeploy.IsNull() && !plan.UsesPushToDeploy.IsUnknown() && plan.UsesPushToDeploy.ValueBool() != env.Attributes.UsesPushToDeploy {
		v := plan.UsesPushToDeploy.ValueBool()
		req.UsesPushToDeploy = &v
		hasUpdates = true
	}
	if !plan.UsesOctane.IsNull() && !plan.UsesOctane.IsUnknown() && plan.UsesOctane.ValueBool() != env.Attributes.UsesOctane {
		v := plan.UsesOctane.ValueBool()
		req.UsesOctane = &v
		hasUpdates = true
	}
	if isSet(plan.Timeout) && !plan.Timeout.Equal(types.Int64PointerValue(env.Attributes.Timeout)) {
		v := int(plan.Timeout.ValueInt64())
		req.Timeout = &v
		hasUpdates = true
	}
	if !plan.SleepTimeout.IsNull() && !plan.SleepTimeout.IsUnknown() {
		v := int(plan.SleepTimeout.ValueInt64())
		req.SleepTimeout = &v
		hasUpdates = true
	}
	if isSet(plan.ShutdownTimeout) && !plan.ShutdownTimeout.Equal(types.Int64PointerValue(env.Attributes.ShutdownTimeout)) {
		v := int(plan.ShutdownTimeout.ValueInt64())
		req.ShutdownTimeout = &v
		hasUpdates = true
	}
	if v := plan.CacheStrategy.ValueString(); !plan.CacheStrategy.IsNull() && !plan.CacheStrategy.IsUnknown() && v != env.Attributes.NetworkSettings.Cache.Strategy {
		req.CacheStrategy = &v
		hasUpdates = true
	}
	// These are Optional+Computed, so a value the user did not set arrives here
	// UNKNOWN rather than null. IsNull() alone is false for an unknown value,
	// and ValueString() on it yields "" -- which sent build_command: "" and
	// made the API null out the application's default build and deploy
	// commands on every single environment create.
	if !plan.BuildCommand.IsNull() && !plan.BuildCommand.IsUnknown() {
		v := plan.BuildCommand.ValueString()
		if env.Attributes.BuildCommand == nil || v != *env.Attributes.BuildCommand {
			req.BuildCommand = &v
			hasUpdates = true
		}
	}
	if !plan.DeployCommand.IsNull() && !plan.DeployCommand.IsUnknown() {
		v := plan.DeployCommand.ValueString()
		if env.Attributes.DeployCommand == nil || v != *env.Attributes.DeployCommand {
			req.DeployCommand = &v
			hasUpdates = true
		}
	}
	if isSet(plan.PostDeployCommand) && !plan.PostDeployCommand.Equal(types.StringPointerValue(env.Attributes.PostDeployCommand.Value)) {
		req.PostDeployCommand = nullableStringRequest(plan.PostDeployCommand)
		hasUpdates = true
	}
	if dbID := databaseID(plan); !dbID.IsNull() && !dbID.IsUnknown() {
		req.DatabaseSchemaID = client.AttachID(dbID.ValueString())
		hasUpdates = true
	}
	if !plan.CacheID.IsNull() && !plan.CacheID.IsUnknown() {
		req.CacheID = client.AttachID(plan.CacheID.ValueString())
		hasUpdates = true
	}
	if isSet(plan.WebsocketApplicationID) {
		req.WebsocketApplicationID = client.AttachID(plan.WebsocketApplicationID.ValueString())
		hasUpdates = true
	}
	// These three are write-only, so they are sent whenever configured.
	if !plan.UsesDeployHook.IsNull() && !plan.UsesDeployHook.IsUnknown() && plan.UsesDeployHook.ValueBool() != env.Attributes.UsesDeployHook {
		v := plan.UsesDeployHook.ValueBool()
		req.UsesDeployHook = &v
		hasUpdates = true
	}
	if !plan.UsesVanityDomain.IsNull() && !plan.UsesVanityDomain.IsUnknown() {
		v := plan.UsesVanityDomain.ValueBool()
		req.UsesVanityDomain = &v
		hasUpdates = true
	}
	if !plan.UsesPurgeEdgeCacheOnDeploy.IsNull() && !plan.UsesPurgeEdgeCacheOnDeploy.IsUnknown() {
		v := plan.UsesPurgeEdgeCacheOnDeploy.ValueBool()
		req.UsesPurgeEdgeCacheOnDeploy = &v
		hasUpdates = true
	}
	if addSettingsFromCreateDiff(&req, plan, env) {
		hasUpdates = true
	}

	if !hasUpdates {
		return nil
	}
	return &req
}

func mapEnvironmentToState(env *client.EnvironmentData, state *EnvironmentResourceModel) {
	// application_id is Required and forces replacement, and the import id
	// names the environment alone. Without adopting it from the relationship
	// an imported environment would be proposed for destruction on the very
	// next plan.
	if appID := env.Relationships.Application.RelatedID(); appID != "" {
		state.ApplicationID = types.StringValue(appID)
	}
	// branch is Required but is not an attribute on the API side; the name
	// comes from the included branch object. Only overwrite when the API
	// reported one, so a response without the include leaves the configured
	// value intact.
	if env.BranchName != "" {
		state.Branch = types.StringValue(env.BranchName)
	}
	state.Name = types.StringValue(env.Attributes.Name)
	state.Slug = types.StringValue(env.Attributes.Slug)
	state.Status = types.StringValue(env.Attributes.Status)
	state.Running = types.BoolValue(environmentRunning(env.Attributes.Status))
	// php_version is not echoed by the API in the form it is sent, so it is
	// left untouched here to preserve the configured value; the read-only
	// major version is surfaced via php_major_version instead. An import
	// seeds it, see ImportState.
	state.PHPMajorVersion = types.StringValue(env.Attributes.PHPMajorVersion)
	setVanityFromAPI(state, env)
	state.NodeVersion = types.StringValue(env.Attributes.NodeVersion)
	state.UsesPushToDeploy = types.BoolValue(env.Attributes.UsesPushToDeploy)
	state.UsesDeployHook = types.BoolValue(env.Attributes.UsesDeployHook)
	state.DeployHookURL = types.StringPointerValue(env.Attributes.DeployHookURL)
	state.UsesOctane = types.BoolValue(env.Attributes.UsesOctane)
	// cache_strategy is readable only from network_settings; there is no
	// top-level attribute of that name in the response.
	state.CacheStrategy = types.StringValue(env.Attributes.NetworkSettings.Cache.Strategy)
	state.CreatedAt = types.StringPointerValue(env.Attributes.CreatedAt)
	state.BuildCommand = types.StringPointerValue(env.Attributes.BuildCommand)
	state.DeployCommand = types.StringPointerValue(env.Attributes.DeployCommand)
	keepStringIfAbsent(&state.Color, env.Attributes.Color)
	keepInt64IfAbsent(&state.Timeout, env.Attributes.Timeout)
	keepInt64IfAbsent(&state.ShutdownTimeout, env.Attributes.ShutdownTimeout)
	if state.DatabaseID.IsNull() {
		readAttachment(&state.DatabaseSchemaID, env.Relationships.Database)
	} else {
		readAttachment(&state.DatabaseID, env.Relationships.Database)
	}
	readAttachment(&state.CacheID, env.Relationships.Cache)
	readAttachment(&state.WebsocketApplicationID, env.Relationships.WebsocketApplication)
	// sleep_timeout is not reported, but hibernation being off means there is
	// none, so a configured one shows up in the next plan.
	if hibernationOff(env) {
		state.SleepTimeout = types.Int64Null()
	}
	mapSettingsToState(env, state)
	mapDetailsToState(env, state)

	// sleep_timeout, uses_purge_edge_cache_on_deploy and uses_vanity_domain
	// are write-only: PATCH accepts them but the response never carries them
	// back. Writing a zero value here would fight the configuration on every
	// refresh, so the configured values are left in place and drift on them is
	// not detectable.
}

// readAttachment records the attachment the API reports for one the
// configuration manages, so a detach or a different attachment made outside
// Terraform shows in the next plan. One the configuration never set stays
// null: that leaves an attachment made in the dashboard alone rather than
// planning to detach it. A response without the relationship changes nothing.
func readAttachment(dst *types.String, rel *client.Relationship) {
	if rel == nil || dst.IsNull() {
		return
	}
	if id := rel.RelatedID(); id != "" {
		*dst = types.StringValue(id)
	} else {
		*dst = types.StringNull()
	}
}

// hibernationOff reports whether the API says sleep mode is off. After an
// update that sent sleep_timeout, that means the API refused it: for an
// instance size that cannot hibernate it answers 200 but turns sleep mode and
// the periodic wake-up off instead.
func hibernationOff(env *client.EnvironmentData) bool {
	return env != nil && env.Attributes.UsesHibernation != nil && !*env.Attributes.UsesHibernation
}

const (
	hibernationRefusedSummary = "Laravel Cloud turned hibernation off instead of applying sleep_timeout"
	hibernationRefusedDetail  = "The environment's app instance has a size that cannot hibernate (Pro sizes cannot), so " +
		"Laravel Cloud accepted sleep_timeout but turned hibernation and the periodic wake-up " +
		"(hibernation_wake_up_interval) off. Move the app instance to a Flex size, or remove sleep_timeout and " +
		"hibernation_wake_up_interval from the configuration."
)

// phpVersionFromMajor rebuilds php_version from the major version the API
// reports: php_version is written as "8.4:1" but read back only as "8.4", and
// every PHP version the API accepts is of the form "<major>:1".
func phpVersionFromMajor(major string) types.String {
	if major == "" {
		return types.StringNull()
	}
	return types.StringValue(major + ":1")
}

// The helpers below resolve an unknown value on a write-only Optional+Computed
// attribute. Terraform requires every Computed attribute to be known once apply
// finishes, but these attributes are never returned by the API, so null -- "the
// user did not set this" -- is the only honest resolution.

func resolveUnknownStringToNull(v *types.String) {
	if v.IsUnknown() {
		*v = types.StringNull()
	}
}

func resolveUnknownBoolToNull(v *types.Bool) {
	if v.IsUnknown() {
		*v = types.BoolNull()
	}
}

func resolveUnknownInt64ToNull(v *types.Int64) {
	if v.IsUnknown() {
		*v = types.Int64Null()
	}
}

// vanityLabelPattern is the API's rule for a vanity domain name: one
// lowercase DNS label.
var vanityLabelPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// setVanityFromAPI records the environment's vanity domain. The API takes a
// subdomain label but reports the full hostname, whose zone differs between
// organizations (laravel.cloud, laravel-demo.cloud, ...), so vanity_domain is
// the hostname's first label and vanity_hostname the whole of it.
func setVanityFromAPI(m *EnvironmentResourceModel, env *client.EnvironmentData) {
	host := env.Attributes.VanityDomain
	if host == nil || *host == "" {
		m.VanityDomain = types.StringNull()
		m.VanityHostname = types.StringNull()
		return
	}
	label, _, _ := strings.Cut(*host, ".")
	m.VanityDomain = types.StringValue(label)
	m.VanityHostname = types.StringValue(*host)
}

// applyVanityDomain points the environment at the configured vanity domain
// label and returns the environment the API reported back.
//
// The API exposes this as PUT /environments/{id}/vanity-domain rather than a
// field on the environment update body, so it is a separate call. A null value
// means "keep whatever the platform assigned" -- there is no endpoint to clear
// one, so nothing is sent.
func applyVanityDomain(ctx context.Context, c *client.Client, environmentID string, vanity types.String) (*client.EnvironmentData, error) {
	if vanity.IsNull() || vanity.IsUnknown() || vanity.ValueString() == "" {
		return nil, nil
	}
	return c.SetVanityDomain(ctx, environmentID, client.UpdateVanityDomainRequest{
		Name: vanity.ValueString(),
	})
}
