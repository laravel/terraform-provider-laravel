package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                   = &BackgroundProcessResource{}
	_ resource.ResourceWithImportState    = &BackgroundProcessResource{}
	_ resource.ResourceWithValidateConfig = &BackgroundProcessResource{}
)

// workerConfigKeys are the config options the API accepts, in order. It
// silently drops any other key, so one would never be stored.
var workerConfigKeys = []string{"backoff", "connection", "force", "queue", "rest", "sleep", "timeout", "tries"}

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
				Description: "Process type (worker, custom). Changing it updates the process in place.",
			},
			"processes": schema.Int64Attribute{
				Required:    true,
				Description: "Number of processes (1-10).",
			},
			"command": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Command to run. Required when type is custom (3-500 chars). For a worker, Laravel Cloud " +
					"generates the queue:work command from config, so leave it unset.",
				PlanModifiers: []planmodifier.String{
					workerCommandModifier{},
				},
			},
			"config": schema.StringAttribute{
				Optional: true,
				Description: "JSON-encoded worker configuration, such as jsonencode({ queue = \"default\", tries = 3 }). " +
					"Keys: " + strings.Join(workerConfigKeys, ", ") + ". Only applies to workers.",
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

// workerCommandModifier keeps a worker's generated command in the plan while
// nothing it is generated from changes. The API rewrites a worker's command
// whenever its type or config is written, so the command is only known after
// apply then.
type workerCommandModifier struct{}

func (workerCommandModifier) Description(context.Context) string {
	return "Keeps the command Laravel Cloud generated unless type or config changes."
}

func (m workerCommandModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (workerCommandModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() || !req.ConfigValue.IsNull() || !resp.PlanValue.IsUnknown() {
		return
	}
	var plan, state BackgroundProcessResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.Type.Equal(state.Type) && plan.Config.Equal(state.Config) {
		resp.PlanValue = req.StateValue
	}
}

// ValidateConfig rejects at plan time what the API would silently overwrite or
// drop, each of which used to surface as an inconsistent result after apply or
// as a perpetual diff.
func (r *BackgroundProcessResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config BackgroundProcessResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	switch config.Type.ValueString() {
	case "worker":
		if !config.Command.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("command"), "command cannot be set for a worker",
				"Laravel Cloud generates a worker's queue:work command from its config and ignores any command sent. "+
					"Remove command and set the worker's options in config instead.")
		}
	case "custom":
		if !config.Config.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("config"), "config cannot be set for a custom process",
				"Laravel Cloud discards config for a custom process. Remove config and put any options in command.")
		}
	}

	if config.Config.IsNull() || config.Config.IsUnknown() {
		return
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(config.Config.ValueString()), &cfg); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("config"), "Invalid config JSON",
			"config must be a JSON object, such as jsonencode({ queue = \"default\" }): "+err.Error())
		return
	}
	for key := range cfg {
		if !slices.Contains(workerConfigKeys, key) {
			resp.Diagnostics.AddAttributeError(path.Root("config"), "Unsupported config key",
				fmt.Sprintf("Laravel Cloud ignores the config key %q. Supported keys: %s.", key, strings.Join(workerConfigKeys, ", ")))
		}
	}
}

