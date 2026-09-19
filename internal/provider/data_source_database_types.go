package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ datasource.DataSource = &DatabaseTypesDataSource{}

type DatabaseTypesDataSource struct {
	client *client.Client
}

type DatabaseTypesDataSourceModel struct {
	ID    types.String            `tfsdk:"id"`
	Types []DatabaseTypeItemModel `tfsdk:"types"`
}

type DatabaseTypeItemModel struct {
	Type     types.String `tfsdk:"type"`
	Label    types.String `tfsdk:"label"`
	Regions  types.List   `tfsdk:"regions"`
	Sizes    types.List   `tfsdk:"sizes"`
	Versions types.List   `tfsdk:"versions"`
}

func NewDatabaseTypesDataSource() datasource.DataSource {
	return &DatabaseTypesDataSource{}
}

func (d *DatabaseTypesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_database_types"
}

func (d *DatabaseTypesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists available database types, their supported regions, and available sizes.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Placeholder identifier.",
			},
			"types": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Available database types.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Computed:    true,
							Description: "Type identifier (e.g. laravel_mysql, aws_rds_postgres). A type whose \"versions\" list is empty is a retired identifier that bakes the version into the type, such as laravel_mysql_84.",
						},
						"label": schema.StringAttribute{
							Computed:    true,
							Description: "Human-friendly label.",
						},
						"regions": schema.ListAttribute{
							Computed:    true,
							ElementType: types.StringType,
							Description: "Supported regions.",
						},
						"sizes": schema.ListAttribute{
							Computed:    true,
							ElementType: types.StringType,
							Description: "Available sizes for this database type (e.g. mysql-flex-1gb, db.m8g.large). Empty for serverless types.",
						},
						"versions": schema.ListAttribute{
							Computed:    true,
							ElementType: types.StringType,
							Description: "Engine versions accepted for this type. Pass one of these as the " +
								"version argument of laravel_cloud_database_cluster.",
						},
					},
				},
			},
		},
	}
}

func (d *DatabaseTypesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DatabaseTypesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	dbTypes, err := d.client.ListDatabaseTypes(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read database types", err.Error())
		return
	}

	state := DatabaseTypesDataSourceModel{
		ID: types.StringValue("database_types"),
	}

	for _, dt := range dbTypes {
		regions, diags := types.ListValueFrom(ctx, types.StringType, dt.Regions)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Sizes may be nil for types that don't have discrete size options
		// (e.g. Neon serverless uses compute units instead).
		sizeStrings := dt.Sizes
		if sizeStrings == nil {
			sizeStrings = []string{}
		}
		sizes, diags := types.ListValueFrom(ctx, types.StringType, sizeStrings)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		versionStrings := dt.Versions
		if versionStrings == nil {
			versionStrings = []string{}
		}
		versions, diags := types.ListValueFrom(ctx, types.StringType, versionStrings)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		state.Types = append(state.Types, DatabaseTypeItemModel{
			Type:     types.StringValue(dt.Type),
			Label:    types.StringValue(dt.Label),
			Regions:  regions,
			Sizes:    sizes,
			Versions: versions,
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
