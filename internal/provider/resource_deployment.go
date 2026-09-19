package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                = &DeploymentResource{}
	_ resource.ResourceWithImportState = &DeploymentResource{}
)

type DeploymentResource struct {
	resourceWithClient
}

type DeploymentResourceModel struct {
	ID              types.String `tfsdk:"id"`
	EnvironmentID   types.String `tfsdk:"environment_id"`
	Status          types.String `tfsdk:"status"`
	BranchName      types.String `tfsdk:"branch_name"`
	CommitHash      types.String `tfsdk:"commit_hash"`
	CommitMessage   types.String `tfsdk:"commit_message"`
	CommitAuthor    types.String `tfsdk:"commit_author"`
	FailureReason   types.String `tfsdk:"failure_reason"`
	PHPMajorVersion types.String `tfsdk:"php_major_version"`
	BuildCommand    types.String `tfsdk:"build_command"`
	NodeVersion     types.String `tfsdk:"node_version"`
	UsesOctane      types.Bool   `tfsdk:"uses_octane"`
	StartedAt       types.String `tfsdk:"started_at"`
	FinishedAt      types.String `tfsdk:"finished_at"`
}

func NewDeploymentResource() resource.Resource {
	return &DeploymentResource{}
}

func (r *DeploymentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_deployment"
}

func (r *DeploymentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Initiates a deployment on a Laravel Cloud environment.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				Required:    true,
				Description: "Environment ID to deploy.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Deployment status.",
			},
			"branch_name": schema.StringAttribute{
				Computed:    true,
				Description: "Branch name used for the deployment.",
			},
			"commit_hash": schema.StringAttribute{
				Computed:    true,
				Description: "Commit hash of the deployment.",
			},
			"commit_message": schema.StringAttribute{
				Computed:    true,
				Description: "Commit message.",
			},
			"commit_author": schema.StringAttribute{
				Computed:    true,
				Description: "Commit author.",
			},
			"failure_reason": schema.StringAttribute{
				Computed:    true,
				Description: "Reason for failure, if any.",
			},
			"php_major_version": schema.StringAttribute{
				Computed:    true,
				Description: "PHP version used for the deployment.",
			},
			"build_command": schema.StringAttribute{
				Computed:    true,
				Description: "Build command executed during deployment.",
			},
			"node_version": schema.StringAttribute{
				Computed:    true,
				Description: "Node.js version used for the deployment.",
			},
			"uses_octane": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the deployment uses Laravel Octane.",
			},
			"started_at": schema.StringAttribute{
				Computed:    true,
				Description: "Deployment start timestamp.",
			},
			"finished_at": schema.StringAttribute{
				Computed:    true,
				Description: "Deployment finish timestamp.",
			},
		},
	}
}

func (r *DeploymentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DeploymentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deployment, err := r.client.CreateDeployment(ctx, plan.EnvironmentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error creating deployment", err.Error())
		return
	}

	mapDeploymentToState(deployment, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DeploymentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DeploymentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deployment, err := r.client.GetDeployment(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading deployment", err.Error())
		return
	}

	mapDeploymentToState(deployment, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DeploymentResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "Deployments are immutable. Create a new deployment instead.")
}

func (r *DeploymentResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Deployments cannot be deleted. Remove from state only.
	resp.Diagnostics.AddWarning(
		"Deployment not deleted from Laravel Cloud",
		"The Laravel Cloud API does not support deleting deployments. "+
			"The resource has been removed from Terraform state.",
	)
}

func (r *DeploymentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mapDeploymentToState(d *client.DeploymentData, state *DeploymentResourceModel) {
	// environment_id is Required and forces replacement; recover it from the
	// relationship so an imported deployment does not trigger a fresh one.
	if envID := d.Relationships.Environment.RelatedID(); envID != "" {
		state.EnvironmentID = types.StringValue(envID)
	}
	state.ID = types.StringValue(d.ID)
	state.Status = types.StringValue(d.Attributes.Status)
	state.BranchName = types.StringValue(d.Attributes.BranchName)
	state.CommitHash = types.StringValue(d.Attributes.CommitHash)
	state.CommitMessage = types.StringValue(d.Attributes.CommitMessage)
	state.CommitAuthor = types.StringValue(d.Attributes.CommitAuthor)
	state.FailureReason = types.StringPointerValue(d.Attributes.FailureReason)
	state.PHPMajorVersion = types.StringValue(d.Attributes.PHPMajorVersion)
	state.BuildCommand = types.StringValue(d.Attributes.BuildCommand)
	state.NodeVersion = types.StringValue(d.Attributes.NodeVersion)
	state.UsesOctane = types.BoolValue(d.Attributes.UsesOctane)
	state.StartedAt = types.StringPointerValue(d.Attributes.StartedAt)
	state.FinishedAt = types.StringPointerValue(d.Attributes.FinishedAt)
}
