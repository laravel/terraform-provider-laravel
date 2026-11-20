package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                   = &EnvironmentInternalNetworkingResource{}
	_ resource.ResourceWithImportState    = &EnvironmentInternalNetworkingResource{}
	_ resource.ResourceWithValidateConfig = &EnvironmentInternalNetworkingResource{}
)

type EnvironmentInternalNetworkingResource struct {
	client *client.Client
}

type EnvironmentInternalNetworkingResourceModel struct {
	ID             types.String `tfsdk:"id"`
	EnvironmentID  types.String `tfsdk:"environment_id"`
	AccessLevel    types.String `tfsdk:"access_level"`
	EnvironmentIDs types.Set    `tfsdk:"environment_ids"`
	Hostname       types.String `tfsdk:"hostname"`
}

func NewEnvironmentInternalNetworkingResource() resource.Resource {
	return &EnvironmentInternalNetworkingResource{}
}

func (r *EnvironmentInternalNetworkingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_environment_internal_networking"
}

func (r *EnvironmentInternalNetworkingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Turns on internal networking for an environment, so other environments on the same dedicated cluster " +
			"can reach it over the private network at its internal hostname. Private Cloud only: the organization needs " +
			"the internal networking entitlement and the environment must run on a dedicated cluster. Destroying this " +
			"resource turns internal networking off.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The environment ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				Required:    true,
				Description: "ID of an environment on a dedicated cluster. Changing it replaces the resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"access_level": schema.StringAttribute{
				Required: true,
				Description: "Which environments can send internal traffic to this one: `all` environments on the same " +
					"cluster, or only the `specific` ones in environment_ids.",
				Validators: []validator.String{
					stringvalidator.OneOf("all", "specific"),
				},
			},
			"environment_ids": schema.SetAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "IDs of the environments that can send internal traffic to this one. Required with " +
					"access_level `specific`, and not allowed with `all`. Each must be another environment of the " +
					"organization on the same cluster.",
			},
			"hostname": schema.StringAttribute{
				Computed:    true,
				Description: "The private hostname other environments use to reach this one.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *EnvironmentInternalNetworkingResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config EnvironmentInternalNetworkingResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config.AccessLevel.IsUnknown() || config.EnvironmentIDs.IsUnknown() {
		return
	}
	switch config.AccessLevel.ValueString() {
	case "specific":
		if config.EnvironmentIDs.IsNull() || len(config.EnvironmentIDs.Elements()) == 0 {
			resp.Diagnostics.AddAttributeError(path.Root("environment_ids"), "Missing environment_ids",
				"access_level \"specific\" needs at least one environment in environment_ids.")
		}
	case "all":
		if !config.EnvironmentIDs.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("environment_ids"), "Unexpected environment_ids",
				"access_level \"all\" lets every environment on the cluster in; remove environment_ids or use \"specific\".")
		}
	}
}

func (r *EnvironmentInternalNetworkingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EnvironmentInternalNetworkingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EnvironmentInternalNetworkingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq, diags := plan.toAPI(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := r.client.EnableInternalNetworking(ctx, plan.EnvironmentID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error enabling internal networking", err.Error())
		return
	}
	resp.Diagnostics.Append(plan.setFromAPI(ctx, env)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EnvironmentInternalNetworkingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EnvironmentInternalNetworkingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := r.client.GetEnvironment(ctx, state.EnvironmentID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading environment", err.Error())
		return
	}
	// Turned off outside Terraform: plan to turn it on again.
	if env.Attributes.InternalNetworking == nil || !env.Attributes.InternalNetworking.Enabled {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(state.setFromAPI(ctx, env)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *EnvironmentInternalNetworkingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan EnvironmentInternalNetworkingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq, diags := plan.toAPI(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := r.client.UpdateInternalNetworking(ctx, plan.EnvironmentID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating internal networking", err.Error())
		return
	}
	resp.Diagnostics.Append(plan.setFromAPI(ctx, env)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EnvironmentInternalNetworkingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EnvironmentInternalNetworkingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.EnvironmentID.ValueString()
	if err := r.client.DisableInternalNetworking(ctx, id); err != nil {
		// A write to an environment that is gone is answered with a 302 to
		// the dashboard rather than a 404, so ask for it to tell.
		if _, getErr := r.client.GetEnvironment(ctx, id); client.IsNotFound(getErr) {
			return
		}
		resp.Diagnostics.AddError("Error disabling internal networking", err.Error())
	}
}

func (r *EnvironmentInternalNetworkingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (m *EnvironmentInternalNetworkingResourceModel) toAPI(ctx context.Context) (client.InternalNetworkingRequest, diag.Diagnostics) {
	req := client.InternalNetworkingRequest{AccessLevel: m.AccessLevel.ValueString()}
	var diags diag.Diagnostics
	if !m.EnvironmentIDs.IsNull() {
		diags = m.EnvironmentIDs.ElementsAs(ctx, &req.EnvironmentIDs, false)
	}
	return req, diags
}

// setFromAPI copies the environment's internal networking into the model.
// With access_level "all" the API reports no environments, which is kept as
// null, the only value the configuration can have then.
func (m *EnvironmentInternalNetworkingResourceModel) setFromAPI(ctx context.Context, env *client.EnvironmentData) diag.Diagnostics {
	m.ID = types.StringValue(env.ID)
	m.EnvironmentID = types.StringValue(env.ID)
	in := env.Attributes.InternalNetworking
	if in == nil {
		in = &client.EnvironmentInternalNetworking{}
	}
	m.AccessLevel = types.StringPointerValue(in.AccessLevel)
	m.Hostname = types.StringPointerValue(in.Hostname)
	if len(in.EnvironmentIDs) == 0 && m.AccessLevel.ValueString() != "specific" {
		m.EnvironmentIDs = types.SetNull(types.StringType)
		return nil
	}
	var diags diag.Diagnostics
	m.EnvironmentIDs, diags = types.SetValueFrom(ctx, types.StringType, in.EnvironmentIDs)
	return diags
}
