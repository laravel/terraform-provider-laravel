package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &RegionsDataSource{}

type RegionsDataSource struct {
	dataSourceWithClient
}

type RegionsDataSourceModel struct {
	ID      types.String      `tfsdk:"id"`
	Regions []RegionItemModel `tfsdk:"regions"`
}

type RegionItemModel struct {
	Name  types.String `tfsdk:"name"`
	Label types.String `tfsdk:"label"`
	Flag  types.String `tfsdk:"flag"`
}

func NewRegionsDataSource() datasource.DataSource {
	return &RegionsDataSource{}
}

func (d *RegionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_regions"
}

func (d *RegionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists available Laravel Cloud regions.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Placeholder identifier.",
			},
			"regions": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Available cloud regions.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "Region identifier (e.g. us-east-1).",
						},
						"label": schema.StringAttribute{
							Computed:    true,
							Description: "Human-readable region label.",
						},
						"flag": schema.StringAttribute{
							Computed:    true,
							Description: "Country identifier for the region's flag, e.g. `us` or `germany`. Not an emoji.",
						},
					},
				},
			},
		},
	}
}

func (d *RegionsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	regions, err := d.client.ListRegions(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read regions", err.Error())
		return
	}

	state := RegionsDataSourceModel{
		ID: types.StringValue("regions"),
		// An empty result must be an empty list, not a null one: a nil
		// slice decodes to null and makes length()/for_each error.
		Regions: []RegionItemModel{},
	}

	for _, r := range regions {
		state.Regions = append(state.Regions, RegionItemModel{
			Name:  types.StringValue(r.Name),
			Label: types.StringValue(r.Label),
			Flag:  types.StringValue(r.Flag),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
