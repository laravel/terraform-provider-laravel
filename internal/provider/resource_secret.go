package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                = &SecretResource{}
	_ resource.ResourceWithImportState = &SecretResource{}
)

// secretKeyRe is the key format the API accepts.
var secretKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type SecretResource struct {
	client *client.Client
}

type SecretResourceModel struct {
	ID    types.String `tfsdk:"id"`
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
	Notes types.String `tfsdk:"notes"`
	// ValueWO is write-only: it is in the configuration and nowhere else, so
	// it is always null here and is read from req.Config.
	ValueWO        types.String `tfsdk:"value_wo"`
	ValueWOVersion types.Int64  `tfsdk:"value_wo_version"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func NewSecretResource() resource.Resource {
	return &SecretResource{}
}

func (r *SecretResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_secret"
}

func (r *SecretResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an organization secret. The value is encrypted to the organization's public key " +
			"before it leaves Terraform, and the API never returns it. Attach a secret to an environment " +
			"with laravel_cloud_environment_secret.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key": schema.StringAttribute{
				Required: true,
				Description: "The secret's name (max 255 chars). It must start with a letter or underscore " +
					"and contain only letters, numbers and underscores.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
					stringvalidator.RegexMatches(secretKeyRe, "must start with a letter or underscore and contain only letters, numbers and underscores"),
				},
			},
			"value": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "The secret's value. It is stored in Terraform state, as every sensitive value is; " +
					"with Terraform 1.11 or later, use value_wo instead to keep it out of state. Exactly one of " +
					"value and value_wo is required. The API never returns it, so a value changed outside " +
					"Terraform is not detected, and the first apply after an import writes the configured value.",
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot("value_wo")),
				},
			},
			"value_wo": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "The secret's value as a write-only argument (Terraform 1.11 or later): it is sent " +
					"and never stored in plan or state. Because Terraform cannot see it change, it is sent " +
					"only on create and when value_wo_version changes; bump the version to rotate the value.",
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("value_wo_version")),
				},
			},
			"value_wo_version": schema.Int64Attribute{
				Optional:    true,
				Description: "Version of value_wo. Changing it sends the current value_wo; it has no other effect.",
				Validators: []validator.Int64{
					int64validator.AlsoRequires(path.MatchRoot("value_wo")),
				},
			},
			"notes": schema.StringAttribute{
				Optional:    true,
				Description: "Free-form notes about the secret (max 500 chars).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 500),
				},
			},
			"created_at": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *SecretResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SecretResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	value, d := secretValue(ctx, req.Config, plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	keyPairID, sealed, err := r.client.SealSecretValue(ctx, value)
	if err != nil {
		resp.Diagnostics.AddError("Error encrypting secret value", err.Error())
		return
	}

	secret, err := r.client.CreateSecret(ctx, client.CreateSecretRequest{
		KeyPairID: keyPairID,
		Key:       plan.Key.ValueString(),
		Value:     sealed,
		Notes:     plan.Notes.ValueStringPointer(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating secret", err.Error())
		return
	}

	mapSecretToState(secret, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SecretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SecretResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	secret, err := r.client.GetSecret(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading secret", err.Error())
		return
	}

	// value is left as it is in state: the API never reports it.
	mapSecretToState(secret, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state SecretResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// key is required on every update, even when it is unchanged.
	updateReq := client.UpdateSecretRequest{Key: plan.Key.ValueString()}
	// value is compared directly. value_wo never reaches state, so it is sent
	// when its version changes, which includes switching to it from value.
	if (!plan.Value.IsNull() && !plan.Value.Equal(state.Value)) ||
		(plan.Value.IsNull() && !plan.ValueWOVersion.Equal(state.ValueWOVersion)) {
		value, d := secretValue(ctx, req.Config, plan)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		keyPairID, sealed, err := r.client.SealSecretValue(ctx, value)
		if err != nil {
			resp.Diagnostics.AddError("Error encrypting secret value", err.Error())
			return
		}
		updateReq.KeyPairID = &keyPairID
		updateReq.Value = &sealed
	}
	// notes is nullable: removing it from config sends null, which clears it.
	if !plan.Notes.Equal(state.Notes) {
		updateReq.Notes = &client.NullableString{Value: plan.Notes.ValueString(), Null: plan.Notes.IsNull()}
	}

	secret, err := r.client.UpdateSecret(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating secret", err.Error())
		return
	}

	mapSecretToState(secret, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SecretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SecretResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteSecret(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting secret", err.Error())
	}
}

func (r *SecretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// secretValue returns the value to send: value when it is set, otherwise the
// write-only value_wo, which only the configuration carries.
func secretValue(ctx context.Context, config tfsdk.Config, plan SecretResourceModel) (string, diag.Diagnostics) {
	if !plan.Value.IsNull() {
		return plan.Value.ValueString(), nil
	}
	var wo types.String
	d := config.GetAttribute(ctx, path.Root("value_wo"), &wo)
	if d.HasError() {
		return "", d
	}
	if wo.IsNull() || wo.IsUnknown() {
		d.AddAttributeError(path.Root("value_wo"), "Missing secret value",
			"Set value, or value_wo with value_wo_version.")
	}
	return wo.ValueString(), d
}

func mapSecretToState(s *client.SecretData, state *SecretResourceModel) {
	state.ID = types.StringValue(s.ID)
	state.Key = types.StringValue(s.Attributes.Key)
	state.Notes = types.StringPointerValue(s.Attributes.Notes)
	state.CreatedAt = types.StringPointerValue(s.Attributes.CreatedAt)
	state.UpdatedAt = types.StringPointerValue(s.Attributes.UpdatedAt)
}
