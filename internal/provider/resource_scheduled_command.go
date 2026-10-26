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
	_ resource.Resource                = &ScheduledCommandResource{}
	_ resource.ResourceWithImportState = &ScheduledCommandResource{}
)

type ScheduledCommandResource struct {
	client *client.Client
}

type ScheduledCommandResourceModel struct {
	ID         types.String `tfsdk:"id"`
	InstanceID types.String `tfsdk:"instance_id"`
	Name       types.String `tfsdk:"name"`
	Command    types.String `tfsdk:"command"`
	Schedule   types.String `tfsdk:"schedule"`
	CreatedAt  types.String `tfsdk:"created_at"`
}

func NewScheduledCommandResource() resource.Resource {
	return &ScheduledCommandResource{}
}

func (r *ScheduledCommandResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_scheduled_command"
}

func (r *ScheduledCommandResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a command that runs on a schedule on a Laravel Cloud instance.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"instance_id": schema.StringAttribute{
				Required:    true,
				Description: "Parent instance ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Description: "A friendly name for the scheduled command (max 255 chars).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
				},
			},
			"command": schema.StringAttribute{
				Required: true,
				Description: "The command to run (max 500 chars), e.g. \"php artisan reports:send\". " +
					"Each command can be scheduled only once per instance.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 500),
				},
			},
			"schedule": schema.StringAttribute{
				Required: true,
				Description: "A five-field cron expression, evaluated in UTC (e.g. \"0 3 * * *\"). " +
					"The @yearly, @annually, @monthly, @weekly, @daily, @midnight and @hourly descriptors are also accepted.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
				},
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

func (r *ScheduledCommandResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ScheduledCommandResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ScheduledCommandResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateScheduledCommandRequest{
		Name:     plan.Name.ValueStringPointer(),
		Command:  plan.Command.ValueString(),
		Schedule: plan.Schedule.ValueString(),
	}

	sc, err := r.client.CreateScheduledCommand(ctx, plan.InstanceID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating scheduled command", err.Error())
		return
	}

	mapScheduledCommandToState(sc, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ScheduledCommandResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ScheduledCommandResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sc, err := r.client.GetScheduledCommand(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading scheduled command", err.Error())
		return
	}

	mapScheduledCommandToState(sc, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ScheduledCommandResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state ScheduledCommandResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := client.UpdateScheduledCommandRequest{}
	// name is nullable: removing it from config sends null, which clears it.
	// An omitted field would leave the old name in place for good.
	if !plan.Name.Equal(state.Name) {
		updateReq.Name = &client.NullableString{Value: plan.Name.ValueString(), Null: plan.Name.IsNull()}
	}
	if !plan.Command.Equal(state.Command) {
		updateReq.Command = plan.Command.ValueStringPointer()
	}
	if !plan.Schedule.Equal(state.Schedule) {
		updateReq.Schedule = plan.Schedule.ValueStringPointer()
	}

	sc, err := r.client.UpdateScheduledCommand(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating scheduled command", err.Error())
		return
	}

	plan.InstanceID = state.InstanceID
	mapScheduledCommandToState(sc, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ScheduledCommandResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ScheduledCommandResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteScheduledCommand(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting scheduled command", err.Error())
	}
}

func (r *ScheduledCommandResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mapScheduledCommandToState(sc *client.ScheduledCommandData, state *ScheduledCommandResourceModel) {
	if instanceID := sc.Relationships.Instance.RelatedID(); instanceID != "" {
		state.InstanceID = types.StringValue(instanceID)
	}
	state.ID = types.StringValue(sc.ID)
	state.Name = types.StringPointerValue(sc.Attributes.Name)
	state.Command = types.StringValue(sc.Attributes.Command)
	state.Schedule = types.StringValue(sc.Attributes.Schedule)
	state.CreatedAt = types.StringPointerValue(sc.Attributes.CreatedAt)
}
