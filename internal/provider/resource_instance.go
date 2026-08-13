package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                = &InstanceResource{}
	_ resource.ResourceWithImportState = &InstanceResource{}
)

type InstanceResource struct {
	client *client.Client
}

type InstanceResourceModel struct {
	ID                               types.String `tfsdk:"id"`
	EnvironmentID                    types.String `tfsdk:"environment_id"`
	Name                             types.String `tfsdk:"name"`
	Type                             types.String `tfsdk:"type"`
	Size                             types.String `tfsdk:"size"`
	ScalingType                      types.String `tfsdk:"scaling_type"`
	MinReplicas                      types.Int64  `tfsdk:"min_replicas"`
	MaxReplicas                      types.Int64  `tfsdk:"max_replicas"`
	UsesScheduler                    types.Bool   `tfsdk:"uses_scheduler"`
	ScalingCPUThresholdPercentage    types.Int64  `tfsdk:"scaling_cpu_threshold_percentage"`
	ScalingMemoryThresholdPercentage types.Int64  `tfsdk:"scaling_memory_threshold_percentage"`
	SleepWithApp                     types.Bool   `tfsdk:"sleep_with_app"`
	VisibilityTimeout                types.Int64  `tfsdk:"visibility_timeout"`
	PollingInterval                  types.Int64  `tfsdk:"polling_interval"`
	ShutdownTimeout                  types.Int64  `tfsdk:"shutdown_timeout"`
	UsesOctane                       types.Bool   `tfsdk:"uses_octane"`
	UsesInertiaSSR                   types.Bool   `tfsdk:"uses_inertia_ssr"`
	HibernationTimeout               types.Int64  `tfsdk:"hibernation_timeout"`
	Paused                           types.Bool   `tfsdk:"paused"`
	IsDefault                        types.Bool   `tfsdk:"is_default"`
	QueueStatus                      types.String `tfsdk:"queue_status"`
	CreatedAt                        types.String `tfsdk:"created_at"`
}

func NewInstanceResource() resource.Resource {
	return &InstanceResource{}
}

func (r *InstanceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_instance"
}