func (r *BackgroundProcessResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	if !plan.Command.IsNull() && !plan.Command.IsUnknown() {
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

	mapBackgroundProcessToState(bp, &plan, false)
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

	mapBackgroundProcessToState(bp, &state, true)
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
	typeChanged := !plan.Type.Equal(state.Type)
	if typeChanged {
		v := plan.Type.ValueString()
		updateReq.Type = &v
	}
	if !plan.Processes.Equal(state.Processes) {
		v := int(plan.Processes.ValueInt64())
		updateReq.Processes = &v
	}
	// The API requires the command whenever type is set to custom.
	if !plan.Command.IsNull() && !plan.Command.IsUnknown() && (typeChanged || !plan.Command.Equal(state.Command)) {
		v := plan.Command.ValueString()
		updateReq.Command = &v
	}
	if !plan.Config.Equal(state.Config) && plan.Type.ValueString() == "worker" {
		if plan.Config.IsNull() {
			// The API replaces the whole config, but ignores an empty one, so
			// clearing it means setting each stored key to null.
			cfg, err := decodeWorkerConfig(json.RawMessage(state.Config.ValueString()))
			if err != nil {
				resp.Diagnostics.AddError("Invalid config JSON in state", err.Error())
				return
			}
			for k := range cfg {
				cfg[k] = nil
			}
			updateReq.Config = cfg
		} else {
			var cfg map[string]any
			if err := json.Unmarshal([]byte(plan.Config.ValueString()), &cfg); err != nil {
				resp.Diagnostics.AddError("Invalid config JSON", err.Error())
				return
			}
			updateReq.Config = cfg
		}
	}

	bp, err := r.client.UpdateBackgroundProcess(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating background process", err.Error())
		return
	}

	plan.InstanceID = state.InstanceID
	mapBackgroundProcessToState(bp, &plan, false)
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

// mapBackgroundProcessToState copies the API's view of a process into state.
// Create and Update must return config exactly as planned, so only Read
// (refresh and import) takes config from the API.
func mapBackgroundProcessToState(bp *client.BackgroundProcessData, state *BackgroundProcessResourceModel, readConfig bool) {
	if instanceID := bp.Relationships.Instance.RelatedID(); instanceID != "" {
		state.InstanceID = types.StringValue(instanceID)
	}
	state.ID = types.StringValue(bp.ID)
	state.Type = types.StringValue(bp.Attributes.ProcessType)
	state.Processes = types.Int64Value(int64(bp.Attributes.Processes))
	// The create response is the unreloaded model, so it omits the column's
	// default, which every later read reports as "none".
	state.StrategyType = types.StringValue(bp.Attributes.StrategyType)
	if bp.Attributes.StrategyType == "" {
		state.StrategyType = types.StringValue("none")
	}
	if bp.Attributes.StrategyThreshold != nil {
		state.StrategyThreshold = types.Int64Value(int64(*bp.Attributes.StrategyThreshold))
	} else {
		state.StrategyThreshold = types.Int64Null()
	}
	state.CreatedAt = types.StringPointerValue(bp.Attributes.CreatedAt)
	state.Command = types.StringPointerValue(bp.Attributes.Command)

	// A custom process has no config: the API stores none for it, and keeps a
	// worker's old config when its type changes to custom.
	if readConfig && bp.Attributes.ProcessType == "worker" {
		state.Config = workerConfigState(bp.Attributes.Config, state.Config)
	}
}

// workerConfigState returns the config to record for the API's stored config.
// The API can report keys as null that were never set, so those are dropped,
// and the prior value is kept when it means the same, so formatting and key
// order in the configuration never show as a diff.
func workerConfigState(raw json.RawMessage, prior types.String) types.String {
	cfg, err := decodeWorkerConfig(raw)
	if err != nil {
		return prior
	}
	if !prior.IsNull() && !prior.IsUnknown() {
		if priorCfg, err := decodeWorkerConfig(json.RawMessage(prior.ValueString())); err == nil && reflect.DeepEqual(cfg, priorCfg) {
			return prior
		}
	}
	if len(cfg) == 0 {
		return types.StringNull()
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return prior
	}
	return types.StringValue(string(b))
}

// decodeWorkerConfig decodes a stored config, without its null keys. The API
// returns an empty config as [] rather than {}.
func decodeWorkerConfig(raw json.RawMessage) (map[string]any, error) {
	cfg := map[string]any{}
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" || s == "[]" {
		return cfg, nil
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	for k, v := range cfg {
		if v == nil {
			delete(cfg, k)
		}
	}
	return cfg, nil
}
