package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ resource.Resource = &EnvironmentVariableResource{}

type EnvironmentVariableResource struct {
	resourceWithClient
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
		Description: "Manages environment variables for a Laravel Cloud environment. This resource replaces ALL environment variables on every apply.",
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
				Description: "Map of environment variable key-value pairs. All variables are replaced on each apply.",
			},
		},
	}
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

	// The Laravel Cloud API does not expose a GET endpoint for environment variables.
	// We trust that state is correct; Terraform detects drift via the plan diff.
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *EnvironmentVariableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
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
		resp.Diagnostics.AddError("Error replacing environment variables", err.Error())
		return
	}

	plan.ID = plan.EnvironmentID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
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
