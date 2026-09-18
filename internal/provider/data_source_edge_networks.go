package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ datasource.DataSource = &EdgeNetworksDataSource{}

type EdgeNetworksDataSource struct {
	client *client.Client
}

type EdgeNetworksDataSourceModel struct {
	ID           types.String           `tfsdk:"id"`
	EdgeNetworks []EdgeNetworkItemModel `tfsdk:"edge_networks"`
}

type EdgeNetworkItemModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Domain      types.String `tfsdk:"domain"`
	TenancyType types.String `tfsdk:"tenancy_type"`
	Status      types.String `tfsdk:"status"`
	CreatedAt   types.String `tfsdk:"created_at"`
}

func NewEdgeNetworksDataSource() datasource.DataSource {
	return &EdgeNetworksDataSource{}
}

func (d *EdgeNetworksDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_edge_networks"
}

func (d *EdgeNetworksDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the edge networks available to the organization. An edge network is a " +
			"CDN zone; it carries no IP addresses. For egress IP ranges, use the " +
			"laravel_cloud_ip_addresses data source instead.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier for this data source.",
			},
			"edge_networks": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Available edge networks.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Edge network ID.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "Edge network name.",
						},
						"domain": schema.StringAttribute{
							Computed:    true,
							Description: "Domain served by this edge network.",
						},
						"tenancy_type": schema.StringAttribute{
							Computed:    true,
							Description: "Tenancy type (shared, dedicated).",
						},
						"status": schema.StringAttribute{
							Computed: true,
							Description: "Edge network status (requesting, creating, available, " +
								"deleting, deleted, unknown).",
						},
						"created_at": schema.StringAttribute{
							Computed:    true,
							Description: "When the edge network was created.",
						},
					},
				},
			},
		},
	}
}

func (d *EdgeNetworksDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *EdgeNetworksDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	networks, err := d.client.ListEdgeNetworks(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read edge networks", err.Error())
		return
	}

	state := EdgeNetworksDataSourceModel{
		ID: types.StringValue("edge_networks"),
	}
	for _, n := range networks {
		state.EdgeNetworks = append(state.EdgeNetworks, EdgeNetworkItemModel{
			ID:          types.StringValue(n.ID),
			Name:        types.StringValue(n.Attributes.Name),
			Domain:      types.StringValue(n.Attributes.Domain),
			TenancyType: types.StringValue(n.Attributes.TenancyType),
			Status:      types.StringValue(n.Attributes.Status),
			CreatedAt:   types.StringPointerValue(n.Attributes.CreatedAt),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
