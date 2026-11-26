package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                = &ApplicationResource{}
	_ resource.ResourceWithImportState = &ApplicationResource{}
)

type ApplicationResource struct {
	client *client.Client
}

type ApplicationResourceModel struct {
	ID                        types.String `tfsdk:"id"`
	Name                      types.String `tfsdk:"name"`
	Repository                types.String `tfsdk:"repository"`
	Region                    types.String `tfsdk:"region"`
	Slug                      types.String `tfsdk:"slug"`
	ClusterID                 types.String `tfsdk:"cluster_id"`
	SourceControlProviderType types.String `tfsdk:"source_control_provider_type"`
	SlackChannel              types.String `tfsdk:"slack_channel"`
	AvatarURL                 types.String `tfsdk:"avatar_url"`
	CreatedAt                 types.String `tfsdk:"created_at"`
	StarterKit                types.String `tfsdk:"starter_kit"`
	RootDirectory             types.String `tfsdk:"root_directory"`
	DefaultEnvironmentID      types.String `tfsdk:"default_environment_id"`
	ServingMode               types.String `tfsdk:"serving_mode"`
	ServingRoot               types.String `tfsdk:"serving_root"`
	BuildCommand              types.String `tfsdk:"build_command"`
	Framework                 types.String `tfsdk:"framework"`
	Language                  types.String `tfsdk:"language"`
	Notes                     types.String `tfsdk:"notes"`
	RepositoryDefaultBranch   types.String `tfsdk:"repository_default_branch"`
}

func NewApplicationResource() resource.Resource {
	return &ApplicationResource{}
}

func (r *ApplicationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_application"
}

