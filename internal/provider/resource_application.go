package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
				Description: "Source code repository (e.g. laravel/laravel). " +
					"Use the owner/name form the platform reports: after an import, a URL or different casing shows as a change, and applying it updates the application's repository.",
			},
			"region": schema.StringAttribute{
				Required:    true,
				Description: "Cloud region (e.g. us-east-1).",
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
				Description: "Dedicated cluster ID. " +
					"The API never reports it back, so the first plan after an import shows it being added; applying that plan only records the value.",
				PlanModifiers: []planmodifier.String{
					requiresReplaceUnlessImported(),
					warnIfNotReported(),
				},
			},
			"source_control_provider_type": schema.StringAttribute{
				Optional: true,
				Description: "Source control provider type (github, gitlab, gitlab_self_hosted, or bitbucket). " +
					"Becomes required by the API on March 9, 2026. The API never reports it back, so the first plan after an import shows it being added; applying that plan only records the value.",
				Validators: []validator.String{
					stringvalidator.OneOf("github", "gitlab", "gitlab_self_hosted", "bitbucket"),
				},
				PlanModifiers: []planmodifier.String{
					warnIfNotReported(),
				},
			},
			"slack_channel": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Slack channel for application notifications.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
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
	if !plan.ClusterID.IsNull() {
		v := plan.ClusterID.ValueString()
		createReq.ClusterID = &v
	}
	if !plan.SourceControlProviderType.IsNull() {
		v := plan.SourceControlProviderType.ValueString()
		createReq.SourceControlProviderType = &v
	}

	app, err := r.client.CreateApplication(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating application", err.Error())
		return
	}

	plan.ID = types.StringValue(app.ID)
	plan.Slug = types.StringValue(app.Attributes.Slug)
	plan.SlackChannel = types.StringPointerValue(app.Attributes.SlackChannel)
	plan.AvatarURL = types.StringPointerValue(app.Attributes.AvatarURL)
	plan.CreatedAt = types.StringPointerValue(app.Attributes.CreatedAt)

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

	state.Name = types.StringValue(app.Attributes.Name)
	state.Slug = types.StringValue(app.Attributes.Slug)
	state.Region = types.StringValue(app.Attributes.Region)
	state.SlackChannel = types.StringPointerValue(app.Attributes.SlackChannel)
	state.AvatarURL = types.StringPointerValue(app.Attributes.AvatarURL)
	state.CreatedAt = types.StringPointerValue(app.Attributes.CreatedAt)

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
		v := plan.SlackChannel.ValueString()
		updateReq.SlackChannel = &v
	}
	// The API reads source_control_provider_type only to resolve a repository
	// being changed and discards it otherwise, so it goes out only alongside
	// one. Adding it on its own then changes nothing, as its plan warning says.
	if updateReq.Repository != nil && !plan.SourceControlProviderType.IsNull() {
		v := plan.SourceControlProviderType.ValueString()
		updateReq.SourceControlProviderType = &v
	}

	app, err := r.client.UpdateApplication(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating application", err.Error())
		return
	}

	plan.ID = state.ID
	plan.Slug = types.StringValue(app.Attributes.Slug)
	plan.SlackChannel = types.StringPointerValue(app.Attributes.SlackChannel)
	plan.AvatarURL = types.StringPointerValue(app.Attributes.AvatarURL)
	plan.CreatedAt = types.StringPointerValue(app.Attributes.CreatedAt)

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