func (r *InstanceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Laravel Cloud instance (compute workload).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				Required:    true,
				Description: "Parent environment ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Instance name (3-40 characters).",
			},
			"type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("service"),
				Description: "Instance type (service or managed_queue). Defaults to service when not set.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"size": schema.StringAttribute{
				Required:    true,
				Description: "Instance size (e.g. flex.c-1vcpu-256mb).",
			},
			"scaling_type": schema.StringAttribute{
				Required:    true,
				Description: "Scaling type (none, custom, auto).",
			},
			"min_replicas": schema.Int64Attribute{
				Required:    true,
				Description: "Minimum number of replicas.",
			},
			"max_replicas": schema.Int64Attribute{
				Required:    true,
				Description: "Maximum number of replicas.",
			},
			"uses_scheduler": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"scaling_cpu_threshold_percentage": schema.Int64Attribute{
				Optional:    true,
				Description: "CPU scaling threshold (50-95).",
			},
			"scaling_memory_threshold_percentage": schema.Int64Attribute{
				Optional:    true,
				Description: "Memory scaling threshold (50-95).",
			},
			"sleep_with_app": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the instance sleeps with the app (managed_queue).",
			},
			"visibility_timeout": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Queue visibility timeout in seconds (managed_queue).",
			},
			"polling_interval": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Queue polling interval in seconds (managed_queue).",
			},
			"shutdown_timeout": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Queue shutdown timeout in seconds (managed_queue).",
			},
			"uses_octane": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether the instance uses Laravel Octane. Applied on update only.",
			},
			"uses_inertia_ssr": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether the instance uses Inertia SSR. Applied on update only.",
			},
			"hibernation_timeout": schema.Int64Attribute{
				Optional:    true,
				Description: "Hibernation timeout in seconds. Applied on update only.",
			},
			"paused": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the instance is paused.",
			},
			"is_default": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this is the default instance.",
			},
			"queue_status": schema.StringAttribute{
				Computed:    true,
				Description: "Raw queue status JSON (managed_queue).",
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *InstanceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *InstanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan InstanceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateInstanceRequest{
		Name:        plan.Name.ValueString(),
		Type:        plan.Type.ValueString(),
		Size:        plan.Size.ValueString(),
		ScalingType: plan.ScalingType.ValueString(),
		MinReplicas: int(plan.MinReplicas.ValueInt64()),
		MaxReplicas: int(plan.MaxReplicas.ValueInt64()),
	}
	if !plan.UsesScheduler.IsNull() {
		v := plan.UsesScheduler.ValueBool()
		createReq.UsesScheduler = &v
	}
	if !plan.ScalingCPUThresholdPercentage.IsNull() {
		v := int(plan.ScalingCPUThresholdPercentage.ValueInt64())
		createReq.ScalingCPUThresholdPercentage = &v
	}
	if !plan.ScalingMemoryThresholdPercentage.IsNull() {
		v := int(plan.ScalingMemoryThresholdPercentage.ValueInt64())
		createReq.ScalingMemoryThresholdPercentage = &v
	}
	if !plan.SleepWithApp.IsNull() {
		v := plan.SleepWithApp.ValueBool()
		createReq.SleepWithApp = &v
	}
	if !plan.VisibilityTimeout.IsNull() {
		v := int(plan.VisibilityTimeout.ValueInt64())
		createReq.VisibilityTimeout = &v
	}
	if !plan.PollingInterval.IsNull() {
		v := int(plan.PollingInterval.ValueInt64())
		createReq.PollingInterval = &v
	}
	if !plan.ShutdownTimeout.IsNull() {
		v := int(plan.ShutdownTimeout.ValueInt64())
		createReq.ShutdownTimeout = &v
	}

	inst, err := r.client.CreateInstance(ctx, plan.EnvironmentID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating instance", err.Error())
		return
	}

	mapInstanceToState(inst, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *InstanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state InstanceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	inst, err := r.client.GetInstance(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading instance", err.Error())
		return
	}

	mapInstanceToState(inst, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *InstanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan InstanceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state InstanceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := client.UpdateInstanceRequest{}
	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		updateReq.Name = &v
	}
	if !plan.Size.Equal(state.Size) {
		v := plan.Size.ValueString()
		updateReq.Size = &v
	}
	if !plan.ScalingType.Equal(state.ScalingType) {
		v := plan.ScalingType.ValueString()
		updateReq.ScalingType = &v
	}
	if !plan.MinReplicas.Equal(state.MinReplicas) {
		v := int(plan.MinReplicas.ValueInt64())
		updateReq.MinReplicas = &v
	}
	if !plan.MaxReplicas.Equal(state.MaxReplicas) {
		v := int(plan.MaxReplicas.ValueInt64())
		updateReq.MaxReplicas = &v
	}
	if !plan.UsesScheduler.Equal(state.UsesScheduler) {
		v := plan.UsesScheduler.ValueBool()
		updateReq.UsesScheduler = &v
	}
	if !plan.SleepWithApp.Equal(state.SleepWithApp) {
		v := plan.SleepWithApp.ValueBool()
		updateReq.SleepWithApp = &v
	}
	if !plan.VisibilityTimeout.Equal(state.VisibilityTimeout) {
		v := int(plan.VisibilityTimeout.ValueInt64())
		updateReq.VisibilityTimeout = &v
	}
	if !plan.PollingInterval.Equal(state.PollingInterval) {
		v := int(plan.PollingInterval.ValueInt64())
		updateReq.PollingInterval = &v
	}
	if !plan.ShutdownTimeout.Equal(state.ShutdownTimeout) {
		v := int(plan.ShutdownTimeout.ValueInt64())
		updateReq.ShutdownTimeout = &v
	}
	// Update-only behavioral fields: not returned by the API, so send them
	// whenever they are set in the plan.
	if !plan.UsesOctane.IsNull() {
		v := plan.UsesOctane.ValueBool()
		updateReq.UsesOctane = &v
	}
	if !plan.UsesInertiaSSR.IsNull() {
		v := plan.UsesInertiaSSR.ValueBool()
		updateReq.UsesInertiaSSR = &v
	}
	if !plan.HibernationTimeout.IsNull() {
		v := int(plan.HibernationTimeout.ValueInt64())
		updateReq.HibernationTimeout = &v
	}

	inst, err := r.client.UpdateInstance(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating instance", err.Error())
		return
	}

	plan.EnvironmentID = state.EnvironmentID
	mapInstanceToState(inst, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *InstanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state InstanceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteInstance(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting instance", err.Error())
	}
}

func (r *InstanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mapInstanceToState(inst *client.InstanceData, state *InstanceResourceModel) {
	state.ID = types.StringValue(inst.ID)
	state.Name = types.StringValue(inst.Attributes.Name)
	state.Type = types.StringValue(inst.Attributes.InstanceType)
	state.Size = types.StringValue(inst.Attributes.Size)
	state.ScalingType = types.StringValue(inst.Attributes.ScalingType)
	state.MinReplicas = types.Int64Value(int64(inst.Attributes.MinReplicas))
	state.MaxReplicas = types.Int64Value(int64(inst.Attributes.MaxReplicas))
	state.UsesScheduler = types.BoolValue(inst.Attributes.UsesScheduler)
	state.CreatedAt = types.StringPointerValue(inst.Attributes.CreatedAt)
	if inst.Attributes.ScalingCPUThresholdPercentage != nil {
		state.ScalingCPUThresholdPercentage = types.Int64Value(int64(*inst.Attributes.ScalingCPUThresholdPercentage))
	}
	if inst.Attributes.ScalingMemoryThresholdPercentage != nil {
		state.ScalingMemoryThresholdPercentage = types.Int64Value(int64(*inst.Attributes.ScalingMemoryThresholdPercentage))
	}

	// Optional+Computed managed-queue fields: null-aware mapping from response.
	if inst.Attributes.SleepWithApp != nil {
		state.SleepWithApp = types.BoolValue(*inst.Attributes.SleepWithApp)
	} else {
		state.SleepWithApp = types.BoolNull()
	}
	if inst.Attributes.VisibilityTimeout != nil {
		state.VisibilityTimeout = types.Int64Value(int64(*inst.Attributes.VisibilityTimeout))
	} else {
		state.VisibilityTimeout = types.Int64Null()
	}
	if inst.Attributes.PollingInterval != nil {
		state.PollingInterval = types.Int64Value(int64(*inst.Attributes.PollingInterval))
	} else {
		state.PollingInterval = types.Int64Null()
	}
	if inst.Attributes.ShutdownTimeout != nil {
		state.ShutdownTimeout = types.Int64Value(int64(*inst.Attributes.ShutdownTimeout))
	} else {
		state.ShutdownTimeout = types.Int64Null()
	}

	// Computed read-only fields.
	if inst.Attributes.Paused != nil {
		state.Paused = types.BoolValue(*inst.Attributes.Paused)
	} else {
		state.Paused = types.BoolNull()
	}
	if inst.Attributes.IsDefault != nil {
		state.IsDefault = types.BoolValue(*inst.Attributes.IsDefault)
	} else {
		state.IsDefault = types.BoolNull()
	}
	if len(inst.Attributes.QueueStatus) > 0 && string(inst.Attributes.QueueStatus) != "null" {
		state.QueueStatus = types.StringValue(string(inst.Attributes.QueueStatus))
	} else {
		state.QueueStatus = types.StringNull()
	}
}
