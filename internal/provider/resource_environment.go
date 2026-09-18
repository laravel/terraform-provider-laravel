package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                = &EnvironmentResource{}
	_ resource.ResourceWithImportState = &EnvironmentResource{}
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
	DatabaseSchemaID           types.String `tfsdk:"database_schema_id"`
	CacheID                    types.String `tfsdk:"cache_id"`
	CreatedAt                  types.String `tfsdk:"created_at"`
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
				Optional:    true,
				Description: "Dedicated cluster ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"php_version": schema.StringAttribute{
				Optional: true,
				Description: "PHP version to run, in the API's \"major:minor\" form (e.g. \"8.4:1\"). " +
					"Read back via the computed php_major_version attribute, which reports the major version only.",
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
			"database_schema_id": schema.StringAttribute{
				Optional:    true,
				Description: "Database schema to attach.",
			},
			"cache_id": schema.StringAttribute{
				Optional:    true,
				Description: "Cache to attach.",
			},
			"created_at": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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

	existing, err := r.client.ListEnvironments(ctx, appID)
	if err == nil {
		for i := range existing {
			if existing[i].Attributes.Name == wantName {
				env = &existing[i]
				adopted = true
				break
			}
		}
	}
	// If no match was found by name, fall through and create a new one.

	if env == nil {
		createReq := client.CreateEnvironmentRequest{
			Name:   wantName,
			Branch: plan.Branch.ValueString(),
		}
		if !plan.ClusterID.IsNull() {
			v := plan.ClusterID.ValueString()
			createReq.ClusterID = &v
		}

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
		}
		// Whether PATCH succeeded or failed, we keep the plan values in
		// state. The API may not echo back the values we sent (e.g.
		// php_version returns "" even when we sent "8.4:1"), so using
		// mapEnvironmentToState here would overwrite user values and
		// cause "inconsistent result after apply".
	}

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
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

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

	// Detach database and cache from the environment before deleting so
	// downstream resources (database schema, cluster) can be cleaned up.
	if !state.DatabaseSchemaID.IsNull() || !state.CacheID.IsNull() {
		detach := client.UpdateEnvironmentRequest{}
		if !state.DatabaseSchemaID.IsNull() {
			detach.DatabaseSchemaID = client.DetachID()
		}
		if !state.CacheID.IsNull() {
			detach.CacheID = client.DetachID()
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
	if !plan.DatabaseSchemaID.Equal(state.DatabaseSchemaID) {
		req.DatabaseSchemaID = attachOrDetach(plan.DatabaseSchemaID)
		hasUpdates = true
	}
	if !plan.CacheID.Equal(state.CacheID) {
		req.CacheID = attachOrDetach(plan.CacheID)
		hasUpdates = true
	}

	if !hasUpdates {
		return nil
	}
	return &req
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
	if !plan.DatabaseSchemaID.IsNull() && !plan.DatabaseSchemaID.IsUnknown() {
		req.DatabaseSchemaID = client.AttachID(plan.DatabaseSchemaID.ValueString())
		hasUpdates = true
	}
	if !plan.CacheID.IsNull() && !plan.CacheID.IsUnknown() {
		req.CacheID = client.AttachID(plan.CacheID.ValueString())
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

	if !hasUpdates {
		return nil
	}
	return &req
}

func mapEnvironmentToState(env *client.EnvironmentData, state *EnvironmentResourceModel) {
	state.Name = types.StringValue(env.Attributes.Name)
	state.Slug = types.StringValue(env.Attributes.Slug)
	state.Status = types.StringValue(env.Attributes.Status)
	// php_version is not echoed by the API in the form it is sent, so it is
	// left untouched here to preserve the configured value; the read-only
	// major version is surfaced via php_major_version instead.
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

	// color, timeout, sleep_timeout, shutdown_timeout,
	// uses_purge_edge_cache_on_deploy and uses_vanity_domain are write-only:
	// PATCH accepts them but the response never carries them back. Writing a
	// zero value here would fight the configuration on every refresh, so the
	// configured values are left in place and drift on them is not detectable.
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
