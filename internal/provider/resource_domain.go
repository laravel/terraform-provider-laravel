package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                = &DomainResource{}
	_ resource.ResourceWithImportState = &DomainResource{}
)

type DomainResource struct {
	client *client.Client
}

type DomainResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	EnvironmentID      types.String `tfsdk:"environment_id"`
	Name               types.String `tfsdk:"name"`
	DomainType         types.String `tfsdk:"type"`
	HostnameStatus     types.String `tfsdk:"hostname_status"`
	SSLStatus          types.String `tfsdk:"ssl_status"`
	OriginStatus       types.String `tfsdk:"origin_status"`
	WWWRedirect        types.String `tfsdk:"www_redirect"`
	WildcardEnabled    types.Bool   `tfsdk:"wildcard_enabled"`
	VerificationMethod types.String `tfsdk:"verification_method"`
	CloudflareStrategy types.String `tfsdk:"cloudflare_strategy"`
	AllowDowntime      types.Bool   `tfsdk:"allow_downtime"`
	Downtime           types.Bool   `tfsdk:"downtime"`
	Stage              types.String `tfsdk:"stage"`
	ActionRequired     types.String `tfsdk:"action_required"`
	LastVerifiedAt     types.String `tfsdk:"last_verified_at"`
	DNSRecords         types.Object `tfsdk:"dns_records"`
	CreatedAt          types.String `tfsdk:"created_at"`
}

func NewDomainResource() resource.Resource {
	return &DomainResource{}
}

func (r *DomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_domain"
}

