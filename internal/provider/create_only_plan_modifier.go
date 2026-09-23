package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// requiresReplaceUnlessImported forces replacement when a create-only
// attribute changes, except when there is no prior value at all.
//
// Some attributes are accepted on create and never reported back -- a storage
// bucket's initial key_name and key_permission, for example. After an import
// by bare id they are null in state, while the configuration supplies a value
// (or a schema default does). An unconditional RequiresReplace reads that as a
// change and proposes destroying the just-imported resource, taking the
// bucket's contents with it.
//
// A null prior value means "never recorded", not "changed", so it is exempt.
// Every other transition still forces replacement, because these attributes
// genuinely cannot be updated in place.
func requiresReplaceUnlessImported() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(
		func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
			resp.RequiresReplace = !req.StateValue.IsNull()
		},
		"Requires replacement unless the attribute has no prior value, which is the case after an import.",
		"Requires replacement unless the attribute has no prior value, which is the case after an import.",
	)
}

// warnIfNotReported explains the diff on an attribute that no public API
// endpoint reports back. Such an attribute is null in state after an import,
// so the first plan shows the configured value being added even though
// applying it changes nothing -- which reads as though Terraform is about to
// modify the resource. The attribute's own update semantics are untouched.
//
// Only use it where the API genuinely cannot report the value, and where
// applying a null-to-value change leaves the platform as it is.
func warnIfNotReported() planmodifier.String {
	return notReportedModifier{}
}

type notReportedModifier struct{}

func (notReportedModifier) Description(context.Context) string {
	return "Warns when the attribute has no recorded value, as after an import, because the API does not report it."
}

func (m notReportedModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (notReportedModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// Nothing is recorded on create by definition, and nothing is planned on
	// destroy.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	if !req.StateValue.IsNull() || req.PlanValue.IsNull() {
		return
	}
	// A modifier cannot see whether another attribute forces replacement, so
	// the text holds only for an otherwise unchanged resource and says so.
	resp.Diagnostics.AddAttributeWarning(req.Path,
		"Value not reported by Laravel Cloud",
		fmt.Sprintf("The Laravel Cloud API does not report %s, so Terraform has no recorded value for it, "+
			"as after an import. The plan shows the configured value being added; if nothing else changes, "+
			"applying only records it in state and leaves the resource as it is. Terraform cannot detect "+
			"whether this value matches the real one, so make sure it is correct.", req.Path),
	)
}
