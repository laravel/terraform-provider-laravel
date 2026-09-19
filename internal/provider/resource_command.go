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
	_ resource.Resource                = &CommandResource{}
	_ resource.ResourceWithImportState = &CommandResource{}
)

type CommandResource struct {
	resourceWithClient
}

type CommandResourceModel struct {
	ID            types.String `tfsdk:"id"`
	EnvironmentID types.String `tfsdk:"environment_id"`
	Command       types.String `tfsdk:"command"`
	Output        types.String `tfsdk:"output"`
	Status        types.String `tfsdk:"status"`
	ExitCode      types.Int64  `tfsdk:"exit_code"`
	FailureReason types.String `tfsdk:"failure_reason"`
	StartedAt     types.String `tfsdk:"started_at"`
	FinishedAt    types.String `tfsdk:"finished_at"`
	CreatedAt     types.String `tfsdk:"created_at"`
}

func NewCommandResource() resource.Resource {
	return &CommandResource{}
}

func (r *CommandResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_command"
}

func (r *CommandResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Runs a command on a Laravel Cloud environment.",
		Attributes: map[string]schema.Attribute{
			"id": computedIDAttribute(),
			"environment_id": schema.StringAttribute{
				Required:    true,
				Description: "Environment ID to run the command on.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"command": schema.StringAttribute{
				Required:    true,
				Description: "The command to execute.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"output": schema.StringAttribute{
				Computed:    true,
				Description: "Command output.",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Command status (pending, command.created, command.running, command.failure, command.success).",
			},
			"exit_code": schema.Int64Attribute{
				Computed:    true,
				Description: "Exit code (0 = success, 1 = failure).",
			},
			"failure_reason": schema.StringAttribute{
				Computed:    true,
				Description: "Reason for failure, if any.",
			},
			"started_at": schema.StringAttribute{
				Computed:    true,
				Description: "Command start timestamp.",
			},
			"finished_at": schema.StringAttribute{
				Computed:    true,
				Description: "Command finish timestamp.",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp.",
			},
		},
	}
}

func (r *CommandResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CommandResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateCommandRequest{
		Command: plan.Command.ValueString(),
	}

	cmd, err := r.client.CreateCommand(ctx, plan.EnvironmentID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating command", err.Error())
		return
	}

	mapCommandToState(cmd, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CommandResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CommandResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cmd, err := r.client.GetCommand(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading command", err.Error())
		return
	}

	mapCommandToState(cmd, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *CommandResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "Commands are immutable. Create a new command instead.")
}

func (r *CommandResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Commands cannot be deleted. Remove from state only.
	resp.Diagnostics.AddWarning(
		"Command not deleted from Laravel Cloud",
		"The Laravel Cloud API does not support deleting commands. "+
			"The resource has been removed from Terraform state.",
	)
}

func (r *CommandResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mapCommandToState(c *client.CommandData, state *CommandResourceModel) {
	// environment_id is Required and forces replacement; recover it from the
	// relationship so an imported command is not re-run on the next plan.
	if envID := c.Relationships.Environment.RelatedID(); envID != "" {
		state.EnvironmentID = types.StringValue(envID)
	}
	state.ID = types.StringValue(c.ID)
	state.Command = types.StringValue(c.Attributes.Command)
	state.Output = types.StringPointerValue(c.Attributes.Output)
	state.Status = types.StringValue(c.Attributes.Status)
	if c.Attributes.ExitCode != nil {
		state.ExitCode = types.Int64Value(int64(*c.Attributes.ExitCode))
	} else {
		state.ExitCode = types.Int64Null()
	}
	state.FailureReason = types.StringPointerValue(c.Attributes.FailureReason)
	state.StartedAt = types.StringPointerValue(c.Attributes.StartedAt)
	state.FinishedAt = types.StringPointerValue(c.Attributes.FinishedAt)
	state.CreatedAt = types.StringPointerValue(c.Attributes.CreatedAt)
}