func (r *ApplicationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Laravel Cloud application.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Application ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Application name (3-40 characters).",
			},
			"repository": schema.StringAttribute{
				Required: true,
				Description: "Source code repository (such as laravel/laravel). " +
					"Use the owner/name form the platform reports: after an import, a URL or different casing shows as a change, and applying it updates the application's repository.",
			},
			"region": schema.StringAttribute{
				Required:    true,
				Description: "Cloud region (such as us-east-1).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"slug": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				Description: "URL-friendly slug.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cluster_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "ID of the dedicated cluster to create the application's default environment on. Used only " +
					"when the application is created: changing it afterwards only records the new value, so move an " +
					"environment with cluster_id on laravel_cloud_environment. An import reads it from the default environment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"source_control_provider_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Source control provider of the application's repository (github, gitlab, gitlab_self_hosted, " +
					"or bitbucket). Becomes required by the API on March 9, 2026. Leave unset to keep the reported one. " +
					"Changing it points the application at the repository of the same name on the other provider.",
				Validators: []validator.String{
					stringvalidator.OneOf("github", "gitlab", "gitlab_self_hosted", "bitbucket"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"starter_kit": schema.StringAttribute{
				Optional: true,
				Description: "Create the repository from a starter kit instead of using an existing one: laravel, " +
					"laravel-react, laravel-vue, laravel-livewire, laravel-svelte, nextjs, nuxt, express, go, django, " +
					"fastapi, rails or astro. repository then names a new GitHub repository (owner/name) to create, " +
					"which must not exist yet, and source_control_provider_type must be github. " +
					"The API never reports it back, so the first plan after an import shows it being added; applying that plan only records the value.",
				Validators: []validator.String{
					stringvalidator.OneOf("laravel", "laravel-react", "laravel-vue", "laravel-livewire", "laravel-svelte",
						"nextjs", "nuxt", "express", "go", "django", "fastapi", "rails", "astro"),
					stringvalidator.ConflictsWith(path.MatchRoot("root_directory")),
				},
				PlanModifiers: []planmodifier.String{
					requiresReplaceUnlessImported(),
					warnIfNotReported(),
				},
			},
			"root_directory": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Subdirectory of the repository the application lives in, for a repository that holds " +
					"more than one project (max 255 chars). Leave unset for the repository root. " +
					"It cannot be changed after creation, so changing it forces a new application. " +
					"Laravel Cloud clears it when repository changes to one without that directory; " +
					"the apply then fails and asks for it to be removed from the configuration, rather than " +
					"replacing the application.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					rootDirectoryPlanModifier{},
				},
			},
			"slack_channel": schema.StringAttribute{
				Optional: true,
				Description: "Slack channel for application notifications. The organization must have Slack connected. " +
					"Removing it turns Slack notifications off.",
			},
			"default_environment_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "ID of the application's default (production) environment, which Laravel Cloud cannot delete. " +
					"Leave unset to keep the one the application was created with. It must be an environment of this " +
					"application that is not a preview. A laravel_cloud_environment of this application cannot be referenced " +
					"here, because the environment depends on the application; use its ID instead.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"serving_mode": schema.StringAttribute{
				Optional: true,
				Description: "How the default environment is served: \"process\" (an application server) or \"static\" " +
					"(files served directly, for a static site). Leave unset to let Laravel Cloud detect it. It cannot be " +
					"changed after creation, so changing it forces a new application. New environments of an application " +
					"with a static environment are static too. " +
					"The API never reports it back, so the first plan after an import shows it being added; applying that plan only records the value.",
				Validators: []validator.String{
					stringvalidator.OneOf("process", "static"),
				},
				PlanModifiers: []planmodifier.String{
					requiresReplaceUnlessImported(),
					warnIfNotReported(),
				},
			},
			"serving_root": schema.StringAttribute{
				Optional: true,
				Description: "Directory a static default environment serves, relative to the repository (\".\" for the " +
					"top level). Requires serving_mode = \"static\". Used only when the application is created: change it " +
					"afterwards with serving_root on laravel_cloud_environment. Changing it here only records the new value. " +
					"The API never reports it back, so the first plan after an import shows it being added; applying that plan only records the value.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
					stringvalidator.AlsoRequires(path.MatchRoot("serving_mode")),
				},
				PlanModifiers: []planmodifier.String{
					warnIfNotReported(),
				},
			},
			"build_command": schema.StringAttribute{
				Optional: true,
				Description: "Build command of a static default environment (max 2000 chars). Leave unset to build nothing. " +
					"Laravel Cloud uses it only with serving_mode = \"static\"; other applications get a build command " +
					"for their framework. Used only when the application is created: change it afterwards with " +
					"build_command on laravel_cloud_environment. Changing it here only records the new value. " +
					"The API never reports it back, so the first plan after an import shows it being added; applying that plan only records the value.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 2000),
				},
				PlanModifiers: []planmodifier.String{
					warnIfNotReported(),
				},
			},
			"framework": schema.StringAttribute{
				Computed:    true,
				Description: "Framework of the default environment, as Laravel Cloud detected it (such as laravel, nextjs).",
				PlanModifiers: []planmodifier.String{
					followsAttributes{"repository", "source_control_provider_type", "default_environment_id"},
				},
			},
			"language": schema.StringAttribute{
				Computed:    true,
				Description: "Language of the default environment (such as php, javascript).",
				PlanModifiers: []planmodifier.String{
					followsAttributes{"repository", "source_control_provider_type", "default_environment_id"},
				},
			},
			"notes": schema.StringAttribute{
				Computed:    true,
				Description: "Notes on the application, set in the dashboard. Null when there are none or the API token cannot view them.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"repository_default_branch": schema.StringAttribute{
				Computed:    true,
				Description: "Default branch of the repository.",
				PlanModifiers: []planmodifier.String{
					repositorySource,
				},
			},
			"avatar_url": schema.StringAttribute{
				Computed:    true,
				Description: "Application avatar URL.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *ApplicationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ApplicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ApplicationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateApplicationRequest{
		Name:       plan.Name.ValueString(),
		Repository: plan.Repository.ValueString(),
		Region:     plan.Region.ValueString(),
	}
	if isSet(plan.ClusterID) {
		createReq.ClusterID = plan.ClusterID.ValueStringPointer()
	}
	if isSet(plan.SourceControlProviderType) {
		createReq.SourceControlProviderType = plan.SourceControlProviderType.ValueStringPointer()
	}
	createReq.StarterKit = plan.StarterKit.ValueStringPointer()
	if isSet(plan.RootDirectory) {
		createReq.RootDirectory = plan.RootDirectory.ValueStringPointer()
	}
	if !plan.ServingMode.IsNull() {
		createReq.Serving = &client.ServingRequest{Mode: plan.ServingMode.ValueString(), Root: *nullableStringRequest(plan.ServingRoot)}
	}
	createReq.BuildCommand = plan.BuildCommand.ValueStringPointer()

	app, err := r.client.CreateApplication(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating application", err.Error())
		return
	}

	// The create endpoint takes no slug, Slack channel or default environment,
	// so configured ones are set by an update straight after. The application
	// exists by then, so a failure is a warning: state keeps the configured
	// values, and the next refresh shows the real ones and plans the change
	// again.
	var followUp client.UpdateApplicationRequest
	if isSet(plan.Slug) && plan.Slug.ValueString() != app.Attributes.Slug {
		followUp.Slug = plan.Slug.ValueStringPointer()
	}
	if !plan.SlackChannel.IsNull() {
		followUp.SlackChannel = nullableStringRequest(plan.SlackChannel)
	}
	if isSet(plan.DefaultEnvironmentID) {
		followUp.DefaultEnvironmentID = plan.DefaultEnvironmentID.ValueStringPointer()
	}
	if followUp != (client.UpdateApplicationRequest{}) {
		updated, err := r.client.UpdateApplication(ctx, app.ID, followUp)
		if err != nil {
			resp.Diagnostics.AddWarning("Could not apply all settings after creating the application",
				"The application was created, but setting its slug, slack_channel or default_environment_id failed: "+
					err.Error()+". Run 'terraform apply' again to retry.")
		} else {
			app = updated
		}
	}
	// The create and update responses carry no relationships, so the default
	// environment, and the cluster it runs on, come from a read.
	if plan.DefaultEnvironmentID.IsUnknown() || plan.ClusterID.IsUnknown() {
		withEnv, err := r.client.GetApplication(ctx, app.ID)
		if err != nil {
			withEnv = &client.ApplicationData{}
			resp.Diagnostics.AddAttributeWarning(path.Root("default_environment_id"), "Could not read the default environment",
				"The application was created, but reading it back failed: "+err.Error()+". The next refresh records it.")
		}
		if plan.DefaultEnvironmentID.IsUnknown() {
			plan.DefaultEnvironmentID = defaultEnvironmentFromAPI(withEnv)
		}
		if plan.ClusterID.IsUnknown() {
			plan.ClusterID = types.StringPointerValue(withEnv.DefaultEnvironmentClusterID)
		}
	}

	plan.ID = types.StringValue(app.ID)
	rememberAppRepository(app.ID, plan.Repository)
	if !isSet(plan.Slug) {
		plan.Slug = types.StringValue(app.Attributes.Slug)
	}
	plan.AvatarURL = types.StringPointerValue(app.Attributes.AvatarURL)
	plan.CreatedAt = types.StringPointerValue(app.Attributes.CreatedAt)
	plan.RootDirectory = types.StringPointerValue(app.Attributes.RootDirectory)
	setUnknownApplicationDetails(&plan, app)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ApplicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ApplicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := r.client.GetApplication(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading application", err.Error())
		return
	}

	// Read keeps the configured repository string, which may differ from the
	// reported one in form, but records a lost one: Laravel Cloud can leave an
	// application with no repository, and the next apply sets it again.
	rememberAppRepository(state.ID.ValueString(), state.Repository)
	if app.Attributes.Repository == nil {
		state.Repository = types.StringNull()
	}
	state.Name = types.StringValue(app.Attributes.Name)
	state.Slug = types.StringValue(app.Attributes.Slug)
	state.Region = types.StringValue(app.Attributes.Region)
	state.SlackChannel = types.StringPointerValue(app.Attributes.SlackChannel)
	state.AvatarURL = types.StringPointerValue(app.Attributes.AvatarURL)
	state.CreatedAt = types.StringPointerValue(app.Attributes.CreatedAt)
	state.DefaultEnvironmentID = defaultEnvironmentFromAPI(app)
	state.Framework = types.StringPointerValue(app.Attributes.Framework)
	state.Language = types.StringPointerValue(app.Attributes.Language)
	state.Notes = types.StringPointerValue(app.Attributes.Notes)
	state.RepositoryDefaultBranch = repositoryDefaultBranch(app)
	state.SourceControlProviderType = types.StringPointerValue(app.Attributes.SourceControlProviderType)
	// cluster_id only placed the default environment at creation, so a
	// recorded value stands. With none, as after an import, it is the default
	// environment's.
	if state.ClusterID.IsNull() {
		state.ClusterID = types.StringPointerValue(app.DefaultEnvironmentClusterID)
	}
	if app.Attributes.RootDirectory == nil && isSet(state.RootDirectory) {
		resp.Diagnostics.Append(setRootDirectoryCleared(ctx, resp.Private, state.RootDirectory.ValueString())...)
	}
	state.RootDirectory = types.StringPointerValue(app.Attributes.RootDirectory)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ApplicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ApplicationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state ApplicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := client.UpdateApplicationRequest{}
	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		updateReq.Name = &v
	}
	if !plan.Slug.IsUnknown() && !plan.Slug.Equal(state.Slug) {
		v := plan.Slug.ValueString()
		updateReq.Slug = &v
	}
	if !plan.Repository.Equal(state.Repository) {
		v := plan.Repository.ValueString()
		updateReq.Repository = &v
	}
	if !plan.SlackChannel.Equal(state.SlackChannel) {
		updateReq.SlackChannel = nullableStringRequest(plan.SlackChannel)
	}
	if isSet(plan.DefaultEnvironmentID) && !plan.DefaultEnvironmentID.Equal(state.DefaultEnvironmentID) {
		updateReq.DefaultEnvironmentID = plan.DefaultEnvironmentID.ValueStringPointer()
	}
	// The API reads source_control_provider_type only to resolve a repository
	// being changed, and discards it otherwise. So a new provider type goes out
	// with the repository, which the API then looks up on that provider.
	if isSet(plan.SourceControlProviderType) && !plan.SourceControlProviderType.Equal(state.SourceControlProviderType) &&
		updateReq.Repository == nil && isSet(plan.Repository) {
		updateReq.Repository = plan.Repository.ValueStringPointer()
	}
	if updateReq.Repository != nil && isSet(plan.SourceControlProviderType) {
		updateReq.SourceControlProviderType = plan.SourceControlProviderType.ValueStringPointer()
	}

	app, err := r.client.UpdateApplication(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating application", err.Error())
		return
	}

	plan.ID = state.ID
	rememberAppRepository(plan.ID.ValueString(), plan.Repository)
	plan.Slug = types.StringValue(app.Attributes.Slug)
	setUnknownApplicationDetails(&plan, app)
	// The update response doesn't include the default environment. Unknown
	// means none was recorded or configured; the next refresh fills it in.
	if plan.ClusterID.IsUnknown() {
		plan.ClusterID = types.StringNull()
	}
	if plan.DefaultEnvironmentID.IsUnknown() {
		plan.DefaultEnvironmentID = defaultEnvironmentFromAPI(app)
	}
	plan.AvatarURL = types.StringPointerValue(app.Attributes.AvatarURL)
	plan.CreatedAt = types.StringPointerValue(app.Attributes.CreatedAt)
	// root_directory cannot be set in place, but Laravel Cloud clears it when
	// the new repository lacks that directory.
	switch {
	case plan.RootDirectory.IsUnknown():
		// Unset in the configuration and the repository changed.
		plan.RootDirectory = types.StringPointerValue(app.Attributes.RootDirectory)
	case isSet(plan.RootDirectory) && app.Attributes.RootDirectory == nil:
		// The configured directory can no longer hold, and replacing the
		// application would delete its environments. State records what Cloud
		// did, and the next plan refuses to replace over it.
		dir := plan.RootDirectory.ValueString()
		plan.RootDirectory = types.StringNull()
		resp.Diagnostics.Append(setRootDirectoryCleared(ctx, resp.Private, dir)...)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		resp.Diagnostics.AddAttributeError(path.Root("root_directory"), "Laravel Cloud cleared root_directory",
			rootDirectoryClearedDetail(dir))
		return
	}
	// Otherwise the planned value stands: after an upgrade it is planned as its
	// null prior state, and the next refresh records the reported value.

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ApplicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ApplicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteApplication(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting application", err.Error())
	}
}

