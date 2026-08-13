package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ datasource.DataSource = &IPAddressesDataSource{}

type IPAddressesDataSource struct {
	client *client.Client
}

type IPAddressesDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Region      types.String `tfsdk:"region"`
	IPAddresses types.List   `tfsdk:"ip_addresses"`
}

func NewIPAddressesDataSource() datasource.DataSource {
	return &IPAddressesDataSource{}
}

func (d *IPAddressesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_ip_addresses"
}

func (d *IPAddressesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Laravel Cloud egress IP addresses for firewall allowlisting.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Placeholder identifier.",
			},
			"region": schema.StringAttribute{
				Optional: true,
				Description: "Limit the result to a single region, e.g. \"us-east-1\". " +
					"Omit to return the addresses for every region. An unknown region is rejected by the API.",
			},
			"ip_addresses": schema.ListAttribute{
				Computed: true,
				Description: "Sorted IPv4 addresses and IPv6 CIDR ranges to allowlist. " +
					"Covers every region unless `region` is set.",
				ElementType: types.StringType,
			},
		},
	}
}

func (d *IPAddressesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *IPAddressesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config IPAddressesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ips, err := d.client.ListIPAddresses(ctx, config.Region.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read IP addresses", err.Error())
		return
	}

	ipList, diags := types.ListValueFrom(ctx, types.StringType, ips)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := "ip_addresses"
	if region := config.Region.ValueString(); region != "" {
		id = "ip_addresses:" + region
	}

	state := IPAddressesDataSourceModel{
		ID:          types.StringValue(id),
		Region:      config.Region,
		IPAddresses: ipList,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
