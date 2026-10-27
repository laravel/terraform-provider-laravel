package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	_ resource.Resource                = &EnvironmentResendResource{}
	_ resource.ResourceWithImportState = &EnvironmentResendResource{}
)

// EnvironmentResendResource connects Resend email sending to an environment.
// An environment has at most one Resend integration, so the resource id is the
// environment id.
type EnvironmentResendResource struct {
	client *client.Client
}

type EnvironmentResendResourceModel struct {
	ID            types.String `tfsdk:"id"`
	EnvironmentID types.String `tfsdk:"environment_id"`
	FromAddress   types.String `tfsdk:"from_address"`
	FromName      types.String `tfsdk:"from_name"`
	KeyName       types.String `tfsdk:"key_name"`
	ReuseKeyID    types.String `tfsdk:"reuse_key_id"`
}

func NewEnvironmentResendResource() resource.Resource {
	return &EnvironmentResendResource{}
}

func (r *EnvironmentResendResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_environment_resend"
}

func (r *EnvironmentResendResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Connects Resend email sending to an environment, with a new sending key or one reused " +
			"from another environment. The organization must have Resend connected, and from_address must " +
			"be on a domain verified in Resend. Destroying this detaches Resend from the environment and " +
			"deletes its sending key unless another environment still uses it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Same as environment_id.",
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
			"from_address": schema.StringAttribute{
				Required:    true,
				Description: "Sender email address, on a domain verified in Resend.",
			},
			"from_name": schema.StringAttribute{
				Required:    true,
				Description: "Sender name (max 255 chars).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
				},
			},
			"key_name": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Name of the sending key created for the environment (max 255 chars). Defaults to a " +
					"name the platform derives from the environment. With reuse_key_id, this reports the reused " +
					"key's name and cannot be set. Changing it creates a new key.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
					stringvalidator.ConflictsWith(path.MatchRoot("reuse_key_id")),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"reuse_key_id": schema.StringAttribute{
				Optional: true,
				Description: "ID of another environment's sending key to reuse instead of creating one. " +
					"Changing it forces a new integration. The API never reports it back, so an imported " +
					"integration leaves it null.",
				PlanModifiers: []planmodifier.String{
					requiresReplaceUnlessImported(),
				},
			},
		},
	}
}

func (r *EnvironmentResendResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EnvironmentResendResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EnvironmentResendResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	strategy := "create"
	attachReq := client.AttachResendRequest{
		FromAddress: plan.FromAddress.ValueString(),
		FromName:    plan.FromName.ValueString(),
	}
	if !plan.ReuseKeyID.IsNull() {
		strategy = "reuse"
		attachReq.ReuseKeyID = plan.ReuseKeyID.ValueStringPointer()
	} else if !plan.KeyName.IsNull() && !plan.KeyName.IsUnknown() {
		attachReq.KeyName = plan.KeyName.ValueStringPointer()
	}
	attachReq.KeyStrategy = &strategy

	envID := plan.EnvironmentID.ValueString()
	env, err := r.client.AttachResend(ctx, envID, attachReq)
	if err != nil {
		resp.Diagnostics.AddError("Error attaching Resend to environment", err.Error())
		return
	}
	if env.Attributes.Resend == nil {
		// The attach response is the environment; re-read it rather than
		// guessing the key name.
		if env, err = r.client.GetEnvironment(ctx, envID); err != nil {
			resp.Diagnostics.AddError("Error reading environment after attaching Resend", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(envID)
	mapResendToState(env.Attributes.Resend, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EnvironmentResendResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EnvironmentResendResourceModel
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
	if env.Attributes.Resend == nil {
		// Detached outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}

	mapResendToState(env.Attributes.Resend, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update changes the sender. Everything else about the integration forces
// replacement, because the sending key cannot be changed in place.
func (r *EnvironmentResendResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state EnvironmentResendResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// reuse_key_id being recorded after an import changes nothing remotely.
	if plan.FromAddress.Equal(state.FromAddress) && plan.FromName.Equal(state.FromName) {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	env, err := r.client.UpdateResend(ctx, state.EnvironmentID.ValueString(), client.UpdateResendRequest{
		FromAddress: plan.FromAddress.ValueString(),
		FromName:    plan.FromName.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating Resend sender", err.Error())
		return
	}
	if env.Attributes.Resend != nil {
		mapResendToState(env.Attributes.Resend, &plan)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EnvironmentResendResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EnvironmentResendResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DetachResend(ctx, state.EnvironmentID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error detaching Resend from environment", err.Error())
	}
}

func (r *EnvironmentResendResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), req.ID)...)
}

func mapResendToState(rs *client.EnvironmentResend, state *EnvironmentResendResourceModel) {
	if rs == nil {
		return
	}
	state.FromAddress = types.StringValue(rs.FromAddress)
	state.FromName = types.StringValue(rs.FromName)
	state.KeyName = types.StringValue(rs.KeyName)
}
