package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// requiresReplaceUnlessImported must keep create-only semantics for a resource
// this configuration created, while exempting the one case that made import
// destructive: an attribute with no prior value, because the API never
// reported it back.
func TestRequiresReplaceUnlessImported(t *testing.T) {
	ctx := context.Background()

	// A one-attribute resource is enough to give the modifier a non-null
	// State.Raw and Plan.Raw; RequiresReplaceIf returns early without them,
	// treating the change as a create or a destroy.
	sch := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"key_name": schema.StringAttribute{Optional: true},
		},
	}
	objType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"key_name": tftypes.String}}

	raw := func(v types.String) tftypes.Value {
		if v.IsNull() {
			return tftypes.NewValue(objType, map[string]tftypes.Value{
				"key_name": tftypes.NewValue(tftypes.String, nil),
			})
		}
		return tftypes.NewValue(objType, map[string]tftypes.Value{
			"key_name": tftypes.NewValue(tftypes.String, v.ValueString()),
		})
	}

	tests := []struct {
		name  string
		state types.String
		plan  types.String
		want  bool
	}{
		{"imported: no prior value, must not replace", types.StringNull(), types.StringValue("primary"), false},
		{"changed: still forces replacement", types.StringValue("primary"), types.StringValue("secondary"), true},
		{"cleared: still forces replacement", types.StringValue("primary"), types.StringNull(), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := planmodifier.StringRequest{
				Path:       path.Root("key_name"),
				StateValue: tc.state,
				PlanValue:  tc.plan,
				State:      tfsdk.State{Schema: sch, Raw: raw(tc.state)},
				Plan:       tfsdk.Plan{Schema: sch, Raw: raw(tc.plan)},
			}
			resp := &planmodifier.StringResponse{PlanValue: tc.plan}

			requiresReplaceUnlessImported().PlanModifyString(ctx, req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}
			if resp.RequiresReplace != tc.want {
				t.Fatalf("RequiresReplace = %v, want %v", resp.RequiresReplace, tc.want)
			}
		})
	}
}

// warnIfNotReported must explain an attribute that shows as added only because
// the API never reported it, and stay silent otherwise.
func TestWarnIfNotReported(t *testing.T) {
	ctx := context.Background()

	sch := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"source_control_provider_type": schema.StringAttribute{Optional: true},
		},
	}
	objType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"source_control_provider_type": tftypes.String}}
	raw := func(v types.String) tftypes.Value {
		var inner tftypes.Value
		if v.IsNull() {
			inner = tftypes.NewValue(tftypes.String, nil)
		} else {
			inner = tftypes.NewValue(tftypes.String, v.ValueString())
		}
		return tftypes.NewValue(objType, map[string]tftypes.Value{"source_control_provider_type": inner})
	}
	noResource := tftypes.NewValue(objType, nil)

	tests := []struct {
		name      string
		state     types.String
		plan      types.String
		stateRaw  *tftypes.Value
		wantsWarn bool
	}{
		{"imported: configured value, nothing recorded", types.StringNull(), types.StringValue("github"), nil, true},
		{"recorded: same value", types.StringValue("github"), types.StringValue("github"), nil, false},
		{"recorded: changed value", types.StringValue("github"), types.StringValue("gitlab"), nil, false},
		{"unset in config", types.StringNull(), types.StringNull(), nil, false},
		{"create: no prior resource", types.StringNull(), types.StringValue("github"), &noResource, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stateRaw := raw(tc.state)
			if tc.stateRaw != nil {
				stateRaw = *tc.stateRaw
			}
			req := planmodifier.StringRequest{
				Path:       path.Root("source_control_provider_type"),
				StateValue: tc.state,
				PlanValue:  tc.plan,
				State:      tfsdk.State{Schema: sch, Raw: stateRaw},
				Plan:       tfsdk.Plan{Schema: sch, Raw: raw(tc.plan)},
			}
			resp := &planmodifier.StringResponse{PlanValue: tc.plan}

			warnIfNotReported().PlanModifyString(ctx, req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error diagnostics: %v", resp.Diagnostics)
			}
			if got := resp.Diagnostics.WarningsCount() > 0; got != tc.wantsWarn {
				t.Fatalf("warned = %v, want %v: %v", got, tc.wantsWarn, resp.Diagnostics)
			}
			if !resp.PlanValue.Equal(tc.plan) {
				t.Fatalf("plan value changed to %v", resp.PlanValue)
			}
		})
	}
}
