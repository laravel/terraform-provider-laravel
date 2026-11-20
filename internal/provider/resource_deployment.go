package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	client *client.Client
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
		Description: "Initiates a deployment on a Laravel Cloud environment. While the environment's resources " +
			"(such as a cache or database) are still provisioning, Laravel Cloud queues the deployment instead of " +
			"starting it; Terraform waits for it to start, for up to the 30 minutes Laravel Cloud keeps it queued, " +
			"and records the deployment it started.",
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

func (r *DeploymentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DeploymentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DeploymentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deployment, queued, err := r.client.CreateDeployment(ctx, plan.EnvironmentID.ValueString())
	if err == nil && queued != nil {
		deployment, err = waitForQueuedDeployment(ctx, r.client, queued.ID)
	}
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

	id := state.ID.ValueString()
	// Provider 1.4.0 and earlier stored the id of a queued deployment, which
	// GET /deployments does not know. Follow it to the deployment it started.
	if strings.HasPrefix(id, queuedDeploymentIDPrefix) {
		queued, err := r.client.GetQueuedDeployment(ctx, id)
		if err != nil {
			if client.IsNotFound(err) {
				resp.State.RemoveResource(ctx)
				return
			}
			resp.Diagnostics.AddError("Error reading queued deployment", err.Error())
			return
		}
		id = queued.Relationships.Deployment.RelatedID()
		if id == "" {
			// Still waiting, or replaced by a newer request: there is no
			// deployment to read, and requesting another is not this read's call.
			state.Status = types.StringValue(queued.Attributes.Status)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}

	deployment, err := r.client.GetDeployment(ctx, id)
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

// queuedDeploymentIDPrefix starts the id of every queued deployment.
const queuedDeploymentIDPrefix = "qdepl"

// queuedDeploymentPollInterval is how often Create checks whether a queued
// deployment has started. A variable so the plan tests need not wait.
var queuedDeploymentPollInterval = 5 * time.Second

// queuedDeploymentPollAttempts covers the 30 minutes Laravel Cloud keeps a
// deployment queued before it expires it, with some margin.
const queuedDeploymentPollAttempts = 400

var errDeploymentQueued = errors.New("deployment is still queued")

// waitForQueuedDeployment waits until Laravel Cloud starts a queued deployment,
// which it does once the environment's resources finish provisioning, and
// returns the deployment it started.
func waitForQueuedDeployment(ctx context.Context, c *client.Client, id string) (*client.DeploymentData, error) {
	var queued *client.QueuedDeploymentData
	err := client.RetryOnConflict(ctx, queuedDeploymentPollAttempts, queuedDeploymentPollInterval, func() error {
		var err error
		queued, err = c.GetQueuedDeployment(ctx, id)
		if err != nil {
			return err
		}
		if queued.Attributes.Status == "pending" {
			return errDeploymentQueued
		}
		return nil
	}, func(err error) bool { return errors.Is(err, errDeploymentQueued) })
	if err != nil {
		return nil, fmt.Errorf("waiting for queued deployment %s to start: %w", id, err)
	}

	deploymentID := queued.Relationships.Deployment.RelatedID()
	if queued.Attributes.Status != "promoted" || deploymentID == "" {
		reason := "it is " + queued.Attributes.Status
		if queued.Attributes.FailureReason != nil && *queued.Attributes.FailureReason != "" {
			reason = *queued.Attributes.FailureReason
		}
		return nil, fmt.Errorf("queued deployment %s did not start: %s", id, reason)
	}
	return c.GetDeployment(ctx, deploymentID)
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
