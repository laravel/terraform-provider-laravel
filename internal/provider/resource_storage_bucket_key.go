package provider

import (
	"context"
	"fmt"
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
	_ resource.Resource                = &StorageBucketKeyResource{}
	_ resource.ResourceWithImportState = &StorageBucketKeyResource{}
)

type StorageBucketKeyResource struct {
	resourceWithClient
}

type StorageBucketKeyResourceModel struct {
	ID              types.String `tfsdk:"id"`
	BucketID        types.String `tfsdk:"bucket_id"`
	Name            types.String `tfsdk:"name"`
	Permission      types.String `tfsdk:"permission"`
	AccessKeyID     types.String `tfsdk:"access_key_id"`
	AccessKeySecret types.String `tfsdk:"access_key_secret"`
	CreatedAt       types.String `tfsdk:"created_at"`
}

func NewStorageBucketKeyResource() resource.Resource {
	return &StorageBucketKeyResource{}
}

func (r *StorageBucketKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_storage_bucket_key"
}

func (r *StorageBucketKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an access key for a Laravel Cloud storage bucket.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"bucket_id": schema.StringAttribute{
				Required:    true,
				Description: "Parent storage bucket ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Key name.",
			},
			"permission": schema.StringAttribute{
				Required:    true,
				Description: "Key permission: read_write or read_only.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"access_key_id": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "S3-compatible access key ID.",
			},
			"access_key_secret": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "S3-compatible secret access key.",
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *StorageBucketKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan StorageBucketKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateStorageBucketKeyRequest{
		Name:       plan.Name.ValueString(),
		Permission: plan.Permission.ValueString(),
	}

	key, err := r.client.CreateStorageBucketKey(ctx, plan.BucketID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating storage bucket key", err.Error())
		return
	}

	mapStorageBucketKeyToState(key, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *StorageBucketKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state StorageBucketKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, err := r.client.GetStorageBucketKey(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading storage bucket key", err.Error())
		return
	}

	mapStorageBucketKeyToState(key, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *StorageBucketKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan StorageBucketKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state StorageBucketKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := client.UpdateStorageBucketKeyRequest{
		Name: plan.Name.ValueString(),
	}

	key, err := r.client.UpdateStorageBucketKey(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating storage bucket key", err.Error())
		return
	}

	mapStorageBucketKeyToState(key, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *StorageBucketKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state StorageBucketKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteStorageBucketKey(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting storage bucket key", err.Error())
	}
}

func (r *StorageBucketKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Read now uses the flat GET /bucket-keys/{id} and so no longer needs
	// bucket_id -- but bucket_id is still Required (Create posts to the bucket)
	// and forces replacement, and nothing the client decodes carries the owning
	// bucket, so a bare-id import would leave it null and plan a
	// destroy/recreate. Import as "<bucket_id>:<key_id>", matching
	// laravel_cloud_database. Requesting ?include=filesystem and decoding
	// relationships.filesystem.data.id would allow a bare-id import later.
	bucketID, id, found := strings.Cut(req.ID, ":")
	if !found || bucketID == "" || id == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"bucket_id:key_id\", got %q.", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bucket_id"), bucketID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func mapStorageBucketKeyToState(key *client.StorageBucketKeyData, state *StorageBucketKeyResourceModel) {
	state.ID = types.StringValue(key.ID)
	state.Name = types.StringValue(key.Attributes.Name)
	state.Permission = types.StringValue(key.Attributes.Permission)
	state.AccessKeyID = preserveCredential(state.AccessKeyID, key.Attributes.AccessKeyID)
	state.AccessKeySecret = preserveCredential(state.AccessKeySecret, key.Attributes.AccessKeySecret)
	state.CreatedAt = types.StringPointerValue(key.Attributes.CreatedAt)
}

// preserveCredential keeps a credential that is already in state when the API
// omits it. Both key credentials are nullable: they are returned when the key is
// created but a later read may answer null, and overwriting the stored value with
// "" would silently break every reference to it. An unknown prior value -- a
// fresh create whose response carried no credential -- collapses to null so the
// apply never returns an unknown value.
func preserveCredential(current types.String, returned *string) types.String {
	if returned != nil {
		return types.StringValue(*returned)
	}
	if current.IsUnknown() {
		return types.StringNull()
	}
	return current
}
