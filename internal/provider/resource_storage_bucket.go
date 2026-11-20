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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                   = &StorageBucketResource{}
	_ resource.ResourceWithImportState    = &StorageBucketResource{}
	_ resource.ResourceWithValidateConfig = &StorageBucketResource{}
)

type StorageBucketResource struct {
	client *client.Client
}

type StorageBucketResourceModel struct {
	ID             types.String       `tfsdk:"id"`
	Name           types.String       `tfsdk:"name"`
	Visibility     types.String       `tfsdk:"visibility"`
	Jurisdiction   types.String       `tfsdk:"jurisdiction"`
	LocationHint   types.String       `tfsdk:"location_hint"`
	KeyName        types.String       `tfsdk:"key_name"`
	KeyPermission  types.String       `tfsdk:"key_permission"`
	AllowedOrigins types.List         `tfsdk:"allowed_origins"`
	CorsSettings   *CorsSettingsModel `tfsdk:"cors_settings"`
	Endpoint       types.String       `tfsdk:"endpoint"`
	URL            types.String       `tfsdk:"url"`
	Status         types.String       `tfsdk:"status"`
	CreatedAt      types.String       `tfsdk:"created_at"`
}

type CorsSettingsModel struct {
	AllowedOrigins types.List  `tfsdk:"allowed_origins"`
	AllowedMethods types.List  `tfsdk:"allowed_methods"`
	AllowedHeaders types.List  `tfsdk:"allowed_headers"`
	ExposeHeaders  types.List  `tfsdk:"expose_headers"`
	MaxAgeSeconds  types.Int64 `tfsdk:"max_age_seconds"`
}

func NewStorageBucketResource() resource.Resource {
	return &StorageBucketResource{}
}

func (r *StorageBucketResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_storage_bucket"
}

