package provider

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                = &EnvironmentSecretResource{}
	_ resource.ResourceWithImportState = &EnvironmentSecretResource{}
)

// EnvironmentSecretResource attaches one organization secret to one
// environment. The attachment has no identity of its own in the API, so the
// resource id is "<environment_id>/<secret_id>".
type EnvironmentSecretResource struct {
	client *client.Client
}

type EnvironmentSecretResourceModel struct {
	ID            types.String `tfsdk:"id"`
	EnvironmentID types.String `tfsdk:"environment_id"`
	SecretID      types.String `tfsdk:"secret_id"`
}

func NewEnvironmentSecretResource() resource.Resource {
	return &EnvironmentSecretResource{}
}

func (r *EnvironmentSecretResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_environment_secret"
}

func (r *EnvironmentSecretResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Attaches an organization secret (laravel_cloud_secret) to an environment. " +
			"Destroying this detaches the secret; the secret itself is kept.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "\"<environment_id>/<secret_id>\".",
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
			"secret_id": schema.StringAttribute{
				Required:    true,
				Description: "Secret ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *EnvironmentSecretResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EnvironmentSecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EnvironmentSecretResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envID, secretID := plan.EnvironmentID.ValueString(), plan.SecretID.ValueString()
	if err := r.client.AttachEnvironmentSecrets(ctx, envID, []string{secretID}); err != nil {
		resp.Diagnostics.AddError("Error attaching secret to environment", err.Error())
		return
	}

	plan.ID = types.StringValue(envID + "/" + secretID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EnvironmentSecretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EnvironmentSecretResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ids, err := r.client.ListEnvironmentSecretIDs(ctx, state.EnvironmentID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading environment secrets", err.Error())
		return
	}
	// Detached outside Terraform (or the secret was deleted): plan to attach
	// it again.
	if !slices.Contains(ids, state.SecretID.ValueString()) {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never called: both attributes force replacement.
func (r *EnvironmentSecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan EnvironmentSecretResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EnvironmentSecretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EnvironmentSecretResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DetachEnvironmentSecret(ctx, state.EnvironmentID.ValueString(), state.SecretID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error detaching secret from environment", err.Error())
	}
}

func (r *EnvironmentSecretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	envID, secretID, ok := strings.Cut(req.ID, "/")
	if !ok || envID == "" || secretID == "" {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected \"<environment_id>/<secret_id>\", got %q.", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), envID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("secret_id"), secretID)...)
}
