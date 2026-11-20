package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ datasource.DataSource = &ResendDomainsDataSource{}

type ResendDomainsDataSource struct {
	client *client.Client
}

type ResendDomainsDataSourceModel struct {
	ID      types.String            `tfsdk:"id"`
	Domains []ResendDomainItemModel `tfsdk:"domains"`
}

type ResendDomainItemModel struct {
	ID             types.String              `tfsdk:"id"`
	Name           types.String              `tfsdk:"name"`
	Region         types.String              `tfsdk:"region"`
	Status         types.String              `tfsdk:"status"`
	Records        []ResendDomainRecordModel `tfsdk:"records"`
	LastVerifiedAt types.String              `tfsdk:"last_verified_at"`
	CreatedAt      types.String              `tfsdk:"created_at"`
}

type ResendDomainRecordModel struct {
	Record   types.String `tfsdk:"record"`
	Name     types.String `tfsdk:"name"`
	Type     types.String `tfsdk:"type"`
	TTL      types.String `tfsdk:"ttl"`
	Status   types.String `tfsdk:"status"`
	Value    types.String `tfsdk:"value"`
	Priority types.Int64  `tfsdk:"priority"`
}

func NewResendDomainsDataSource() datasource.DataSource {
	return &ResendDomainsDataSource{}
}

func (d *ResendDomainsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_resend_domains"
}

func (d *ResendDomainsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the organization's Resend sending domains, with the DNS records each needs. " +
			"The organization must have the Resend integration.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Placeholder identifier.",
			},
			"domains": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Sending domains, sorted by name.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Sending domain ID.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "Domain name.",
						},
						"region": schema.StringAttribute{
							Computed:    true,
							Description: "Resend region the domain sends from.",
						},
						"status": schema.StringAttribute{
							Computed: true,
							Description: "Verification status: `not_started`, `pending`, `verified`, `partially_verified`, " +
								"`partially_failed`, `failed` or `temporary_failure`.",
						},
						"records": schema.ListNestedAttribute{
							Computed:    true,
							Description: "DNS records to add for the domain.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"record": schema.StringAttribute{
										Computed:    true,
										Description: "What the record is for, such as `DKIM` or `SPF`.",
									},
									"name": schema.StringAttribute{
										Computed:    true,
										Description: "Record name, relative to the domain.",
									},
									"type": schema.StringAttribute{
										Computed:    true,
										Description: "Record type, such as `TXT` or `MX`.",
									},
									"ttl": schema.StringAttribute{
										Computed:    true,
										Description: "TTL, such as `Auto`.",
									},
									"status": schema.StringAttribute{
										Computed:    true,
										Description: "Verification status of the record.",
									},
									"value": schema.StringAttribute{
										Computed:    true,
										Description: "Record value.",
									},
									"priority": schema.Int64Attribute{
										Computed:    true,
										Description: "Priority of an MX record; null for other types.",
									},
								},
							},
						},
						"last_verified_at": schema.StringAttribute{
							Computed:    true,
							Description: "When the domain was last verified.",
						},
						"created_at": schema.StringAttribute{
							Computed:    true,
							Description: "When the domain was added.",
						},
					},
				},
			},
		},
	}
}

func (d *ResendDomainsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected data source configure type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *ResendDomainsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	domains, err := d.client.ListResendDomains(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Resend sending domains", err.Error())
		return
	}

	state := ResendDomainsDataSourceModel{
		ID:      types.StringValue("resend_domains"),
		Domains: []ResendDomainItemModel{},
	}
	for _, dom := range domains {
		item := ResendDomainItemModel{
			ID:             types.StringValue(dom.ID),
			Name:           types.StringValue(dom.Attributes.Name),
			Region:         types.StringValue(dom.Attributes.Region),
			Status:         types.StringValue(dom.Attributes.Status),
			Records:        []ResendDomainRecordModel{},
			LastVerifiedAt: types.StringPointerValue(dom.Attributes.LastVerifiedAt),
			CreatedAt:      types.StringPointerValue(dom.Attributes.CreatedAt),
		}
		for _, r := range dom.Attributes.Records {
			item.Records = append(item.Records, ResendDomainRecordModel{
				Record:   types.StringValue(r.Record),
				Name:     types.StringValue(r.Name),
				Type:     types.StringValue(r.Type),
				TTL:      types.StringValue(r.TTL),
				Status:   types.StringValue(r.Status),
				Value:    types.StringValue(r.Value),
				Priority: types.Int64PointerValue(r.Priority),
			})
		}
		state.Domains = append(state.Domains, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
