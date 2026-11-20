package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

// followsAttributes keeps a read-only attribute's prior value, like
// UseStateForUnknown, except when the configuration changes one of the
// attributes it is derived from: Laravel Cloud may then report a new value, so
// it is left unknown for the apply to fill in.
type followsAttributes []string

func (f followsAttributes) Description(context.Context) string {
	return "Keeps the prior value unless " + strings.Join(f, " or ") + " changes."
}

func (f followsAttributes) MarkdownDescription(ctx context.Context) string {
	return f.Description(ctx)
}

// keep reports whether the prior value still holds.
func (f followsAttributes) keep(ctx context.Context, state tfsdk.State, plan tfsdk.Plan, config tfsdk.Config, configValue attr.Value) bool {
	// No prior value on create, nothing planned on destroy, and a configured
	// value stands as it is.
	if state.Raw.IsNull() || plan.Raw.IsNull() || !configValue.IsNull() {
		return false
	}
	// The configuration, not the plan: another attribute's plan modifiers may
	// not have run yet. An attribute left unset keeps its prior value.
	for _, name := range f {
		var configured, prior attr.Value
		if config.GetAttribute(ctx, path.Root(name), &configured).HasError() ||
			state.GetAttribute(ctx, path.Root(name), &prior).HasError() {
			return false
		}
		if configured.IsUnknown() || !configured.IsNull() && !configured.Equal(prior) {
			return false
		}
	}
	return true
}

func (f followsAttributes) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.PlanValue.IsUnknown() && f.keep(ctx, req.State, req.Plan, req.Config, req.ConfigValue) {
		resp.PlanValue = req.StateValue
	}
}

func (f followsAttributes) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.PlanValue.IsUnknown() && f.keep(ctx, req.State, req.Plan, req.Config, req.ConfigValue) {
		resp.PlanValue = req.StateValue
	}
}

func (f followsAttributes) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.PlanValue.IsUnknown() && f.keep(ctx, req.State, req.Plan, req.Config, req.ConfigValue) {
		resp.PlanValue = req.StateValue
	}
}
