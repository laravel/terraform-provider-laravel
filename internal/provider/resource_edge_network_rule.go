package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                = &EdgeNetworkRuleResource{}
	_ resource.ResourceWithImportState = &EdgeNetworkRuleResource{}
	_ resource.ResourceWithModifyPlan  = &EdgeNetworkRuleResource{}
)

// A ruleset takes one change at a time. The API waits five seconds for the
// one in progress, then answers 409; several rules in one apply hit that.
var (
	edgeRuleBusyAttempts = 36
	edgeRuleBusyInterval = 5 * time.Second
)

type EdgeNetworkRuleResource struct {
	client *client.Client
}

type EdgeNetworkRuleResourceModel struct {
	ID            types.String                  `tfsdk:"id"`
	EdgeNetworkID types.String                  `tfsdk:"edge_network_id"`
	Ruleset       types.String                  `tfsdk:"ruleset"`
	Name          types.String                  `tfsdk:"name"`
	Description   types.String                  `tfsdk:"description"`
	IsActive      types.Bool                    `tfsdk:"is_active"`
	Settings      types.String                  `tfsdk:"settings"`
	Position      *EdgeNetworkRulePositionModel `tfsdk:"position"`
}

type EdgeNetworkRulePositionModel struct {
	Before types.String `tfsdk:"before"`
	After  types.String `tfsdk:"after"`
	Index  types.Int64  `tfsdk:"index"`
}

func NewEdgeNetworkRuleResource() resource.Resource {
	return &EdgeNetworkRuleResource{}
}

func (r *EdgeNetworkRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_edge_network_rule"
}

func (r *EdgeNetworkRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	onePosition := []path.Expression{
		path.MatchRelative().AtParent().AtName("before"),
		path.MatchRelative().AtParent().AtName("after"),
		path.MatchRelative().AtParent().AtName("index"),
	}
	resp.Schema = schema.Schema{
		Description: "Manages a custom rule in a ruleset (firewall, rate limit or cache) of a dedicated edge network. " +
			"Rules in a ruleset are evaluated in order. An edge network takes at most 50 custom rules across its rulesets. " +
			"A ruleset takes one change at a time, so a change made while another is in progress is retried for up to three minutes.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Rule ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"edge_network_id": schema.StringAttribute{
				Required: true,
				Description: "ID of a dedicated edge network, such as one from laravel_cloud_edge_networks. " +
					"The shared edge network takes no custom rules. Changing it replaces the rule.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ruleset": schema.StringAttribute{
				Required:    true,
				Description: "Ruleset the rule belongs to: `firewall`, `rate_limit` or `cache`. Changing it replaces the rule.",
				Validators: []validator.String{
					stringvalidator.OneOf("firewall", "rate_limit", "cache"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Rule name, unique within the ruleset (max 100 chars).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 100),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Rule description (max 255 chars).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
				},
			},
			"is_active": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
				Description: "Whether the rule applies. An inactive rule keeps its position but matches no requests. " +
					"Defaults to `true`. Laravel Cloud creates every rule active, so a new rule, or one that a change " +
					"replaces, must be active: set `false` on an existing rule to switch it off.",
			},
			"settings": schema.StringAttribute{
				Required: true,
				Description: "The rule's action and conditions as a JSON object, usually written with jsonencode(). " +
					"Its keys depend on the ruleset; GET /edge-networks/{id}/rulesets/{ruleset} lists the actions, fields " +
					"and operators each accepts. Conditions go in `criteria_groups`: a request matches when every " +
					"condition of at least one group matches. Keys left out take the API's defaults (such as a rate " +
					"limit's `counting_key`) and are not compared, and null values are ignored.",
				Validators: []validator.String{
					jsonObjectValidator{},
				},
			},
			"position": schema.SingleNestedAttribute{
				Optional: true,
				Description: "Where to place the rule among the ruleset's rules: exactly one of `before`, `after` or " +
					"`index`. Without it, a new rule is added last. It is applied when the rule is created and when " +
					"it changes, and is not read back, so moves made outside Terraform are not detected.",
				Attributes: map[string]schema.Attribute{
					"before": schema.StringAttribute{
						Optional:    true,
						Description: "ID of another rule in the ruleset to place this rule before.",
						Validators:  []validator.String{stringvalidator.ExactlyOneOf(onePosition...)},
					},
					"after": schema.StringAttribute{
						Optional:    true,
						Description: "ID of another rule in the ruleset to place this rule after.",
						Validators:  []validator.String{stringvalidator.ExactlyOneOf(onePosition...)},
					},
					"index": schema.Int64Attribute{
						Optional:    true,
						Description: "Position in the ruleset, 1 being first.",
						Validators:  []validator.Int64{int64validator.AtLeast(1)},
					},
				},
			},
		},
	}
}

