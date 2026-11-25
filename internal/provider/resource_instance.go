package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	Probes                           types.Object `tfsdk:"probes"`
	QueueStatus                      types.String `tfsdk:"queue_status"`
	Fifo                             types.Bool   `tfsdk:"fifo"`
	QueueType                        types.String `tfsdk:"queue_type"`
	RetireAt                         types.String `tfsdk:"retire_at"`
	RetiredAt                        types.String `tfsdk:"retired_at"`
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
				Description: "Instance name (3-40 characters). Changing a managed queue's name replaces the queue.",
				// The API renames a managed queue's instance but not its
				// SQS queue, so the app and worker end up on a queue that doesn't
				// exist. A new queue under the new name is the only safe rename.
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIf(
						func(ctx context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
							var instanceType types.String
							resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("type"), &instanceType)...)
							resp.RequiresReplace = instanceType.ValueString() == "managed_queue"
						},
						"Changing a managed queue's name replaces the queue.",
						"Changing a managed queue's name replaces the queue.",
					),
				},
			},
			"type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("service"),
				Description: "Instance type (service, managed_queue or app). Defaults to service when not set. Laravel Cloud " +
					"creates an environment's app instance with the environment, so app takes over that instance instead " +
					"of creating one, and destroy leaves it to the environment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"size": schema.StringAttribute{
				Required:    true,
				Description: "Instance size (such as flex-512mb).",
			},
			"scaling_type": schema.StringAttribute{
				Required:    true,
				Description: "Scaling type (none, custom, auto).",
			},
			"min_replicas": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Minimum number of replicas. Set it only for the \"custom\" scaling type: " +
					"for \"auto\" and \"none\" the platform sets it to 1. Required for custom scaling " +
					"except on managed queues, where it is the minimum number of workers: optional " +
					"on current-generation (mq.*) sizes, where 0 lets the queue scale to zero when " +
					"idle, and rejected on previous-generation sizes, which always scale to zero.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"max_replicas": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Maximum number of replicas. Set it only for the \"custom\" scaling type: " +
					"for \"auto\" the platform sets it to the plan's replica limit, and for \"none\" to 1.",
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
				Description: "CPU usage, in percent (50-95), above which the instance scales out. Leave unset for the platform default. Not allowed with scaling_type \"none\".",
			},
			"scaling_memory_threshold_percentage": schema.Int64Attribute{
				Optional:    true,
				Description: "Memory usage, in percent (50-95), above which the instance scales out. Leave unset for the platform default. Not allowed with scaling_type \"none\".",
			},
			"sleep_with_app": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the instance's replicas scale to zero along with the app, waking when it does. Only for service instances.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"visibility_timeout": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Queue visibility timeout in seconds (managed_queue). Laravel Cloud applies a custom value " +
					"only for organizations with the advanced managed queues entitlement, and otherwise manages it " +
					"itself: setting it then fails the apply. Leave it unset to let the platform choose.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"polling_interval": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Queue polling interval in seconds (1-60). Only for previous-generation " +
					"managed queues (sizes such as mq-pro-256mb), which require it; current-generation " +
					"(mq.*) sizes have no polling interval.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"shutdown_timeout": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Queue shutdown timeout in seconds (managed_queue). Laravel Cloud applies a custom value " +
					"only for organizations with the advanced managed queues entitlement, and otherwise manages it " +
					"itself: setting it then fails the apply. Leave it unset to let the platform choose.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"uses_octane": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Whether the app is served by Laravel Octane. Only for app instances, which Laravel Cloud " +
					"creates with the environment: set type = \"app\" to manage it. The same setting as the " +
					"environment's uses_octane: set it in one place.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"uses_inertia_ssr": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Whether the app runs an Inertia SSR server. Only for app instances, which Laravel Cloud " +
					"creates with the environment: set type = \"app\" to manage it. Changing it also changes the " +
					"environment's build command.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"hibernation_timeout": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Minutes of inactivity (1-60) before the app hibernates (scales to zero). Set 0 to turn " +
					"hibernation off. Only for app instances, which Laravel Cloud creates with the environment, so " +
					"set type = \"app\" to manage it. Use it instead of the environment's deprecated sleep_timeout, " +
					"which writes the same setting but cannot turn hibernation off.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"paused": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Whether the managed queue is paused, which stops it dispatching jobs to its workers. " +
					"Set true to pause it and false to resume it. Only for managed_queue instances; null for others.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_default": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Whether this managed queue is the environment's default, the one a job goes to when it " +
					"names no queue. Set true to make it the default; only managed_queue instances can be. It cannot " +
					"be set false: the default moves when another queue is made the default.",
			},
			"probes": instanceProbesSchema(),
			"queue_status": schema.StringAttribute{
				Computed:    true,
				Description: "Raw queue status JSON (managed_queue).",
			},
			"fifo": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				Description: "Whether the managed queue is a FIFO queue, which delivers jobs exactly once and in order. " +
					"Only for managed queues, on organizations with current-generation queues. Laravel Cloud adds a \".fifo\" suffix to the " +
					"queue's name, which can then have at most 34 characters; name is set without it. Changing it " +
					"replaces the queue.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"queue_type": schema.StringAttribute{
				Computed:    true,
				Description: "The managed queue's type: standard or fifo. Null for other instance types.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"retire_at": schema.StringAttribute{
				Computed:    true,
				Description: "When the instance's size is scheduled to be retired because it is being discontinued.",
			},
			"retired_at": schema.StringAttribute{
				Computed: true,
				Description: "When the instance was retired because its size is no longer supported. A retired queue " +
					"stops processing jobs until its size is upgraded.",
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
		resp.Diagnostics.AddError("Unexpected resource configure type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
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
	if plan.Type.ValueString() == "app" {
		r.adoptAppInstance(ctx, plan, resp)
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
	if !plan.PollingInterval.IsNull() && !plan.PollingInterval.IsUnknown() {
		v := int(plan.PollingInterval.ValueInt64())
		createReq.PollingInterval = &v
	}
	// fifo is declined for anything but a managed queue.
	if plan.Type.ValueString() == "managed_queue" && plan.Fifo.ValueBool() {
		v := true
		createReq.Fifo = &v
	}
	// A managed queue runs exactly one worker with one process on the cloud
	// connection, as the CLI creates it. Without it the API answers 422.
	if plan.Type.ValueString() == "managed_queue" {
		createReq.BackgroundProcesses = []client.CreateBackgroundProcessRequest{{
			Type:      "worker",
			Processes: 1,
			Config:    map[string]any{"connection": "cloud", "queue": plan.Name.ValueString()},
		}}
	}

	inst, err := r.client.CreateInstance(ctx, plan.EnvironmentID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating instance", err.Error())
		return
	}

	// probes are accepted only by the update route, so they are applied here.
	//
	// The instance exists by this point, so a failure must not abort Create:
	// returning early would skip the state write below and strand a live,
	// billing instance that Terraform cannot destroy. The next apply retries.
	if updateReq, ok := instanceCreateFollowUp(plan); ok {
		if updated, err := r.client.UpdateInstance(ctx, inst.ID, updateReq); err != nil {
			resp.Diagnostics.AddWarning(
				"Could not apply all settings after creating the instance",
				"The instance was created, but applying probes failed: "+err.Error()+
					". The configured values are kept in state; run 'terraform apply' again to retry.",
			)
		} else {
			inst = updated
		}
	}

	configured := plan
	configuredProbes := plan.Probes
	configuredDefault := plan.IsDefault
	configuredPaused := plan.Paused
	mapInstanceToState(inst, &plan)
	// The instance exists, so a queue timeout the API ignored is a warning,
	// and the configured value stays in state, which Terraform requires. The
	// next refresh records what the API kept, so the next plan offers the
	// change again and Update fails, saying why.
	for _, m := range queueTimeoutsNotApplied(configured, plan) {
		resp.Diagnostics.AddAttributeWarning(path.Root(m.attr), m.attr+" was not applied",
			m.detail()+" The configured value is kept in state for now, so the next plan shows the difference again.")
		m.restore(&plan, configured)
	}
	// The API ignores probes it cannot apply rather than rejecting them. The
	// instance exists, so keep the configured value and let the next plan
	// retry, where Update reports why they were not applied.
	if !configuredProbes.IsNull() && plan.Probes.IsNull() {
		resp.Diagnostics.AddWarning("Health checks were not applied", probesNotAppliedDetail)
		plan.Probes = configuredProbes
	}
	// A new queue is still creating, and the API makes it the default or
	// pauses it only once it is available, so these wait for that. If one
	// still fails, the configured value stays in state, which Terraform
	// requires: the next refresh records what the queue really is, so the next
	// plan shows the change again and Update retries it.
	if isSet(configuredDefault) && configuredDefault.ValueBool() && !plan.IsDefault.ValueBool() {
		if updated, err := queueActionWhenAvailable(ctx, r.client.SetDefaultInstance, inst.ID); err != nil {
			resp.Diagnostics.AddWarning(
				"Could not make the queue the default after creating it",
				"The instance was created, but making it the environment's default queue failed: "+err.Error()+
					". The configured value is kept in state, so the next plan shows the change again and "+
					"'terraform apply' retries it.",
			)
			plan.IsDefault = configuredDefault
		} else {
			plan.IsDefault = types.BoolPointerValue(updated.Attributes.IsDefault)
		}
	}
	if isSet(configuredPaused) && configuredPaused.ValueBool() && !plan.Paused.ValueBool() {
		if updated, err := queueActionWhenAvailable(ctx, r.client.PauseInstance, inst.ID); err != nil {
			resp.Diagnostics.AddWarning(
				"Could not pause the queue after creating it",
				"The instance was created, but pausing it failed: "+err.Error()+". The configured value is kept "+
					"in state, so the next plan shows the change again and 'terraform apply' retries it.",
			)
			plan.Paused = configuredPaused
		} else {
			plan.Paused = types.BoolPointerValue(updated.Attributes.Paused)
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// adoptAppInstance takes over the app instance Laravel Cloud creates with
// every environment, which the API cannot create, and applies the configured
// settings to it. Nothing is created, so a failure leaves nothing behind and
// the next apply tries again.
func (r *InstanceResource) adoptAppInstance(ctx context.Context, plan InstanceResourceModel, resp *resource.CreateResponse) {
	envID := plan.EnvironmentID.ValueString()
	instances, err := r.client.ListInstances(ctx, envID)
	if err != nil {
		resp.Diagnostics.AddError("Error finding the environment's app instance", err.Error())
		return
	}
	var inst *client.InstanceData
	for i := range instances {
		if instances[i].Attributes.InstanceType == "app" {
			inst = &instances[i]
			break
		}
	}
	if inst == nil {
		resp.Diagnostics.AddAttributeError(path.Root("type"), "Environment has no app instance",
			fmt.Sprintf("An app instance cannot be created: Laravel Cloud creates one with each environment, and "+
				"environment %s has none.", envID))
		return
	}

	var current InstanceResourceModel
	mapInstanceToState(inst, &current)
	if updateReq := instanceUpdateRequest(plan, current); updateReq != (client.UpdateInstanceRequest{}) {
		if inst, err = r.client.UpdateInstance(ctx, inst.ID, updateReq); err != nil {
			resp.Diagnostics.AddError("Error updating the environment's app instance", err.Error())
			return
		}
	}
	configuredProbes := plan.Probes
	if probes, _ := probesFromObject(configuredProbes); probes != nil && inst.Attributes.Probes == nil {
		resp.Diagnostics.AddAttributeError(path.Root("probes"), "Health checks were not applied", probesNotAppliedDetail)
		return
	}
	mapInstanceToState(inst, &plan)
	plan.EnvironmentID = types.StringValue(envID)
	plan.Probes = configuredProbes
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// The API refuses to make a managed queue the default, pause it or resume it
// while its SQS queue is creating or updating ("This queue is not available
// yet"), as a queue is right after it is created or resized. These bound how
// long that is retried. The interval is a variable so the plan tests need not
// wait.
const queueAvailableAttempts = 60

var queueAvailableInterval = 5 * time.Second

// queueActionWhenAvailable runs one of a managed queue's action routes,
// retrying while the queue is not available yet.
func queueActionWhenAvailable(ctx context.Context, action func(context.Context, string) (*client.InstanceData, error), id string) (*client.InstanceData, error) {
	var inst *client.InstanceData
	err := client.RetryOnConflict(ctx, queueAvailableAttempts, queueAvailableInterval, func() error {
		var err error
		inst, err = action(ctx, id)
		return err
	}, client.IsBusy)
	return inst, err
}

// queueTimeoutNotApplied is a queue timeout the configuration sets that the
// API stored differently.
type queueTimeoutNotApplied struct {
	attr       string
	configured int64
	stored     types.Int64
}

// queueTimeoutsNotApplied reports the configured visibility_timeout and
// shutdown_timeout that the API did not store. Laravel Cloud accepts custom
// values only from organizations with the advanced managed queues
// entitlement; for any other it drops them without an error and keeps its
// own. The public API does not expose the entitlement, so the provider can
// only tell after the fact, by comparing what was stored with what was set.
func queueTimeoutsNotApplied(configured, stored InstanceResourceModel) []queueTimeoutNotApplied {
	var out []queueTimeoutNotApplied
	for _, a := range []struct {
		attr               string
		configured, stored types.Int64
	}{
		{"visibility_timeout", configured.VisibilityTimeout, stored.VisibilityTimeout},
		{"shutdown_timeout", configured.ShutdownTimeout, stored.ShutdownTimeout},
	} {
		if isSet(a.configured) && !a.configured.Equal(a.stored) {
			out = append(out, queueTimeoutNotApplied{attr: a.attr, configured: a.configured.ValueInt64(), stored: a.stored})
		}
	}
	return out
}

func (m queueTimeoutNotApplied) detail() string {
	stored := "no value"
	if isSet(m.stored) {
		stored = strconv.FormatInt(m.stored.ValueInt64(), 10)
	}
	return fmt.Sprintf("Laravel Cloud kept %s at %s instead of %d. It accepts a custom %s only from organizations "+
		"with the advanced managed queues entitlement, and otherwise ignores it and manages the value itself. "+
		"Remove %s from the configuration to let the platform choose.", m.attr, stored, m.configured, m.attr, m.attr)
}

// restore puts the configured value back on state.
func (m queueTimeoutNotApplied) restore(state *InstanceResourceModel, configured InstanceResourceModel) {
	switch m.attr {
	case "visibility_timeout":
		state.VisibilityTimeout = configured.VisibilityTimeout
	case "shutdown_timeout":
		state.ShutdownTimeout = configured.ShutdownTimeout
	}
}

// probesNotAppliedDetail explains a probes value the API accepted but ignored.
const probesNotAppliedDetail = "The API did not apply probes. Health checks are available only on app instances, " +
	"and only on plans that include them."

// instanceCreateFollowUp collects the update-only attributes the create route
// ignores, reporting whether any of them were configured. uses_octane,
// uses_inertia_ssr and hibernation_timeout are not among them: they apply only
// to app instances, which the API cannot create.
func instanceCreateFollowUp(plan InstanceResourceModel) (client.UpdateInstanceRequest, bool) {
	var req client.UpdateInstanceRequest
	set := false
	if probes, _ := probesFromObject(plan.Probes); probes != nil {
		req.Probes = probesRequest(probes)
		set = true
	}
	return req, set
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

	updateReq := instanceUpdateRequest(plan, state)
	planProbes, _ := probesFromObject(plan.Probes)

	inst, err := r.client.UpdateInstance(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating instance", err.Error())
		return
	}
	if planProbes != nil && inst.Attributes.Probes == nil {
		resp.Diagnostics.AddAttributeError(path.Root("probes"), "Health checks were not applied", probesNotAppliedDetail)
		return
	}
	// Prior state is kept, so the next refresh records what the API stored.
	var stored InstanceResourceModel
	mapInstanceToState(inst, &stored)
	if notApplied := queueTimeoutsNotApplied(plan, stored); len(notApplied) > 0 {
		for _, m := range notApplied {
			resp.Diagnostics.AddAttributeError(path.Root(m.attr), m.attr+" was not applied", m.detail())
		}
		return
	}

	// Making a queue the default is its own call, and the API refuses it for
	// the queue that already is.
	configuredDefault := plan.IsDefault
	if isSet(configuredDefault) && configuredDefault.ValueBool() && !state.IsDefault.ValueBool() {
		if inst, err = queueActionWhenAvailable(ctx, r.client.SetDefaultInstance, state.ID.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error making the queue the default", err.Error())
			return
		}
	}
	if isSet(plan.Paused) && !plan.Paused.Equal(state.Paused) {
		action := r.client.ResumeInstance
		if plan.Paused.ValueBool() {
			action = r.client.PauseInstance
		}
		if inst, err = queueActionWhenAvailable(ctx, action, state.ID.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error pausing or resuming the queue", err.Error())
			return
		}
	}

	plan.EnvironmentID = state.EnvironmentID
	configuredProbes := plan.Probes
	mapInstanceToState(inst, &plan)
	// The action routes answer without probes, which only the update
	// response reports.
	plan.Probes = configuredProbes
	if isSet(configuredDefault) {
		plan.IsDefault = configuredDefault
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// instanceUpdateRequest builds the PATCH that moves an instance from state to
// plan, sending only what changed.
func instanceUpdateRequest(plan, state InstanceResourceModel) client.UpdateInstanceRequest {
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
	// An unknown replica count is one the platform picks, and the API rejects
	// either count outright for "auto".
	autoScaling := plan.ScalingType.ValueString() == "auto"
	if !autoScaling && isSet(plan.MinReplicas) && !plan.MinReplicas.Equal(state.MinReplicas) {
		v := int(plan.MinReplicas.ValueInt64())
		updateReq.MinReplicas = &v
	}
	if !autoScaling && isSet(plan.MaxReplicas) && !plan.MaxReplicas.Equal(state.MaxReplicas) {
		v := int(plan.MaxReplicas.ValueInt64())
		updateReq.MaxReplicas = &v
	}
	// A removed threshold is sent as null, which returns it to the platform
	// default.
	if !plan.ScalingCPUThresholdPercentage.Equal(state.ScalingCPUThresholdPercentage) {
		v := plan.ScalingCPUThresholdPercentage
		updateReq.ScalingCPUThresholdPercentage = &client.NullableInt{Value: int(v.ValueInt64()), Null: v.IsNull()}
	}
	if !plan.ScalingMemoryThresholdPercentage.Equal(state.ScalingMemoryThresholdPercentage) {
		v := plan.ScalingMemoryThresholdPercentage
		updateReq.ScalingMemoryThresholdPercentage = &client.NullableInt{Value: int(v.ValueInt64()), Null: v.IsNull()}
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
	if !plan.PollingInterval.IsUnknown() && !plan.PollingInterval.IsNull() && !plan.PollingInterval.Equal(state.PollingInterval) {
		v := int(plan.PollingInterval.ValueInt64())
		updateReq.PollingInterval = &v
	}
	if isSet(plan.UsesOctane) && !plan.UsesOctane.Equal(state.UsesOctane) {
		v := plan.UsesOctane.ValueBool()
		updateReq.UsesOctane = &v
	}
	if isSet(plan.UsesInertiaSSR) && !plan.UsesInertiaSSR.Equal(state.UsesInertiaSSR) {
		v := plan.UsesInertiaSSR.ValueBool()
		updateReq.UsesInertiaSSR = &v
	}
	// 0 turns hibernation off, which the API spells null.
	if isSet(plan.HibernationTimeout) && !plan.HibernationTimeout.Equal(state.HibernationTimeout) {
		v := plan.HibernationTimeout.ValueInt64()
		updateReq.HibernationTimeout = &client.NullableInt{Value: int(v), Null: v == 0}
	}
	planProbes, _ := probesFromObject(plan.Probes)
	stateProbes, _ := probesFromObject(state.Probes)
	if !probesEqual(planProbes, stateProbes) {
		updateReq.Probes = probesRequest(planProbes)
	}
	return updateReq
}

func (r *InstanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state InstanceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API refuses to delete an environment's app instance, which this
	// resource took over or imported. Deleting the environment deletes it, so
	// it is only dropped from state here.
	if state.Type.ValueString() == "app" {
		resp.Diagnostics.AddWarning(
			"App instance not deleted",
			fmt.Sprintf("Instance %q is its environment's app instance, which Laravel Cloud cannot delete on its own. "+
				"It was removed from Terraform state and is deleted along with the environment.", state.Name.ValueString()),
		)
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
	// The API stores a FIFO queue's name with the ".fifo" suffix it adds on
	// create, which the configuration leaves out.
	state.QueueType = types.StringPointerValue(inst.Attributes.QueueType)
	state.Fifo = types.BoolValue(state.QueueType.ValueString() == "fifo")
	name := inst.Attributes.Name
	if state.Fifo.ValueBool() {
		name = strings.TrimSuffix(name, fifoQueueSuffix)
	}
	state.Name = types.StringValue(name)
	state.RetireAt = types.StringPointerValue(inst.Attributes.RetireAt)
	state.RetiredAt = types.StringPointerValue(inst.Attributes.RetiredAt)
	state.Type = types.StringValue(inst.Attributes.InstanceType)
	state.Size = types.StringValue(inst.Attributes.Size)
	state.ScalingType = types.StringValue(inst.Attributes.ScalingType)
	state.MinReplicas = types.Int64PointerValue(inst.Attributes.MinReplicas)
	state.MaxReplicas = types.Int64PointerValue(inst.Attributes.MaxReplicas)
	state.UsesScheduler = types.BoolValue(inst.Attributes.UsesScheduler)
	state.CreatedAt = types.StringPointerValue(inst.Attributes.CreatedAt)
	// null is the platform default, and what the API reports once scaling
	// stops, so it must replace whatever was there before.
	state.ScalingCPUThresholdPercentage = types.Int64Null()
	if inst.Attributes.ScalingCPUThresholdPercentage != nil {
		state.ScalingCPUThresholdPercentage = types.Int64Value(int64(*inst.Attributes.ScalingCPUThresholdPercentage))
	}
	state.ScalingMemoryThresholdPercentage = types.Int64Null()
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

	// App-instance settings; null for every other type.
	state.UsesOctane = types.BoolPointerValue(inst.Attributes.UsesOctane)
	state.UsesInertiaSSR = types.BoolPointerValue(inst.Attributes.UsesInertiaSSR)
	state.HibernationTimeout = hibernationTimeoutFromAPI(inst)

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

	// probes are managed only once configured: health checks the platform set
	// up on its own are not adopted, so leaving the attribute unset never
	// plans to remove them.
	if !state.Probes.IsNull() {
		state.Probes = probesToObject(probesFromAPI(inst.Attributes.Probes))
	}
}

// hibernationTimeoutFromAPI maps the API's timeout onto the attribute. For an
// app instance null means hibernation is off, which the configuration spells
// 0, so that is what it reads as; other instance types have no timeout.
func hibernationTimeoutFromAPI(inst *client.InstanceData) types.Int64 {
	if inst.Attributes.HibernationTimeout != nil {
		return types.Int64Value(*inst.Attributes.HibernationTimeout)
	}
	if inst.Attributes.InstanceType == "app" {
		return types.Int64Value(0)
	}
	return types.Int64Null()
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

	if isSet(config.IsDefault) && !config.IsDefault.ValueBool() {
		resp.Diagnostics.AddAttributeError(
			path.Root("is_default"),
			"is_default cannot be set to false",
			"A queue stops being the default only when another queue is made the default. "+
				"Set is_default = true on that queue instead, and remove it here.",
		)
	}
	// An unset type is a service instance, which can be neither.
	if isSet(config.IsDefault) && !config.Type.IsUnknown() && configuredInstanceType(config.Type) != "managed_queue" {
		resp.Diagnostics.AddAttributeError(path.Root("is_default"), "is_default is only for managed queues",
			fmt.Sprintf("Only a managed_queue instance can be the default queue; this one is %q.", configuredInstanceType(config.Type)))
	}
	if isSet(config.Paused) && !config.Type.IsUnknown() && configuredInstanceType(config.Type) != "managed_queue" {
		resp.Diagnostics.AddAttributeError(path.Root("paused"), "paused is only for managed queues",
			fmt.Sprintf("Only a managed_queue instance can be paused; this one is %q.", configuredInstanceType(config.Type)))
	}
	validateProbesConfig(config, &resp.Diagnostics)
	validateManagedQueueConfig(config, &resp.Diagnostics)

	validateAppInstanceConfig(config, &resp.Diagnostics)

	if config.ScalingType.ValueString() == "none" {
		for _, a := range []struct {
			name string
			v    types.Int64
		}{
			{"scaling_cpu_threshold_percentage", config.ScalingCPUThresholdPercentage},
			{"scaling_memory_threshold_percentage", config.ScalingMemoryThresholdPercentage},
		} {
			if isSet(a.v) {
				resp.Diagnostics.AddAttributeError(path.Root(a.name), a.name+" cannot be used without scaling",
					"The Laravel Cloud API requires the scaling thresholds to be empty when scaling_type is \"none\". "+
						"Remove "+a.name+", or set scaling_type to \"custom\" or \"auto\".")
			}
		}
	}

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

	// Outside "auto", min_replicas is a required field of the create request,
	// except for managed queues, where it is optional or prohibited by size.
	isQueue := config.Type.ValueString() == "managed_queue"
	if config.ScalingType.ValueString() == "custom" && !isSet(config.MinReplicas) && !isQueue && !config.Type.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("min_replicas"),
			"min_replicas is required for custom scaling",
			"scaling_type is \"custom\", which requires min_replicas to be set.",
		)
	}
}

// validateAppInstanceConfig checks the settings only app instances have. The
// API ignores them on other types and reports null, so they could never
// converge there, and it answers a timeout outside 1-60 minutes with "The
// hibernation timeout must be null or an integer between 1 and 60."
// configuredInstanceType is the type a configuration gives an instance: an
// unset type defaults to service.
func configuredInstanceType(t types.String) string {
	if t.IsNull() {
		return "service"
	}
	return t.ValueString()
}

func validateAppInstanceConfig(config InstanceResourceModel, diags *diag.Diagnostics) {
	instanceType := configuredInstanceType(config.Type)
	if !config.Type.IsUnknown() && instanceType != "app" {
		for name, v := range map[string]attr.Value{
			"uses_octane":         config.UsesOctane,
			"uses_inertia_ssr":    config.UsesInertiaSSR,
			"hibernation_timeout": config.HibernationTimeout,
		} {
			if !v.IsNull() {
				diags.AddAttributeError(path.Root(name), name+" is only for app instances",
					fmt.Sprintf("Only an environment's app instance has %s; this one is %q. Set type = \"app\" "+
						"to manage the environment's app instance.", name, instanceType))
			}
		}
	}
	if v := config.HibernationTimeout; isSet(v) && v.ValueInt64() != 0 && (v.ValueInt64() < 1 || v.ValueInt64() > 60) {
		diags.AddAttributeError(path.Root("hibernation_timeout"), "hibernation_timeout is out of range",
			fmt.Sprintf("hibernation_timeout is in minutes and must be between 1 and 60, or 0 to turn hibernation "+
				"off; got %d.", v.ValueInt64()))
	}
}

// ModifyPlan leaves the replica counts to the platform when it chooses them.
//
// For "auto" the API sets them to 1 and the plan's replica limit, and for
// "none" to 1 and 1, whatever the instance had before. min_replicas and
// max_replicas are Optional+Computed with UseStateForUnknown, so an unset
// count carries the prior value forward, which is wrong once the scaling type
// changes: switching from "custom" to "auto" would plan the old counts and the
// apply would fail with an inconsistent result. An unset count is unknown
// instead whenever the scaling type is new, and is read back after the apply.
func (r *InstanceResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Nothing to do when the resource is being destroyed.
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan, config InstanceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	scalingType := plan.ScalingType.ValueString()
	if plan.ScalingType.IsUnknown() || (scalingType != "auto" && scalingType != "none") {
		return
	}
	if !req.State.Raw.IsNull() {
		var state InstanceResourceModel
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
		if resp.Diagnostics.HasError() || state.ScalingType.Equal(plan.ScalingType) {
			return
		}
	}

	if config.MinReplicas.IsNull() {
		plan.MinReplicas = types.Int64Unknown()
	}
	if config.MaxReplicas.IsNull() {
		plan.MaxReplicas = types.Int64Unknown()
	}
	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

// fifoQueueSuffix is what the API appends to a FIFO queue's name. A queue
// name may have at most 39 characters including it.
const (
	fifoQueueSuffix     = ".fifo"
	maxFifoQueueNameLen = 39 - len(fifoQueueSuffix)
)

// validateManagedQueueConfig catches what the API would reject or ignore by
// queue generation, which the size names: current-generation sizes start with
// "mq.", previous-generation ones with "mq-", such as "mq-pro-256mb".
func validateManagedQueueConfig(config InstanceResourceModel, diags *diag.Diagnostics) {
	if config.Type.IsUnknown() || config.Size.IsUnknown() {
		return
	}
	isQueue := config.Type.ValueString() == "managed_queue"
	legacyQueue := isQueue && strings.HasPrefix(config.Size.ValueString(), "mq-")
	if isSet(config.Fifo) && config.Fifo.ValueBool() {
		if !isQueue {
			diags.AddAttributeError(path.Root("fifo"), "fifo is only for managed queues",
				fmt.Sprintf("Only a managed_queue instance can be a FIFO queue; this one is %q.", config.Type.ValueString()))
		}
		if n := config.Name.ValueString(); isQueue && isSet(config.Name) && len(n) > maxFifoQueueNameLen {
			diags.AddAttributeError(path.Root("name"), "name is too long for a FIFO queue",
				fmt.Sprintf("Laravel Cloud adds %q to a FIFO queue's name, which can then have at most %d characters; "+
					"%q has %d.", fifoQueueSuffix, maxFifoQueueNameLen, n, len(n)))
		}
	}
	if isSet(config.PollingInterval) && !legacyQueue {
		diags.AddAttributeError(path.Root("polling_interval"), "polling_interval does not apply",
			fmt.Sprintf("Only previous-generation managed queues have a polling interval; the API ignores it for this "+
				"instance (type %q, size %q). Remove polling_interval.", config.Type.ValueString(), config.Size.ValueString()))
	}
	// visibility_timeout and shutdown_timeout are not checked here: Laravel
	// Cloud honors them only for organizations with the advanced managed
	// queues entitlement, which a plan cannot see. Create and Update report it
	// when the API ignores them (queueTimeoutsNotApplied).
	if legacyQueue && isSet(config.MinReplicas) {
		diags.AddAttributeError(path.Root("min_replicas"), "min_replicas does not apply to this queue",
			fmt.Sprintf("Previous-generation managed queues such as size %q always scale to zero when idle, and the API "+
				"rejects min_replicas for them. Remove min_replicas.", config.Size.ValueString()))
	}
}

// validateProbesConfig rejects probes the API would drop. A check, or the
// whole object, with nothing set is stored as nothing, so it could never be
// read back as configured.
func validateProbesConfig(config InstanceResourceModel, diags *diag.Diagnostics) {
	// An unknown value, say from another resource's output, is checked again
	// once it is known.
	p, known := probesFromObject(config.Probes)
	if p == nil || !known {
		return
	}
	if isSet(config.Type) && config.Type.ValueString() != "app" {
		diags.AddAttributeError(path.Root("probes"), "probes are only for app instances",
			fmt.Sprintf("Only app instances accept health checks; this one is %q.", config.Type.ValueString()))
	}
	if p.Startup == nil && p.Readiness == nil && p.Liveness == nil {
		diags.AddAttributeError(path.Root("probes"), "probes sets no check",
			"Set at least one of startup, readiness or liveness, or remove probes to use the platform defaults.")
	}
	for name, c := range map[string]*InstanceProbeModel{"startup": p.Startup, "readiness": p.Readiness, "liveness": p.Liveness} {
		if c != nil && c.Path.IsNull() && c.Port.IsNull() && c.Delay.IsNull() && c.Interval.IsNull() && c.Timeout.IsNull() && c.Tries.IsNull() {
			diags.AddAttributeError(path.Root("probes").AtName(name), "Health check sets no field",
				"Set at least one field of the "+name+" check, or remove it to use the platform default.")
		}
	}
}
