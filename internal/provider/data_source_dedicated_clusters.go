package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ datasource.DataSource = &DedicatedClustersDataSource{}

type DedicatedClustersDataSource struct {
	client *client.Client
}

type DedicatedClustersDataSourceModel struct {
	ID       types.String                `tfsdk:"id"`
	Clusters []DedicatedClusterItemModel `tfsdk:"clusters"`
}

type DedicatedClusterItemModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Region      types.String `tfsdk:"region"`
	ClusterType types.String `tfsdk:"type"`
	TenancyType types.String `tfsdk:"tenancy_type"`
	Status      types.String `tfsdk:"status"`
	CreatedAt   types.String `tfsdk:"created_at"`
}

func NewDedicatedClustersDataSource() datasource.DataSource {
	return &DedicatedClustersDataSource{}
}

func (d *DedicatedClustersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_dedicated_clusters"
}

func (d *DedicatedClustersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists dedicated clusters available to your organization.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Placeholder identifier.",
			},
			"clusters": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Available dedicated clusters.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Cluster ID.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "Cluster name.",
						},
						"region": schema.StringAttribute{
							Computed:    true,
							Description: "Cluster region.",
						},
						"type": schema.StringAttribute{
							Computed:    true,
							Description: "Cluster type.",
						},
						"tenancy_type": schema.StringAttribute{
							Computed:    true,
							Description: "Tenancy type (shared, dedicated).",
						},
						"status": schema.StringAttribute{
							Computed:    true,
							Description: "Cluster status.",
						},
						"created_at": schema.StringAttribute{
							Computed:    true,
							Description: "Creation timestamp.",
						},
					},
				},
			},
		},
	}
}

func (d *DedicatedClustersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *DedicatedClustersDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	clusters, err := d.client.ListDedicatedClusters(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read dedicated clusters", err.Error())
		return
	}

	state := DedicatedClustersDataSourceModel{
		ID: types.StringValue("dedicated_clusters"),
	}

	for _, c := range clusters {
		state.Clusters = append(state.Clusters, DedicatedClusterItemModel{
			ID:          types.StringValue(c.ID),
			Name:        types.StringValue(c.Attributes.Name),
			Region:      types.StringValue(c.Attributes.Region),
			ClusterType: types.StringValue(c.Attributes.ClusterType),
			TenancyType: types.StringValue(c.Attributes.TenancyType),
			Status:      types.StringValue(c.Attributes.Status),
			CreatedAt:   types.StringPointerValue(c.Attributes.CreatedAt),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
