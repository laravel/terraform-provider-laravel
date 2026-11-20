package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ datasource.DataSource = &ResendReusableKeysDataSource{}

type ResendReusableKeysDataSource struct {
	client *client.Client
}

type ResendReusableKeysDataSourceModel struct {
	ID            types.String                 `tfsdk:"id"`
	EnvironmentID types.String                 `tfsdk:"environment_id"`
	Keys          []ResendReusableKeyItemModel `tfsdk:"keys"`
}

type ResendReusableKeyItemModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	EnvironmentCount types.Int64  `tfsdk:"environment_count"`
}

func NewResendReusableKeysDataSource() datasource.DataSource {
	return &ResendReusableKeysDataSource{}
}

func (d *ResendReusableKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_resend_reusable_keys"
}

func (d *ResendReusableKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the Resend sending keys of other environments that an environment can reuse. " +
			"Pass a key's id as reuse_key_id on laravel_cloud_environment_resend. " +
			"The organization must have the Resend integration.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The environment ID.",
			},
			"environment_id": schema.StringAttribute{
				Required:    true,
				Description: "ID of the environment that would reuse a key.",
			},
			"keys": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Reusable sending keys.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Sending key ID.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "Sending key name.",
						},
						"environment_count": schema.Int64Attribute{
							Computed:    true,
							Description: "How many environments use the key.",
						},
					},
				},
			},
		},
	}
}

func (d *ResendReusableKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ResendReusableKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ResendReusableKeysDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keys, err := d.client.ListReusableResendKeys(ctx, state.EnvironmentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read reusable Resend keys", err.Error())
		return
	}

	state.ID = state.EnvironmentID
	state.Keys = []ResendReusableKeyItemModel{}
	for _, k := range keys {
		state.Keys = append(state.Keys, ResendReusableKeyItemModel{
			ID:               types.StringValue(k.ID),
			Name:             types.StringValue(k.Name),
			EnvironmentCount: types.Int64Value(k.EnvironmentCount),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
