package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ datasource.DataSource = &CacheTypesDataSource{}

type CacheTypesDataSource struct {
	client *client.Client
}

type CacheTypesDataSourceModel struct {
	ID    types.String         `tfsdk:"id"`
	Types []CacheTypeItemModel `tfsdk:"types"`
}

type CacheTypeItemModel struct {
	Type                types.String         `tfsdk:"type"`
	Label               types.String         `tfsdk:"label"`
	Regions             []types.String       `tfsdk:"regions"`
	SupportsAutoUpgrade types.Bool           `tfsdk:"supports_auto_upgrade"`
	Sizes               []CacheSizeItemModel `tfsdk:"sizes"`
}

type CacheSizeItemModel struct {
	Value types.String `tfsdk:"value"`
	Label types.String `tfsdk:"label"`
}

func NewCacheTypesDataSource() datasource.DataSource {
	return &CacheTypesDataSource{}
}

func (d *CacheTypesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_cache_types"
}

func (d *CacheTypesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists available cache types and their sizes.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Placeholder identifier.",
			},
			"types": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Available cache types.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Computed:    true,
							Description: "Type identifier (e.g. upstash_redis, laravel_valkey).",
						},
						"label": schema.StringAttribute{
							Computed:    true,
							Description: "Human-friendly label.",
						},
						"regions": schema.ListAttribute{
							Computed:    true,
							ElementType: types.StringType,
							Description: "Regions where this cache type is available.",
						},
						"supports_auto_upgrade": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether this type supports automatic upgrades.",
						},
						"sizes": schema.ListNestedAttribute{
							Computed:    true,
							Description: "Available sizes for this cache type.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"value": schema.StringAttribute{
										Computed:    true,
										Description: "Size value (e.g. 250mb).",
									},
									"label": schema.StringAttribute{
										Computed:    true,
										Description: "Human-friendly label.",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *CacheTypesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *CacheTypesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	cacheTypes, err := d.client.ListCacheTypes(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read cache types", err.Error())
		return
	}

	state := CacheTypesDataSourceModel{
		ID: types.StringValue("cache_types"),
	}

	for _, ct := range cacheTypes {
		item := CacheTypeItemModel{
			Type:                types.StringValue(ct.Type),
			Label:               types.StringValue(ct.Label),
			SupportsAutoUpgrade: types.BoolValue(ct.SupportsAutoUpgrade),
		}
		for _, region := range ct.Regions {
			item.Regions = append(item.Regions, types.StringValue(region))
		}
		for _, s := range ct.Sizes {
			item.Sizes = append(item.Sizes, CacheSizeItemModel{
				Value: types.StringValue(s.Value),
				Label: types.StringValue(s.Label),
			})
		}
		state.Types = append(state.Types, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