func (r *StorageBucketResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Laravel Cloud object storage bucket.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Bucket name (2-40 characters, lowercase alphanumeric with hyphens/underscores).",
			},
			"visibility": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Bucket visibility: 'private' or 'public'. Defaults to 'private'.",
				Default:     stringdefault.StaticString("private"),
			},
			"jurisdiction": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Bucket jurisdiction: 'default', 'eu' or 'us'. Defaults to 'default'.",
				Default:     stringdefault.StaticString("default"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"location_hint": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Where to place the bucket: automatic, wnam (western North America), enam (eastern North America), " +
					"weur (western Europe), eeur (eastern Europe), apac (Asia-Pacific) or oc (Oceania). The platform " +
					"defaults to automatic, the only value allowed outside the default jurisdiction. Changing it forces a new bucket.",
				Validators: []validator.String{
					stringvalidator.OneOf("automatic", "wnam", "enam", "weur", "eeur", "apac", "oc"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"key_name": schema.StringAttribute{
				Required:    true,
				Description: "Name for the initial access key (2-40 characters). Changing it renames the key in place.",
			},
			"key_permission": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Permission for the initial key: 'read_write' or 'read_only'. Defaults to 'read_write'. " +
					"A key's permission cannot change, so changing it replaces the initial key with a new one, " +
					"which has new credentials; the bucket and its contents are kept.",
				Default: stringdefault.StaticString("read_write"),
				Validators: []validator.String{
					stringvalidator.OneOf("read_write", "read_only"),
				},
			},
			"allowed_origins": schema.ListAttribute{
				Optional:           true,
				Description:        "Allowed CORS origins.",
				ElementType:        types.StringType,
				DeprecationMessage: "allowed_origins is an alias for cors_settings.allowed_origins -- writing either updates both. It was announced for removal from the Laravel Cloud API on May 17, 2026 but is still served; use cors_settings.allowed_origins instead.",
			},
			"cors_settings": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "CORS configuration for the bucket.",
				Attributes: map[string]schema.Attribute{
					"allowed_origins": schema.ListAttribute{
						Optional:    true,
						Description: "Allowed CORS origins.",
						ElementType: types.StringType,
					},
					"allowed_methods": schema.ListAttribute{
						Optional:    true,
						Description: "Allowed CORS methods.",
						ElementType: types.StringType,
					},
					"allowed_headers": schema.ListAttribute{
						Optional:    true,
						Description: "Allowed CORS request headers.",
						ElementType: types.StringType,
					},
					"expose_headers": schema.ListAttribute{
						Optional:    true,
						Description: "CORS response headers exposed to the browser.",
						ElementType: types.StringType,
					},
					"max_age_seconds": schema.Int64Attribute{
						Optional:    true,
						Description: "How long (in seconds) the browser may cache the CORS preflight response.",
					},
				},
			},
			"endpoint": schema.StringAttribute{
				Computed:    true,
				Description: "Bucket S3 endpoint URL.",
			},
			"url": schema.StringAttribute{
				Computed:    true,
				Description: "Bucket public URL.",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Bucket status.",
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *StorageBucketResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *StorageBucketResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan StorageBucketResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateStorageBucketRequest{
		Name:          plan.Name.ValueString(),
		Visibility:    plan.Visibility.ValueString(),
		Jurisdiction:  plan.Jurisdiction.ValueString(),
		KeyName:       plan.KeyName.ValueString(),
		KeyPermission: plan.KeyPermission.ValueString(),
	}
	if isSet(plan.LocationHint) {
		createReq.LocationHint = plan.LocationHint.ValueStringPointer()
	}
	if !plan.AllowedOrigins.IsNull() {
		var origins []string
		resp.Diagnostics.Append(plan.AllowedOrigins.ElementsAs(ctx, &origins, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.AllowedOrigins = origins //nolint:staticcheck // deprecated attribute kept for backward compatibility until removal
	}
	if plan.CorsSettings != nil {
		cors, d := buildCorsSettings(ctx, plan.CorsSettings)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.CorsSettings = cors
	}

	bucket, err := r.client.CreateStorageBucket(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating storage bucket", err.Error())
		return
	}

	mapStorageBucketToState(ctx, bucket, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *StorageBucketResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state StorageBucketResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket, err := r.client.GetStorageBucket(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading storage bucket", err.Error())
		return
	}

	mapStorageBucketToState(ctx, bucket, &state, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *StorageBucketResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan StorageBucketResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state StorageBucketResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := client.UpdateStorageBucketRequest{}
	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		updateReq.Name = &v
	}
	if !plan.Visibility.Equal(state.Visibility) {
		v := plan.Visibility.ValueString()
		updateReq.Visibility = &v
	}
	// An emptied or removed list is a change like any other, so both of the
	// CORS fields below distinguish "unchanged" from "cleared". Sending
	// nothing for a cleared value left the rules live on the bucket while
	// state recorded them as gone, with no way to ever take them off.
	if !plan.AllowedOrigins.Equal(state.AllowedOrigins) {
		origins := []string{}
		if !plan.AllowedOrigins.IsNull() {
			resp.Diagnostics.Append(plan.AllowedOrigins.ElementsAs(ctx, &origins, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
		updateReq.AllowedOrigins = &origins //nolint:staticcheck // deprecated attribute kept for backward compatibility until removal
	}
	switch {
	case plan.CorsSettings != nil:
		cors, d := buildCorsSettings(ctx, plan.CorsSettings)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.CorsSettings = cors
	case state.CorsSettings != nil:
		// The block was removed. There is no way to take CORS off a bucket
		// outright: the API ignores both cors_settings: null and {}, and
		// rejects an empty allowed_methods with "At least one method must be
		// provided". Emptying the origin list is the one state it accepts that
		// means "allow nothing", so that is what removing the block does.
		updateReq.CorsSettings = &client.CorsSettings{AllowedOrigins: &[]string{}}
	}

	// A change to only the initial key leaves nothing for the bucket itself.
	var bucket *client.StorageBucketData
	var err error
	if updateReq == (client.UpdateStorageBucketRequest{}) {
		bucket, err = r.client.GetStorageBucket(ctx, state.ID.ValueString())
	} else {
		bucket, err = r.client.UpdateStorageBucket(ctx, state.ID.ValueString(), updateReq)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error updating storage bucket", err.Error())
		return
	}

	if err := r.updateInitialKey(ctx, state, plan); err != nil {
		resp.Diagnostics.AddError("Error updating the storage bucket's initial key", err.Error())
		return
	}

	// location_hint cannot change in place, so the planned value stands. After an
	// upgrade it is planned as its null prior state, and adopting the reported
	// value here would contradict the plan; the next refresh records it.
	plannedLocationHint := plan.LocationHint
	mapStorageBucketToState(ctx, bucket, &plan, &resp.Diagnostics)
	plan.LocationHint = plannedLocationHint
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *StorageBucketResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state StorageBucketResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API refuses to delete a bucket that still has access keys, and
	// every bucket gets one on create. Keys managed as separate resources are
	// destroyed before the bucket, so what remains here is the initial key and
	// any made outside Terraform; destroying the bucket removes them anyway.
	id := state.ID.ValueString()
	keys, err := r.client.ListStorageBucketKeys(ctx, id)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error listing storage bucket keys", err.Error())
		return
	}
	for _, k := range keys {
		if err := r.client.DeleteStorageBucketKey(ctx, k.ID); err != nil && !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Error deleting storage bucket key "+k.Attributes.Name, err.Error())
			return
		}
	}

	if err := r.client.DeleteStorageBucket(ctx, id); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting storage bucket", err.Error())
	}
}

// updateInitialKey applies a change to key_name or key_permission to the key
// the bucket was created with, which the API identifies only by name.
//
// A rename is done in place. A key's permission cannot change, so a new
// permission means a new key: it is created before the old one is deleted, so
// a failure never leaves the bucket without its key.
//
// A null prior value means the attribute was never recorded, as after an
// import, so the configured value is adopted without touching any key.
func (r *StorageBucketResource) updateInitialKey(ctx context.Context, state, plan StorageBucketResourceModel) error {
	if state.KeyName.IsNull() || state.KeyPermission.IsNull() {
		return nil
	}
	renamed := !plan.KeyName.Equal(state.KeyName)
	repermed := !plan.KeyPermission.Equal(state.KeyPermission)
	if !renamed && !repermed {
		return nil
	}

	bucketID := state.ID.ValueString()
	keys, err := r.client.ListStorageBucketKeys(ctx, bucketID)
	if err != nil {
		return err
	}
	var oldKeyID string
	for _, k := range keys {
		if k.Attributes.Name == state.KeyName.ValueString() {
			oldKeyID = k.ID
			break
		}
	}
	if oldKeyID == "" {
		return fmt.Errorf("the bucket has no key named %q to change; it may have been renamed or deleted outside Terraform", state.KeyName.ValueString())
	}

	if !repermed {
		_, err := r.client.UpdateStorageBucketKey(ctx, oldKeyID, client.UpdateStorageBucketKeyRequest{Name: plan.KeyName.ValueString()})
		return err
	}

	if _, err := r.client.CreateStorageBucketKey(ctx, bucketID, client.CreateStorageBucketKeyRequest{
		Name:       plan.KeyName.ValueString(),
		Permission: plan.KeyPermission.ValueString(),
	}); err != nil {
		return err
	}
	return r.client.DeleteStorageBucketKey(ctx, oldKeyID)
}

func (r *StorageBucketResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// buildCorsSettings converts the Terraform nested cors_settings model into the
// client request payload, translating each TF list into a []string and the
// optional max_age_seconds into a *int.
func buildCorsSettings(ctx context.Context, m *CorsSettingsModel) (*client.CorsSettings, diag.Diagnostics) {
	var diags diag.Diagnostics
	cors := &client.CorsSettings{}

	for _, f := range []struct {
		list types.List
		dst  **[]string
	}{
		{m.AllowedOrigins, &cors.AllowedOrigins},
		{m.AllowedMethods, &cors.AllowedMethods},
		{m.AllowedHeaders, &cors.AllowedHeaders},
		{m.ExposeHeaders, &cors.ExposeHeaders},
	} {
		if f.list.IsNull() || f.list.IsUnknown() {
			continue
		}
		// Non-nil even when empty: the API merges this body into the bucket's
		// current rules, so an emptied list only takes effect as an explicit [].
		values := []string{}
		diags.Append(f.list.ElementsAs(ctx, &values, false)...)
		*f.dst = &values
	}

	if !m.MaxAgeSeconds.IsNull() && !m.MaxAgeSeconds.IsUnknown() {
		v := int(m.MaxAgeSeconds.ValueInt64())
		cors.MaxAgeSeconds = &v
	}

	return cors, diags
}

func mapStorageBucketToState(ctx context.Context, b *client.StorageBucketData, state *StorageBucketResourceModel, diags *diag.Diagnostics) {
	state.ID = types.StringValue(b.ID)
	state.Name = types.StringValue(b.Attributes.Name)
	state.Visibility = types.StringValue(b.Attributes.Visibility)
	state.Jurisdiction = types.StringValue(b.Attributes.Jurisdiction)
	state.LocationHint = types.StringPointerValue(b.Attributes.LocationHint)
	state.Endpoint = types.StringPointerValue(b.Attributes.Endpoint)
	state.URL = types.StringPointerValue(b.Attributes.URL)
	state.Status = types.StringValue(b.Attributes.Status)
	state.CreatedAt = types.StringPointerValue(b.Attributes.CreatedAt)
	// allowed_origins is Optional but not Computed, and the API reports it as a
	// live alias of cors_settings.allowed_origins -- it comes back populated
	// even for a configuration that only ever set cors_settings. Adopting it
	// unconditionally then wrote a value into an attribute the plan said was
	// null, which Terraform rejects outright as "Provider produced inconsistent
	// result after apply". It is refreshed only for configurations that
	// actually use the deprecated attribute.
	if b.Attributes.AllowedOrigins != nil && !state.AllowedOrigins.IsNull() {
		list, d := types.ListValueFrom(ctx, types.StringType, b.Attributes.AllowedOrigins)
		diags.Append(d...)
		state.AllowedOrigins = list
	}

	// cors_settings is intentionally not read back from the API response. The
	// API returns it as opaque, unstable JSON (StorageBucketAttributes.CorsSettings
	// is json.RawMessage), so round-tripping it into the typed nested object would
	// risk spurious diffs. We instead preserve the configured value as-is,
	// mirroring how this provider handles other write-mostly fields.
}

// ValidateConfig moves the API's one CORS invariant from a runtime 422 to a
// plan-time error: cors_settings.allowed_methods must name at least one method,
// so an explicitly empty list can never apply.
//
// Emptying allowed_origins is the supported way to turn CORS off; emptying the
// method list is simply rejected.
func (r *StorageBucketResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config StorageBucketResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config.CorsSettings == nil {
		return
	}

	methods := config.CorsSettings.AllowedMethods
	if methods.IsNull() || methods.IsUnknown() || len(methods.Elements()) > 0 {
		return
	}
	resp.Diagnostics.AddAttributeError(
		path.Root("cors_settings").AtName("allowed_methods"),
		"cors_settings.allowed_methods cannot be empty",
		"The Laravel Cloud API rejects an empty allowed_methods with \"At least one method "+
			"must be provided\". Remove the attribute to leave the bucket's methods unchanged, "+
			"or set allowed_origins = [] to stop the bucket accepting any cross-origin request.",
	)
}
