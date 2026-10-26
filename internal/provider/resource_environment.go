package provider

import (
	"context"
	"encoding/json"
	"fmt"

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
	Color                      types.String `tfsdk:"color"`
	ClusterID                  types.String `tfsdk:"cluster_id"`
	PHPVersion                 types.String `tfsdk:"php_version"`
	PHPMajorVersion            types.String `tfsdk:"php_major_version"`
	VanityDomain               types.String `tfsdk:"vanity_domain"`
	NodeVersion                types.String `tfsdk:"node_version"`
	BuildCommand               types.String `tfsdk:"build_command"`
	DeployCommand              types.String `tfsdk:"deploy_command"`
	UsesPushToDeploy           types.Bool   `tfsdk:"uses_push_to_deploy"`
	UsesDeployHook             types.Bool   `tfsdk:"uses_deploy_hook"`
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
			"color": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Environment color (blue, green, orange, purple, red, yellow, cyan, gray). " +
					"Write-only: the API accepts this but never reports it back, so a change made " +
					"outside Terraform cannot be detected and state keeps the configured value.",
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
				Description: "PHP version to run, in the API's \"major:minor\" form (e.g. \"8.4:1\"). " +
					"The API reports only the major version (see php_major_version), so an import " +
					"derives this from it. A version changed outside Terraform is not detected.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"php_major_version": schema.StringAttribute{
				Computed:    true,
				Description: "Major PHP version reported by the API (e.g. \"8.4\").",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"vanity_domain": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Vanity domain hostname for the environment (3-100 characters). " +
					"Set through the API's dedicated vanity-domain endpoint rather than the " +
					"environment update body. Leave unset to keep the assigned default.",
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
				Optional:    true,
				Computed:    true,
				Description: "Custom build command (max 2000 chars).",
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
				Description: "Request timeout (5-60 seconds). Write-only: the API accepts this " +
					"but never reports it back, so drift cannot be detected.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"sleep_timeout": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Sleep timeout (1-60). Write-only: the API accepts this but never " +
					"reports it back, so drift cannot be detected.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"shutdown_timeout": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Shutdown timeout (1-600). Write-only: the API accepts this but " +
					"never reports it back, so drift cannot be detected.",
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
					"Removing it detaches the database.",
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
				Optional:    true,
				Description: "Cache to attach.",
			},
			"websocket_application_id": schema.StringAttribute{
				Optional:    true,
				Description: "WebSocket application (laravel_cloud_websocket_application) to attach. Removing it detaches the application.",
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
				Computed: true,
				Description: "How the environment is served: \"process\" (an application server) or \"static\" " +
					"(files served directly). Fixed when the environment is created. Null when the API does not report it.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
				Description: "Command that starts a dynamic application that is not PHP, e.g. \"gunicorn myproject.wsgi\" " +
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
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
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
				"environments must be checked before creating one -- otherwise a duplicate is created. "+
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

	if env == nil {
		createReq := client.CreateEnvironmentRequest{
			Name:   wantName,
			Branch: plan.Branch.ValueString(),
		}
		if !plan.ClusterID.IsNull() {
			v := plan.ClusterID.ValueString()
			createReq.ClusterID = &v
		}
		createReq.EdgeNetworkID = plan.EdgeNetworkID.ValueStringPointer()

		created, err := r.client.CreateEnvironment(ctx, appID, createReq)
		if err != nil {
			resp.Diagnostics.AddError("Error creating environment", err.Error())
			return
		}
		env = created
	}

	// Set server-generated fields from the API response.
	plan.ID = types.StringValue(env.ID)
	plan.Slug = types.StringValue(env.Attributes.Slug)
	plan.Status = types.StringValue(env.Attributes.Status)
	plan.PHPMajorVersion = types.StringValue(env.Attributes.PHPMajorVersion)
	plan.VanityDomain = types.StringPointerValue(env.Attributes.VanityDomain)
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
	setUnknownSettingsFromAPI(&plan, env)

	// The remaining Optional+Computed attributes are write-only: the API
	// accepts them on PATCH but never returns them, so there is no value to
	// adopt. Resolve them to null rather than to a fabricated zero value,
	// which would then be re-sent on every apply.
	resolveUnknownStringToNull(&plan.Color)
	resolveUnknownBoolToNull(&plan.UsesVanityDomain)
	resolveUnknownBoolToNull(&plan.UsesPurgeEdgeCacheOnDeploy)
	resolveUnknownInt64ToNull(&plan.Timeout)
	resolveUnknownInt64ToNull(&plan.SleepTimeout)
	resolveUnknownInt64ToNull(&plan.ShutdownTimeout)

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
		if err := applyVanityDomain(ctx, r.client, env.ID, configuredVanity); err != nil {
			resp.Diagnostics.AddWarning(
				"Could not set the vanity domain after creating the environment",
				"The environment was created, but setting its vanity domain failed: "+err.Error()+
					". The configured value is kept in state; run 'terraform apply' again to retry.",
			)
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
		patched, err := r.client.UpdateEnvironment(ctx, env.ID, *patchReq)
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

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
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
		if err := applyVanityDomain(ctx, r.client, state.ID.ValueString(), plan.VanityDomain); err != nil {
			resp.Diagnostics.AddError("Error setting vanity domain", err.Error())
			return
		}
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
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	_, err := r.client.UpdateEnvironment(ctx, state.ID.ValueString(), *updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating environment", err.Error())
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
	plan.Slug = state.Slug
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envID := state.ID.ValueString()

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
	// color is write-only: it is never echoed back, so it cannot be compared
	// against the response. Send it whenever the user set it.
	if v := plan.Color.ValueString(); !plan.Color.IsNull() && !plan.Color.IsUnknown() {
		req.Color = &v
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
	if !plan.Timeout.IsNull() && !plan.Timeout.IsUnknown() {
		v := int(plan.Timeout.ValueInt64())
		req.Timeout = &v
		hasUpdates = true
	}
	if !plan.SleepTimeout.IsNull() && !plan.SleepTimeout.IsUnknown() {
		v := int(plan.SleepTimeout.ValueInt64())
		req.SleepTimeout = &v
		hasUpdates = true
	}
	if !plan.ShutdownTimeout.IsNull() && !plan.ShutdownTimeout.IsUnknown() {
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
	// php_version is not echoed by the API in the form it is sent, so it is
	// left untouched here to preserve the configured value; the read-only
	// major version is surfaced via php_major_version instead. An import
	// seeds it, see ImportState.
	state.PHPMajorVersion = types.StringValue(env.Attributes.PHPMajorVersion)
	state.VanityDomain = types.StringPointerValue(env.Attributes.VanityDomain)
	state.NodeVersion = types.StringValue(env.Attributes.NodeVersion)
	state.UsesPushToDeploy = types.BoolValue(env.Attributes.UsesPushToDeploy)
	state.UsesDeployHook = types.BoolValue(env.Attributes.UsesDeployHook)
	state.UsesOctane = types.BoolValue(env.Attributes.UsesOctane)
	// cache_strategy is readable only from network_settings; there is no
	// top-level attribute of that name in the response.
	state.CacheStrategy = types.StringValue(env.Attributes.NetworkSettings.Cache.Strategy)
	state.CreatedAt = types.StringPointerValue(env.Attributes.CreatedAt)
	state.BuildCommand = types.StringPointerValue(env.Attributes.BuildCommand)
	state.DeployCommand = types.StringPointerValue(env.Attributes.DeployCommand)
	mapSettingsToState(env, state)

	// color, timeout, sleep_timeout, shutdown_timeout,
	// uses_purge_edge_cache_on_deploy and uses_vanity_domain are write-only:
	// PATCH accepts them but the response never carries them back. Writing a
	// zero value here would fight the configuration on every refresh, so the
	// configured values are left in place and drift on them is not detectable.
}

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

// applyVanityDomain points the environment at the configured vanity hostname.
//
// The API exposes this as PUT /environments/{id}/vanity-domain rather than a
// field on the environment update body, so it is a separate call. A null value
// means "keep whatever the platform assigned" -- there is no endpoint to clear
// one, so nothing is sent.
func applyVanityDomain(ctx context.Context, c *client.Client, environmentID string, vanity types.String) error {
	if vanity.IsNull() || vanity.IsUnknown() || vanity.ValueString() == "" {
		return nil
	}
	_, err := c.SetVanityDomain(ctx, environmentID, client.UpdateVanityDomainRequest{
		Name: vanity.ValueString(),
	})
	return err
}
