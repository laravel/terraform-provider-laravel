package provider

import (
	"context"
	"fmt"

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
				Optional:    true,
				Description: "WWW redirect (root_to_www, www_to_root).",
			},
			"wildcard_enabled": schema.BoolAttribute{
				Optional:    true,
				Description: "Enable wildcard subdomain.",
			},
			"verification_method": schema.StringAttribute{
				Optional:    true,
				Description: "Verification method (pre_verification, real_time). Editable in place via the API's update endpoint.",
			},
			"cloudflare_strategy": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Cloudflare integration strategy (none, dns, dns_proxy). Set at creation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"allow_downtime": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether to allow downtime while attaching the domain (create-time only).",
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
	state.ID = types.StringValue(d.ID)
	state.Name = types.StringValue(d.Attributes.Name)
	state.DomainType = types.StringValue(d.Attributes.DomainType)
	state.HostnameStatus = types.StringValue(d.Attributes.HostnameStatus)
	state.SSLStatus = types.StringValue(d.Attributes.SSLStatus)
	state.OriginStatus = types.StringValue(d.Attributes.OriginStatus)
	state.CloudflareStrategy = types.StringPointerValue(d.Attributes.CloudflareStrategy)
	state.Downtime = types.BoolPointerValue(d.Attributes.Downtime)
	state.CreatedAt = types.StringPointerValue(d.Attributes.CreatedAt)
	if d.Attributes.Redirect != nil {
		state.WWWRedirect = types.StringValue(*d.Attributes.Redirect)
	}
}