func (r *EdgeNetworkRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EdgeNetworkRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EdgeNetworkRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// ModifyPlan refuses this when it is known at plan time; this catches a
	// value that was unknown then. Nothing has been created yet.
	if !plan.IsActive.ValueBool() {
		resp.Diagnostics.AddAttributeError(path.Root("is_active"), inactiveRuleCreateSummary, inactiveRuleCreateDetail)
		return
	}

	edgeNetworkID, ruleset := plan.EdgeNetworkID.ValueString(), plan.Ruleset.ValueString()
	createReq := client.CreateEdgeNetworkRuleRequest{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueStringPointer(),
		Settings:    json.RawMessage(plan.Settings.ValueString()),
		Position:    plan.Position.toAPI(),
	}
	var rule *client.EdgeNetworkRuleData
	err := r.retryWhileBusy(ctx, func() error {
		var err error
		rule, err = r.client.CreateEdgeNetworkRule(ctx, edgeNetworkID, ruleset, createReq)
		return err
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating edge network rule", err.Error())
		return
	}
	plan.ID = types.StringValue(rule.ID)
	setEdgeNetworkRuleFromAPI(&plan, rule)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// inactiveRuleCreate explains why a rule the apply would create cannot be
// inactive.
const (
	inactiveRuleCreateSummary = "A new edge network rule cannot start inactive"
	inactiveRuleCreateDetail  = "Laravel Cloud creates every rule active and cannot create one inactive, so this rule " +
		"would apply to traffic as soon as it was created. Create it with is_active = true (or leave is_active unset) " +
		"when it should apply, and set is_active = false on an existing rule to switch it off. A change that replaces " +
		"the rule, such as a new edge_network_id or ruleset, creates a new rule too."
)

// ModifyPlan refuses is_active = false on a rule the apply would create, new
// or replacing another. Laravel Cloud creates every rule active, so the rule
// would apply to traffic until a second request deactivated it, and for good
// if that request failed.
func (r *EdgeNetworkRuleResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	if !req.State.Raw.IsNull() && len(resp.RequiresReplace) == 0 {
		return
	}
	var active types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("is_active"), &active)...)
	if resp.Diagnostics.HasError() || !isSet(active) || active.ValueBool() {
		return
	}
	resp.Diagnostics.AddAttributeError(path.Root("is_active"), inactiveRuleCreateSummary, inactiveRuleCreateDetail)
}

