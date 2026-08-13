package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ datasource.DataSource = &InstanceSizesDataSource{}

type InstanceSizesDataSource struct {
	client *client.Client
}

type InstanceSizesDataSourceModel struct {
	ID    types.String            `tfsdk:"id"`
	Sizes []InstanceSizeItemModel `tfsdk:"sizes"`
}

type InstanceSizeItemModel struct {
	Name         types.String `tfsdk:"name"`
	Label        types.String `tfsdk:"label"`
	Description  types.String `tfsdk:"description"`
	CPUType      types.String `tfsdk:"cpu_type"`
	ComputeClass types.String `tfsdk:"compute_class"`
	CPUCount     types.Int64  `tfsdk:"cpu_count"`
	MemoryMiB    types.Int64  `tfsdk:"memory_mib"`
}

func NewInstanceSizesDataSource() datasource.DataSource {
	return &InstanceSizesDataSource{}
}

func (d *InstanceSizesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_instance_sizes"
}

func (d *InstanceSizesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists available instance sizes.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Placeholder identifier.",
			},
			"sizes": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Available instance sizes.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "Size identifier (e.g. compute-1).",
						},
						"label": schema.StringAttribute{
							Computed:    true,
							Description: "Human-friendly label.",
						},
						"description": schema.StringAttribute{
							Computed:    true,
							Description: "Size description.",
						},
						"cpu_type": schema.StringAttribute{
							Computed:    true,
							Description: "CPU type.",
						},
						"compute_class": schema.StringAttribute{
							Computed:    true,
							Description: "Compute class (general, compute, memory).",
						},
						"cpu_count": schema.Int64Attribute{
							Computed:    true,
							Description: "Number of CPU cores.",
						},
						"memory_mib": schema.Int64Attribute{
							Computed:    true,
							Description: "Memory in MiB.",
						},
					},
				},
			},
		},
	}
}

func (d *InstanceSizesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *InstanceSizesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	result, err := d.client.ListInstanceSizes(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read instance sizes", err.Error())
		return
	}

	state := InstanceSizesDataSourceModel{
		ID: types.StringValue("instance_sizes"),
	}

	for _, s := range result.Data.General {
		state.Sizes = append(state.Sizes, InstanceSizeItemModel{
			Name:         types.StringValue(s.Name),
			Label:        types.StringValue(s.Label),
			Description:  types.StringValue(s.Description),
			CPUType:      types.StringValue(s.CPUType),
			ComputeClass: types.StringValue(s.ComputeClass),
			CPUCount:     types.Int64Value(int64(s.CPUCount)),
			MemoryMiB:    types.Int64Value(int64(s.MemoryMiB)),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
