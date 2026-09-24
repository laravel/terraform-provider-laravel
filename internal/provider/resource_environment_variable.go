package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ resource.Resource = &EnvironmentVariableResource{}

type EnvironmentVariableResource struct {
	client *client.Client
}

type EnvironmentVariableResourceModel struct {
	ID            types.String            `tfsdk:"id"`
	EnvironmentID types.String            `tfsdk:"environment_id"`
	Variables     map[string]types.String `tfsdk:"variables"`
}

func NewEnvironmentVariableResource() resource.Resource {
	return &EnvironmentVariableResource{}
}

func (r *EnvironmentVariableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_environment_variables"
}

func (r *EnvironmentVariableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages environment variables for a Laravel Cloud environment. Keys in this map are written on every apply, and a key removed from the map is deleted from the environment. Variables set outside Terraform are left alone: they are neither read into state nor removed.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Computed identifier (same as environment_id).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				Required:    true,
				Description: "Environment ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"variables": schema.MapAttribute{
				Required:    true,
				ElementType: types.StringType,
				Sensitive:   true,
				Description: "Map of environment variable key-value pairs. Removing a key deletes that variable from the environment.",
			},
		},
	}
}

func (r *EnvironmentVariableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EnvironmentVariableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EnvironmentVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vars := buildVariablesList(plan.Variables)

	setReq := client.AddEnvironmentVariablesRequest{
		Method:    "set",
		Variables: vars,
	}

	_, err := r.client.SetEnvironmentVariables(ctx, plan.EnvironmentID.ValueString(), setReq)
	if err != nil {
		resp.Diagnostics.AddError("Error setting environment variables", err.Error())
		return
	}

	plan.ID = plan.EnvironmentID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EnvironmentVariableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EnvironmentVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// GET /environments/{id}/variables answers 405 -- that route takes POST
	// only -- but the environment payload carries the variables, keys and
	// values both, so drift is detectable after all.
	env, err := r.client.GetEnvironment(ctx, state.EnvironmentID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			// The environment is gone, and with it the variables.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading environment variables", err.Error())
		return
	}

	remote := make(map[string]string, len(env.Attributes.EnvironmentVariables))
	for _, v := range env.Attributes.EnvironmentVariables {
		remote[v.Key] = v.Value
	}

	// Only the keys already in state are refreshed. The environment also holds
	// variables this resource never set -- from the dashboard, another tool, or
	// the platform itself -- and adopting those would be actively destructive:
	// they would enter state, the next plan would see them missing from the
	// configuration, and Update would delete them. Reading is not a licence to
	// take ownership of things Terraform did not create.
	refreshed := make(map[string]types.String, len(state.Variables))
	for key := range state.Variables {
		if value, ok := remote[key]; ok {
			refreshed[key] = types.StringValue(value)
		}
		// A key that is no longer there is dropped, which is what makes an
		// external deletion show up as a diff that puts it back.
	}
	state.Variables = refreshed

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *EnvironmentVariableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan EnvironmentVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state EnvironmentVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envID := plan.EnvironmentID.ValueString()

	// A key dropped from the configuration is not mentioned by the "set" call
	// below, so it has to be deleted by name. The API keeps a separate delete
	// route precisely because "set" writes the keys it is given rather than
	// replacing the whole set -- without this, removing a variable took it out
	// of state while leaving it live in the environment, which is the wrong
	// direction to be wrong in for something that holds credentials.
	if removed := removedVariableKeys(plan.Variables, state.Variables); len(removed) > 0 {
		req := client.DeleteEnvironmentVariablesRequest{Keys: removed}
		if err := r.client.DeleteEnvironmentVariables(ctx, envID, req); err != nil && !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Error removing environment variables", err.Error())
			return
		}
	}

	setReq := client.AddEnvironmentVariablesRequest{
		Method:    "set",
		Variables: buildVariablesList(plan.Variables),
	}

	if _, err := r.client.SetEnvironmentVariables(ctx, envID, setReq); err != nil {
		resp.Diagnostics.AddError("Error replacing environment variables", err.Error())
		return
	}

	plan.ID = plan.EnvironmentID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// removedVariableKeys returns the keys present in state but no longer in plan.
func removedVariableKeys(plan, state map[string]types.String) []string {
	var removed []string
	for k := range state {
		if _, ok := plan[k]; !ok {
			removed = append(removed, k)
		}
	}
	sort.Strings(removed)
	return removed
}

func (r *EnvironmentVariableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EnvironmentVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keys := make([]string, 0, len(state.Variables))
	for k := range state.Variables {
		keys = append(keys, k)
	}

	if len(keys) > 0 {
		deleteReq := client.DeleteEnvironmentVariablesRequest{
			Keys: keys,
		}
		if err := r.client.DeleteEnvironmentVariables(ctx, state.EnvironmentID.ValueString(), deleteReq); err != nil {
			resp.Diagnostics.AddError("Error deleting environment variables", err.Error())
		}
	}
}

func buildVariablesList(vars map[string]types.String) []client.EnvironmentVariable {
	result := make([]client.EnvironmentVariable, 0, len(vars))
	for k, v := range vars {
		result = append(result, client.EnvironmentVariable{
			Key:   k,
			Value: v.ValueString(),
		})
	}
	return result
}
