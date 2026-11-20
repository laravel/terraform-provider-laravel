package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ datasource.DataSource = &DatabasePresetsDataSource{}

type DatabasePresetsDataSource struct {
	client *client.Client
}

type DatabasePresetsDataSourceModel struct {
	ID      types.String              `tfsdk:"id"`
	Type    types.String              `tfsdk:"type"`
	Region  types.String              `tfsdk:"region"`
	Presets []DatabasePresetItemModel `tfsdk:"presets"`
}

type DatabasePresetItemModel struct {
	Key    types.String `tfsdk:"key"`
	Label  types.String `tfsdk:"label"`
	Config types.String `tfsdk:"config"`
}

func NewDatabasePresetsDataSource() datasource.DataSource {
	return &DatabasePresetsDataSource{}
}

func (d *DatabasePresetsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_database_presets"
}

func (d *DatabasePresetsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the presets (such as Dev, Prod and Scale) the organization can use for a database cluster " +
			"of one type in one region. Sizes are resolved for the region, and presets the region or the " +
			"organization's plan cannot offer are left out.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Placeholder identifier.",
			},
			"type": schema.StringAttribute{
				Required: true,
				Description: "Database type: `laravel_mysql`, `aws_rds_mysql`, `aws_rds_postgres` or " +
					"`neon_serverless_postgres`. Versioned identifiers such as `laravel_mysql_8` are refused.",
			},
			"region": schema.StringAttribute{
				Required:    true,
				Description: "Region, such as `us-east-2`.",
			},
			"presets": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Presets, in the order the API lists them.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Computed:    true,
							Description: "Preset key, such as `dev` or `prod`.",
						},
						"label": schema.StringAttribute{
							Computed:    true,
							Description: "Display name.",
						},
						"config": schema.StringAttribute{
							Computed: true,
							Description: "The preset's cluster settings as a JSON object, such as size, storage and " +
								"retention_days. Its keys depend on the type; read them with jsondecode().",
						},
					},
				},
			},
		},
	}
}

func (d *DatabasePresetsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DatabasePresetsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state DatabasePresetsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	presets, err := d.client.ListDatabasePresets(ctx, state.Type.ValueString(), state.Region.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read database presets", err.Error())
		return
	}

	state.ID = types.StringValue(state.Type.ValueString() + ":" + state.Region.ValueString())
	state.Presets = []DatabasePresetItemModel{}
	for _, p := range presets {
		state.Presets = append(state.Presets, DatabasePresetItemModel{
			Key:    types.StringValue(p.Key),
			Label:  types.StringValue(p.Label),
			Config: types.StringValue(string(p.Config)),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
