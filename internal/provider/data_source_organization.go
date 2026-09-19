package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &OrganizationDataSource{}

type OrganizationDataSource struct {
	dataSourceWithClient
}

type OrganizationDataSourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	Slug types.String `tfsdk:"slug"`
}

func NewOrganizationDataSource() datasource.DataSource {
	return &OrganizationDataSource{}
}

func (d *OrganizationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_organization"
}

func (d *OrganizationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the current organization.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Organization ID.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Organization name.",
			},
			"slug": schema.StringAttribute{
				Computed:    true,
				Description: "Organization slug.",
			},
		},
	}
}

func (d *OrganizationDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	org, err := d.client.GetOrganization(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read organization", err.Error())
		return
	}

	state := OrganizationDataSourceModel{
		ID:   types.StringValue(org.ID),
		Name: types.StringValue(org.Attributes.Name),
		Slug: types.StringValue(org.Attributes.Slug),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