func (r *EdgeNetworkRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EdgeNetworkRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.client.GetEdgeNetworkRule(ctx, state.EdgeNetworkID.ValueString(), state.Ruleset.ValueString(), state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading edge network rule", err.Error())
		return
	}

	setEdgeNetworkRuleFromAPI(&state, rule)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *EdgeNetworkRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state EdgeNetworkRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := client.UpdateEdgeNetworkRuleRequest{
		Name:        plan.Name.ValueStringPointer(),
		Description: plan.Description.ValueStringPointer(),
		IsActive:    plan.IsActive.ValueBoolPointer(),
		Settings:    json.RawMessage(plan.Settings.ValueString()),
	}
	// Moving is the only change the position makes, so it is sent only when
	// it changed; resending it would move the rule back after a move made by
	// another rule's position.
	if !reflect.DeepEqual(plan.Position, state.Position) {
		updateReq.Position = plan.Position.toAPI()
	}

	var rule *client.EdgeNetworkRuleData
	err := r.retryWhileBusy(ctx, func() error {
		var err error
		rule, err = r.client.UpdateEdgeNetworkRule(ctx, state.EdgeNetworkID.ValueString(), state.Ruleset.ValueString(), state.ID.ValueString(), updateReq)
		return err
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating edge network rule", err.Error())
		return
	}

	plan.ID = state.ID
	setEdgeNetworkRuleFromAPI(&plan, rule)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EdgeNetworkRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EdgeNetworkRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	edgeNetworkID, ruleset, id := state.EdgeNetworkID.ValueString(), state.Ruleset.ValueString(), state.ID.ValueString()
	err := r.retryWhileBusy(ctx, func() error {
		return r.client.DeleteEdgeNetworkRule(ctx, edgeNetworkID, ruleset, id)
	})
	if err != nil {
		// Deleting a rule that is already gone is answered with a 302 to
		// the dashboard rather than a 404, so ask for it to tell.
		if _, getErr := r.client.GetEdgeNetworkRule(ctx, edgeNetworkID, ruleset, id); client.IsNotFound(getErr) {
			return
		}
		resp.Diagnostics.AddError("Error deleting edge network rule", err.Error())
	}
}

func (r *EdgeNetworkRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"edge_network_id:ruleset:rule_id\", such as "+
				"\"edgenet-...:firewall:nru-...\", got %q.", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("edge_network_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("ruleset"), strings.ReplaceAll(parts[1], "-", "_"))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[2])...)
}

func (r *EdgeNetworkRuleResource) retryWhileBusy(ctx context.Context, operation func() error) error {
	return client.RetryOnConflict(ctx, edgeRuleBusyAttempts, edgeRuleBusyInterval, operation, client.IsRulesetBusy)
}

func (p *EdgeNetworkRulePositionModel) toAPI() *client.EdgeNetworkRulePosition {
	if p == nil {
		return nil
	}
	return &client.EdgeNetworkRulePosition{
		Before: p.Before.ValueStringPointer(),
		After:  p.After.ValueStringPointer(),
		Index:  p.Index.ValueInt64Pointer(),
	}
}

// setEdgeNetworkRuleFromAPI copies the rule the API reports into the model.
// The position is left alone: the API does not report one. The settings keep
// the model's JSON when the API's match it, since the API adds nulls and
// defaults for keys the configuration left out.
func setEdgeNetworkRuleFromAPI(m *EdgeNetworkRuleResourceModel, rule *client.EdgeNetworkRuleData) {
	m.Ruleset = types.StringValue(rule.Attributes.Type)
	m.Name = types.StringValue(rule.Attributes.Name)
	m.Description = types.StringPointerValue(rule.Attributes.Description)
	m.IsActive = types.BoolValue(rule.Attributes.IsActive)
	if m.Settings.IsNull() || m.Settings.IsUnknown() || !settingsMatch(m.Settings.ValueString(), rule.Attributes.Settings) {
		m.Settings = types.StringValue(compactSettings(rule.Attributes.Settings))
	}
}

// settingsMatch reports whether every non-null value in want is in got.
func settingsMatch(want string, got json.RawMessage) bool {
	var w, g any
	if json.Unmarshal([]byte(want), &w) != nil || json.Unmarshal(got, &g) != nil {
		return false
	}
	return jsonContains(g, w)
}

func jsonContains(got, want any) bool {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return false
		}
		for k, v := range w {
			if v != nil && !jsonContains(g[k], v) {
				return false
			}
		}
		return true
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !jsonContains(g[i], w[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(got, want)
	}
}

// compactSettings returns the settings without their null values, which the
// API reports for every key the rule does not use, encoded the way
// jsonencode() encodes them: keys sorted, no spaces.
func compactSettings(raw json.RawMessage) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	out, err := json.Marshal(dropNulls(v))
	if err != nil {
		return string(raw)
	}
	return string(out)
}

func dropNulls(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range t {
			if val != nil {
				out[k] = dropNulls(val)
			}
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = dropNulls(val)
		}
		return out
	default:
		return v
	}
}

// jsonObjectValidator checks that a string is a JSON object.
type jsonObjectValidator struct{}

func (jsonObjectValidator) Description(context.Context) string {
	return "must be a JSON object"
}

func (v jsonObjectValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (jsonObjectValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(req.ConfigValue.ValueString()), &obj); err != nil || obj == nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid JSON object",
			"settings must be a JSON object, such as jsonencode({ action = \"block\", criteria_groups = [...] }).")
	}
}
