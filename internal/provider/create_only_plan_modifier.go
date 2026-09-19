package provider

import (
	"context"

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
