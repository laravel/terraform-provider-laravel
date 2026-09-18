package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                = &BackgroundProcessResource{}
	_ resource.ResourceWithImportState = &BackgroundProcessResource{}
)

type BackgroundProcessResource struct {
	client *client.Client
}

type BackgroundProcessResourceModel struct {
	ID                types.String `tfsdk:"id"`
	InstanceID        types.String `tfsdk:"instance_id"`
	Type              types.String `tfsdk:"type"`
	Processes         types.Int64  `tfsdk:"processes"`
	Command           types.String `tfsdk:"command"`
	Config            types.String `tfsdk:"config"`
	StrategyType      types.String `tfsdk:"strategy_type"`
	StrategyThreshold types.Int64  `tfsdk:"strategy_threshold"`
	CreatedAt         types.String `tfsdk:"created_at"`
}

func NewBackgroundProcessResource() resource.Resource {
	return &BackgroundProcessResource{}
}

func (r *BackgroundProcessResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_background_process"
}

func (r *BackgroundProcessResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a background process (worker or custom daemon) on a Laravel Cloud instance.",
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
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Process type (worker, custom).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"processes": schema.Int64Attribute{
				Required:    true,
				Description: "Number of processes (1-10).",
			},
			"command": schema.StringAttribute{
				Optional:    true,
				Description: "Custom command (required when type is custom, 3-500 chars).",
			},
			"config": schema.StringAttribute{
				Optional:    true,
				Description: "JSON-encoded worker configuration (connection, queue, tries, backoff, sleep, rest, timeout, force).",
			},
			"strategy_type": schema.StringAttribute{
				Computed:    true,
				Description: "Scaling strategy type (none, growth_rate, queue_size).",
			},
			"strategy_threshold": schema.Int64Attribute{
				Computed:    true,
				Description: "Scaling strategy threshold (null when no autoscaling strategy is set).",
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *BackgroundProcessResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *BackgroundProcessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BackgroundProcessResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateBackgroundProcessRequest{
		Type:      plan.Type.ValueString(),
		Processes: int(plan.Processes.ValueInt64()),
	}
	if !plan.Command.IsNull() {
		v := plan.Command.ValueString()
		createReq.Command = &v
	}
	if !plan.Config.IsNull() {
		var cfg map[string]any
		if err := json.Unmarshal([]byte(plan.Config.ValueString()), &cfg); err != nil {
			resp.Diagnostics.AddError("Invalid config JSON", err.Error())
			return
		}
		createReq.Config = cfg
	}

	bp, err := r.client.CreateBackgroundProcess(ctx, plan.InstanceID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating background process", err.Error())
		return
	}

	mapBackgroundProcessToState(bp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BackgroundProcessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BackgroundProcessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bp, err := r.client.GetBackgroundProcess(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading background process", err.Error())
		return
	}

	mapBackgroundProcessToState(bp, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *BackgroundProcessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan BackgroundProcessResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state BackgroundProcessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := client.UpdateBackgroundProcessRequest{}
	if !plan.Processes.Equal(state.Processes) {
		v := int(plan.Processes.ValueInt64())
		updateReq.Processes = &v
	}
	if !plan.Command.IsNull() && !plan.Command.Equal(state.Command) {
		v := plan.Command.ValueString()
		updateReq.Command = &v
	}
	if !plan.Config.IsNull() && !plan.Config.Equal(state.Config) {
		var cfg map[string]any
		if err := json.Unmarshal([]byte(plan.Config.ValueString()), &cfg); err != nil {
			resp.Diagnostics.AddError("Invalid config JSON", err.Error())
			return
		}
		updateReq.Config = cfg
	}

	bp, err := r.client.UpdateBackgroundProcess(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating background process", err.Error())
		return
	}

	plan.InstanceID = state.InstanceID
	mapBackgroundProcessToState(bp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BackgroundProcessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BackgroundProcessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteBackgroundProcess(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting background process", err.Error())
	}
}

func (r *BackgroundProcessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mapBackgroundProcessToState(bp *client.BackgroundProcessData, state *BackgroundProcessResourceModel) {
	if instanceID := bp.Relationships.Instance.RelatedID(); instanceID != "" {
		state.InstanceID = types.StringValue(instanceID)
	}
	state.ID = types.StringValue(bp.ID)
	state.Type = types.StringValue(bp.Attributes.ProcessType)
	state.Processes = types.Int64Value(int64(bp.Attributes.Processes))
	state.StrategyType = types.StringValue(bp.Attributes.StrategyType)
	if bp.Attributes.StrategyThreshold != nil {
		state.StrategyThreshold = types.Int64Value(int64(*bp.Attributes.StrategyThreshold))
	} else {
		state.StrategyThreshold = types.Int64Null()
	}
	state.CreatedAt = types.StringPointerValue(bp.Attributes.CreatedAt)
	if bp.Attributes.Command != nil {
		state.Command = types.StringValue(*bp.Attributes.Command)
	}
	// The API echoes config back as an array of objects whereas it is sent
	// (and stored in state) as a single JSON object, so the two shapes are
	// not comparable. Preserve the configured value rather than overwriting
	// it from the response, which would otherwise cause a perpetual diff.
}