func (r *ApplicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read never rewrites repository, so an import would leave it null and the
	// first plan would propose adding it. It is seeded here instead. The API
	// reports it as an object, not the "owner/name" string sent on create.
	app, err := r.client.GetApplication(ctx, req.ID)
	if err != nil {
		if client.IsNotFound(err) {
			return // Read reports the missing application.
		}
		resp.Diagnostics.AddError("Error importing application", err.Error())
		return
	}
	if repo := app.Attributes.Repository; repo != nil && repo.FullName != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("repository"), repo.FullName)...)
	}
}

// rootDirectoryClearedKey is the private state key holding the root_directory
// Laravel Cloud cleared after a repository change, as a JSON string.
const rootDirectoryClearedKey = "root_directory_cleared"

// privateSetter is the private state of a Read or Update response.
type privateSetter interface {
	SetKey(context.Context, string, []byte) diag.Diagnostics
}

func setRootDirectoryCleared(ctx context.Context, p privateSetter, dir string) diag.Diagnostics {
	if p == nil {
		return nil
	}
	b, _ := json.Marshal(dir)
	return p.SetKey(ctx, rootDirectoryClearedKey, b)
}

func rootDirectoryClearedDetail(dir string) string {
	return fmt.Sprintf("The repository has no %q directory, so Laravel Cloud cleared root_directory and the application "+
		"now builds from the repository root. root_directory cannot be set in place, and replacing the application "+
		"would delete its environments, so Terraform will not do either. Remove root_directory from the "+
		"configuration to accept the repository root.", dir)
}