func (r *DomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a domain attached to a Laravel Cloud environment.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				Required:    true,
				Description: "Parent environment ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Domain name (3-255 characters).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Computed:    true,
				Description: "Domain type (root, www, wildcard).",
			},
			"hostname_status": schema.StringAttribute{
				Computed:    true,
				Description: "Hostname verification status.",
			},
			"ssl_status": schema.StringAttribute{
				Computed:    true,
				Description: "SSL certificate status.",
			},
			"origin_status": schema.StringAttribute{
				Computed:    true,
				Description: "Origin verification status.",
			},
			"www_redirect": schema.StringAttribute{
				Optional: true,
				Description: "WWW redirect (root_to_www, www_to_root). Create-time only: " +
					"the API's update endpoint accepts verification_method and nothing else, " +
					"so changing this forces a new domain.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"wildcard_enabled": schema.BoolAttribute{
				Optional: true,
				Description: "Enable wildcard subdomain. Create-time only; changing this " +
					"forces a new domain.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"verification_method": schema.StringAttribute{
				Optional:    true,
				Description: "Verification method (pre_verification, real_time). Editable in place via the API's update endpoint.",
			},
			"cloudflare_strategy": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Cloudflare integration strategy (none, dns, dns_proxy). " +
					"Create-time only; changing this forces a new domain.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"allow_downtime": schema.BoolAttribute{
				Optional: true,
				Description: "Whether to allow downtime while attaching the domain. " +
					"Create-time only; changing this forces a new domain.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"stage": schema.StringAttribute{
				Computed:    true,
				Description: "Verification stage (pre_verification, origin).",
			},
			"action_required": schema.StringAttribute{
				Computed: true,
				Description: "What still has to be done before the domain verifies " +
					"(add_txt_records, add_dns_records, failed), or null when nothing is pending.",
			},
			"last_verified_at": schema.StringAttribute{
				Computed:    true,
				Description: "When the domain was last successfully verified.",
			},
			"dns_records": schema.SingleNestedAttribute{
				Computed: true,
				Description: "The DNS records that must exist for this domain to verify and " +
					"serve traffic. Use these to create the records at your DNS provider.",
				Attributes: map[string]schema.Attribute{
					"ssl": schema.ListNestedAttribute{
						Computed:    true,
						Description: "Records required for SSL certificate issuance.",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"type": schema.StringAttribute{
									Computed:    true,
									Description: "Record type (CNAME or TXT).",
								},
								"name": schema.StringAttribute{
									Computed:    true,
									Description: "Record name.",
								},
								"value": schema.StringAttribute{
									Computed:    true,
									Description: "Record value.",
								},
							},
						},
					},
					"pre_verification": schema.StringAttribute{
						Computed:    true,
						Description: "TXT value used for pre-verification.",
					},
					"origin": schema.StringAttribute{
						Computed:    true,
						Description: "Origin address the domain should point at.",
					},
					"origin_cname": schema.StringAttribute{
						Computed:    true,
						Description: "Origin CNAME target.",
					},
					"dcv": schema.StringAttribute{
						Computed:    true,
						Description: "Domain control validation value.",
					},
				},
			},
			"downtime": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether attaching the domain incurs downtime.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *DomainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateDomainRequest{
		Name: plan.Name.ValueString(),
	}
	if !plan.WWWRedirect.IsNull() {
		v := plan.WWWRedirect.ValueString()
		createReq.WWWRedirect = &v
	}
	if !plan.WildcardEnabled.IsNull() {
		v := plan.WildcardEnabled.ValueBool()
		createReq.WildcardEnabled = &v
	}
	if !plan.VerificationMethod.IsNull() {
		v := plan.VerificationMethod.ValueString()
		createReq.VerificationMethod = &v
	}
	if !plan.CloudflareStrategy.IsNull() && !plan.CloudflareStrategy.IsUnknown() {
		v := plan.CloudflareStrategy.ValueString()
		createReq.CloudflareStrategy = &v
	}
	if !plan.AllowDowntime.IsNull() {
		v := plan.AllowDowntime.ValueBool()
		createReq.AllowDowntime = &v
	}

	domain, err := r.client.CreateDomain(ctx, plan.EnvironmentID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating domain", err.Error())
		return
	}

	mapDomainToState(domain, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain, err := r.client.GetDomain(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading domain", err.Error())
		return
	}

	mapDomainToState(domain, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state DomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// verification_method is the entire PATCH body and the API requires it.
	// Sending "" when the user never configured one is a guaranteed 422, and
	// since every other attribute forces replacement there is nothing else an
	// update could be for -- so with no verification method there is no call
	// to make.
	if plan.VerificationMethod.IsNull() || plan.VerificationMethod.IsUnknown() {
		// Carry the plan forward, not the prior state: verification_method is
		// Optional-only, so removing it from config plans a null, and writing
		// the old value back would contradict the plan and fail the apply with
		// "provider produced inconsistent result after apply". The computed
		// attributes are copied across because there is no fresh response.
		plan.ID = state.ID
		plan.EnvironmentID = state.EnvironmentID
		plan.DomainType = state.DomainType
		plan.HostnameStatus = state.HostnameStatus
		plan.SSLStatus = state.SSLStatus
		plan.OriginStatus = state.OriginStatus
		plan.CloudflareStrategy = state.CloudflareStrategy
		plan.Downtime = state.Downtime
		plan.Stage = state.Stage
		plan.ActionRequired = state.ActionRequired
		plan.LastVerifiedAt = state.LastVerifiedAt
		plan.DNSRecords = state.DNSRecords
		plan.CreatedAt = state.CreatedAt
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	updateReq := client.UpdateDomainRequest{
		VerificationMethod: plan.VerificationMethod.ValueString(),
	}

	domain, err := r.client.UpdateDomain(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating domain", err.Error())
		return
	}

	plan.EnvironmentID = state.EnvironmentID
	mapDomainToState(domain, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteDomain(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting domain", err.Error())
	}
}

func (r *DomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mapDomainToState(d *client.DomainData, state *DomainResourceModel) {
	if envID := d.Relationships.Environment.RelatedID(); envID != "" {
		state.EnvironmentID = types.StringValue(envID)
	}
	state.ID = types.StringValue(d.ID)
	state.Name = types.StringValue(d.Attributes.Name)
	state.DomainType = types.StringValue(d.Attributes.DomainType)
	state.HostnameStatus = types.StringValue(d.Attributes.HostnameStatus)
	state.SSLStatus = types.StringValue(d.Attributes.SSLStatus)
	state.OriginStatus = types.StringValue(d.Attributes.OriginStatus)
	state.CloudflareStrategy = types.StringPointerValue(d.Attributes.CloudflareStrategy)
	state.Downtime = types.BoolPointerValue(d.Attributes.Downtime)
	state.Stage = types.StringValue(d.Attributes.Stage)
	state.ActionRequired = types.StringPointerValue(d.Attributes.ActionRequired)
	state.LastVerifiedAt = types.StringPointerValue(d.Attributes.LastVerifiedAt)
	state.DNSRecords = domainDNSRecordsObject(d.Attributes.DNSRecords)
	state.CreatedAt = types.StringPointerValue(d.Attributes.CreatedAt)
	if d.Attributes.Redirect != nil {
		state.WWWRedirect = types.StringValue(*d.Attributes.Redirect)
	}
}

var domainSSLRecordAttrTypes = map[string]attr.Type{
	"type":  types.StringType,
	"name":  types.StringType,
	"value": types.StringType,
}

var domainDNSRecordsAttrTypes = map[string]attr.Type{
	"ssl":              types.ListType{ElemType: types.ObjectType{AttrTypes: domainSSLRecordAttrTypes}},
	"pre_verification": types.StringType,
	"origin":           types.StringType,
	"origin_cname":     types.StringType,
	"dcv":              types.StringType,
}

// domainDNSRecordsObject converts the API's dns_records payload into the
// Terraform object exposed on the resource.
// The Must-free construction is deliberate: these values are built from an API
// response, and a provider that panics on an unexpected payload takes the whole
// Terraform run down with a stack trace instead of reporting drift.
func domainDNSRecordsObject(r client.DomainDNSRecords) types.Object {
	sslType := types.ObjectType{AttrTypes: domainSSLRecordAttrTypes}

	ssl := make([]attr.Value, 0, len(r.SSL))
	for _, rec := range r.SSL {
		obj, diags := types.ObjectValue(domainSSLRecordAttrTypes, map[string]attr.Value{
			"type":  types.StringValue(rec.Type),
			"name":  types.StringPointerValue(rec.Name),
			"value": types.StringPointerValue(rec.Value),
		})
		if diags.HasError() {
			obj = types.ObjectNull(domainSSLRecordAttrTypes)
		}
		ssl = append(ssl, obj)
	}
	sslList, diags := types.ListValue(sslType, ssl)
	if diags.HasError() {
		sslList = types.ListNull(sslType)
	}

	records, diags := types.ObjectValue(domainDNSRecordsAttrTypes, map[string]attr.Value{
		"ssl":              sslList,
		"pre_verification": types.StringValue(r.PreVerification),
		"origin":           types.StringValue(r.Origin),
		"origin_cname":     types.StringValue(r.OriginCNAME),
		"dcv":              types.StringValue(r.DCV),
	})
	if diags.HasError() {
		return types.ObjectNull(domainDNSRecordsAttrTypes)
	}
	return records
}
