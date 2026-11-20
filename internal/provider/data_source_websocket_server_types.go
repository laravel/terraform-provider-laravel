package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ datasource.DataSource = &WebsocketServerTypesDataSource{}

type WebsocketServerTypesDataSource struct {
	client *client.Client
}

type WebsocketServerTypesDataSourceModel struct {
	ID    types.String                   `tfsdk:"id"`
	Types []WebsocketServerTypeItemModel `tfsdk:"types"`
}

type WebsocketServerTypeItemModel struct {
	Type           types.String                       `tfsdk:"type"`
	Label          types.String                       `tfsdk:"label"`
	Available      types.Bool                         `tfsdk:"available"`
	Regions        []types.String                     `tfsdk:"regions"`
	MaxConnections []WebsocketConnectionTierItemModel `tfsdk:"max_connections"`
}

type WebsocketConnectionTierItemModel struct {
	Value     types.Int64  `tfsdk:"value"`
	Label     types.String `tfsdk:"label"`
	Available types.Bool   `tfsdk:"available"`
}

func NewWebsocketServerTypesDataSource() datasource.DataSource {
	return &WebsocketServerTypesDataSource{}
}

func (d *WebsocketServerTypesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_websocket_server_types"
}

func (d *WebsocketServerTypesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the WebSocket server types, with the regions and max connection tiers each supports. " +
			"Use a tier's value as max_connections on laravel_cloud_websocket_server.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Placeholder identifier.",
			},
			"types": schema.ListNestedAttribute{
				Computed:    true,
				Description: "WebSocket server types.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Computed:    true,
							Description: "Type identifier, such as `reverb`.",
						},
						"label": schema.StringAttribute{
							Computed:    true,
							Description: "Display name.",
						},
						"available": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether the organization can create servers of this type.",
						},
						"regions": schema.ListAttribute{
							Computed:    true,
							ElementType: types.StringType,
							Description: "Regions the type is offered in.",
						},
						"max_connections": schema.ListNestedAttribute{
							Computed:    true,
							Description: "Max connection tiers, smallest first.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"value": schema.Int64Attribute{
										Computed:    true,
										Description: "Value to pass as max_connections.",
									},
									"label": schema.StringAttribute{
										Computed:    true,
										Description: "Display label.",
									},
									"available": schema.BoolAttribute{
										Computed:    true,
										Description: "Whether the organization's plan includes this tier.",
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

func (d *WebsocketServerTypesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WebsocketServerTypesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	serverTypes, err := d.client.ListWebsocketServerTypes(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read WebSocket server types", err.Error())
		return
	}

	state := WebsocketServerTypesDataSourceModel{
		ID:    types.StringValue("websocket_server_types"),
		Types: []WebsocketServerTypeItemModel{},
	}
	for _, t := range serverTypes {
		item := WebsocketServerTypeItemModel{
			Type:           types.StringValue(t.Type),
			Label:          types.StringValue(t.Label),
			Available:      types.BoolValue(t.Available),
			Regions:        []types.String{},
			MaxConnections: []WebsocketConnectionTierItemModel{},
		}
		for _, r := range t.Regions {
			item.Regions = append(item.Regions, types.StringValue(r))
		}
		for _, tier := range t.MaxConnections {
			item.MaxConnections = append(item.MaxConnections, WebsocketConnectionTierItemModel{
				Value:     types.Int64Value(tier.Value),
				Label:     types.StringValue(tier.Label),
				Available: types.BoolValue(tier.Available),
			})
		}
		state.Types = append(state.Types, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