// repositorySource names the attributes that pick the repository Laravel Cloud
// resolves: changing source_control_provider_type alone sends the current
// repository with it, which then names the same-named repository on the new
// provider. Whatever Cloud reads from the repository follows them.
var repositorySource = followsAttributes{"repository", "source_control_provider_type"}

// rootDirectoryPlanModifier replaces the application when root_directory is
// changed, except over a directory Laravel Cloud itself cleared, and reads it
// back when a repository change may clear it.
type rootDirectoryPlanModifier struct{}

func (rootDirectoryPlanModifier) Description(context.Context) string {
	return "Requires replacement when changed, unless Laravel Cloud cleared the configured value."
}

func (m rootDirectoryPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (rootDirectoryPlanModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if req.ConfigValue.IsNull() {
		// Kept from state, but a new repository may clear it, and so may a new
		// provider type, which points at the same-named repository there.
		if !req.StateValue.IsNull() && !repositorySource.keep(ctx, req.State, req.Plan, req.Config, req.ConfigValue) {
			resp.PlanValue = types.StringUnknown()
		}
		return
	}
	if req.PlanValue.Equal(req.StateValue) {
		return
	}
	if req.StateValue.IsNull() && req.Private != nil {
		b, d := req.Private.GetKey(ctx, rootDirectoryClearedKey)
		resp.Diagnostics.Append(d...)
		var cleared string
		if len(b) > 0 && json.Unmarshal(b, &cleared) == nil && cleared == req.ConfigValue.ValueString() {
			resp.Diagnostics.AddAttributeError(req.Path, "Laravel Cloud cleared root_directory", rootDirectoryClearedDetail(cleared))
			return
		}
	}
	resp.RequiresReplace = true
}

// defaultEnvironmentFromAPI is the application's default environment, which
// the API reports only when the request includes defaultEnvironment.
func defaultEnvironmentFromAPI(app *client.ApplicationData) types.String {
	if id := app.Relationships.DefaultEnvironment.RelatedID(); id != "" {
		return types.StringValue(id)
	}
	return types.StringNull()
}

func repositoryDefaultBranch(app *client.ApplicationData) types.String {
	if app.Attributes.Repository == nil {
		return types.StringNull()
	}
	return types.StringPointerValue(app.Attributes.Repository.DefaultBranch)
}

// setUnknownApplicationDetails fills in the read-only details the plan left
// unknown from an API response.
func setUnknownApplicationDetails(plan *ApplicationResourceModel, app *client.ApplicationData) {
	setUnknownStringPtrFromAPI(&plan.Framework, app.Attributes.Framework)
	setUnknownStringPtrFromAPI(&plan.Language, app.Attributes.Language)
	setUnknownStringPtrFromAPI(&plan.Notes, app.Attributes.Notes)
	setUnknownStringPtrFromAPI(&plan.SourceControlProviderType, app.Attributes.SourceControlProviderType)
	if plan.RepositoryDefaultBranch.IsUnknown() {
		plan.RepositoryDefaultBranch = repositoryDefaultBranch(app)
	}
}
