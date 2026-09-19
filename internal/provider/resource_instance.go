package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                   = &InstanceResource{}
	_ resource.ResourceWithImportState    = &InstanceResource{}
	_ resource.ResourceWithValidateConfig = &InstanceResource{}
	_ resource.ResourceWithModifyPlan     = &InstanceResource{}
)

type InstanceResource struct {
	resourceWithClient
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
				Optional: true,
				Computed: true,
				Description: "Minimum number of replicas. Only applicable to the \"custom\" " +
					"scaling type; the API rejects it for \"auto\", and it does not apply to " +
					"managed queues, which always scale to zero when idle.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"max_replicas": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Maximum number of replicas. Only applicable to the \"custom\" " +
					"scaling type; the API rejects it for \"auto\".",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
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
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"visibility_timeout": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Queue visibility timeout in seconds (managed_queue).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"polling_interval": schema.Int64Attribute{
				Computed: true,
				Description: "Queue polling interval in seconds (managed_queue). Read-only: " +
					"the API reports this value but accepts it in neither the create nor the " +
					"update request, so it is managed by the platform.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"shutdown_timeout": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Queue shutdown timeout in seconds (managed_queue).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
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
	}
	// min_replicas/max_replicas are rejected outright for the "auto" scaling
	// type, so they are sent only when the user actually set them.
	if !plan.MinReplicas.IsNull() && !plan.MinReplicas.IsUnknown() {
		v := int(plan.MinReplicas.ValueInt64())
		createReq.MinReplicas = &v
	}
	if !plan.MaxReplicas.IsNull() && !plan.MaxReplicas.IsUnknown() {
		v := int(plan.MaxReplicas.ValueInt64())
		createReq.MaxReplicas = &v
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
	// These three are Optional+Computed with no default, so a value the user
	// did not set arrives UNKNOWN, not null. IsNull() alone is false for an
	// unknown value and ValueBool()/ValueInt64() then yield the zero value, so
	// the request carried sleep_with_app=false, visibility_timeout=0 and
	// shutdown_timeout=0 on every create that left them out -- overwriting
	// whatever the platform would otherwise have chosen.
	if !plan.SleepWithApp.IsNull() && !plan.SleepWithApp.IsUnknown() {
		v := plan.SleepWithApp.ValueBool()
		createReq.SleepWithApp = &v
	}
	if !plan.VisibilityTimeout.IsNull() && !plan.VisibilityTimeout.IsUnknown() {
		v := int(plan.VisibilityTimeout.ValueInt64())
		createReq.VisibilityTimeout = &v
	}
	if !plan.ShutdownTimeout.IsNull() && !plan.ShutdownTimeout.IsUnknown() {
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
	// A null planned replica count means "not applicable" -- the instance is
	// scaling automatically, and the API rejects the field outright. Sending
	// ValueInt64() of a null would post 0, which is neither what the plan says
	// nor a value the API accepts.
	if !plan.MinReplicas.Equal(state.MinReplicas) && !plan.MinReplicas.IsNull() {
		v := int(plan.MinReplicas.ValueInt64())
		updateReq.MinReplicas = &v
	}
	if !plan.MaxReplicas.Equal(state.MaxReplicas) && !plan.MaxReplicas.IsNull() {
		v := int(plan.MaxReplicas.ValueInt64())
		updateReq.MaxReplicas = &v
	}
	if !plan.UsesScheduler.Equal(state.UsesScheduler) {
		v := plan.UsesScheduler.ValueBool()
		updateReq.UsesScheduler = &v
	}
	if !plan.SleepWithApp.IsUnknown() && !plan.SleepWithApp.Equal(state.SleepWithApp) {
		v := plan.SleepWithApp.ValueBool()
		updateReq.SleepWithApp = &v
	}
	if !plan.VisibilityTimeout.IsUnknown() && !plan.VisibilityTimeout.Equal(state.VisibilityTimeout) {
		v := int(plan.VisibilityTimeout.ValueInt64())
		updateReq.VisibilityTimeout = &v
	}
	if !plan.ShutdownTimeout.IsUnknown() && !plan.ShutdownTimeout.Equal(state.ShutdownTimeout) {
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
	// environment_id is not part of the import id and forces replacement, so
	// without adopting it from the relationship an imported instance would be
	// proposed for destruction on the very next plan.
	if envID := inst.Relationships.Environment.RelatedID(); envID != "" {
		state.EnvironmentID = types.StringValue(envID)
	}
	state.ID = types.StringValue(inst.ID)
	state.Name = types.StringValue(inst.Attributes.Name)
	state.Type = types.StringValue(inst.Attributes.InstanceType)
	state.Size = types.StringValue(inst.Attributes.Size)
	state.ScalingType = types.StringValue(inst.Attributes.ScalingType)
	state.MinReplicas = types.Int64PointerValue(inst.Attributes.MinReplicas)
	state.MaxReplicas = types.Int64PointerValue(inst.Attributes.MaxReplicas)
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
	if inst.Attributes.QueueStatus.IsZero() {
		state.QueueStatus = types.StringNull()
	} else {
		state.QueueStatus = types.StringValue(inst.Attributes.QueueStatus.Value)
	}
}

// ValidateConfig moves the scaling_type/replica incompatibility from a runtime
// 422 to a plan-time error. The API documents min_replicas and max_replicas as
// "only applicable to the custom scaling type, and rejected when used with
// auto", so a config that sets them alongside auto can never apply.
func (r *InstanceResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config InstanceResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// scaling_type may be unknown when it comes from another resource.
	if config.ScalingType.IsNull() || config.ScalingType.IsUnknown() {
		return
	}

	isSet := func(v types.Int64) bool { return !v.IsNull() && !v.IsUnknown() }

	if config.ScalingType.ValueString() == "auto" {
		if isSet(config.MinReplicas) {
			resp.Diagnostics.AddAttributeError(
				path.Root("min_replicas"),
				"min_replicas cannot be used with automatic scaling",
				"The Laravel Cloud API rejects min_replicas when scaling_type is \"auto\". "+
					"Remove min_replicas, or set scaling_type to \"custom\".",
			)
		}
		if isSet(config.MaxReplicas) {
			resp.Diagnostics.AddAttributeError(
				path.Root("max_replicas"),
				"max_replicas cannot be used with automatic scaling",
				"The Laravel Cloud API rejects max_replicas when scaling_type is \"auto\". "+
					"Remove max_replicas, or set scaling_type to \"custom\".",
			)
		}
		return
	}

	// Outside "auto", min_replicas is a required field of the create request.
	if config.ScalingType.ValueString() == "custom" && !isSet(config.MinReplicas) {
		resp.Diagnostics.AddAttributeError(
			path.Root("min_replicas"),
			"min_replicas is required for custom scaling",
			"scaling_type is \"custom\", which requires min_replicas to be set.",
		)
	}
}

// ModifyPlan clears the replica counts when the instance scales automatically.
//
// min_replicas and max_replicas are Optional+Computed with UseStateForUnknown,
// so removing them from config does not plan a change -- the prior values are
// carried forward. Switching an existing instance from "custom" to "auto" would
// therefore leave the old counts in state and on the server, which is exactly
// the combination ValidateConfig declares impossible and the API rejects, with
// no way out: removing them from config does nothing and setting them fails
// validation.
//
// Planning them as null instead makes the transition mean what it says.
func (r *InstanceResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Nothing to do when the resource is being destroyed.
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan InstanceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.ScalingType.IsUnknown() || plan.ScalingType.ValueString() != "auto" {
		return
	}

	changed := false
	if !plan.MinReplicas.IsNull() {
		plan.MinReplicas = types.Int64Null()
		changed = true
	}
	if !plan.MaxReplicas.IsNull() {
		plan.MaxReplicas = types.Int64Null()
		changed = true
	}
	if changed {
		resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
	}
}
