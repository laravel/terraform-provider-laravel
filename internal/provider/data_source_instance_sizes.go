package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ datasource.DataSource = &InstanceSizesDataSource{}

type InstanceSizesDataSource struct {
	dataSourceWithClient
}

type InstanceSizesDataSourceModel struct {
	ID    types.String            `tfsdk:"id"`
	Sizes []InstanceSizeItemModel `tfsdk:"sizes"`
}

type InstanceSizeItemModel struct {
	Name          types.String  `tfsdk:"name"`
	Label         types.String  `tfsdk:"label"`
	Description   types.String  `tfsdk:"description"`
	CPUType       types.String  `tfsdk:"cpu_type"`
	ComputeClass  types.String  `tfsdk:"compute_class"`
	CPUCount      types.Float64 `tfsdk:"cpu_count"`
	InstanceClass types.String  `tfsdk:"instance_class"`
	MemoryMiB     types.Int64   `tfsdk:"memory_mib"`
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
							Description: "Size identifier (e.g. flex.c-1vcpu-256mb, mq-pro-1gb).",
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
						"cpu_count": schema.Float64Attribute{
							Computed: true,
							Description: "Number of CPU cores. Managed-queue sizes may express " +
								"a fractional vCPU (e.g. 0.5).",
						},
						"instance_class": schema.StringAttribute{
							Computed: true,
							Description: "Which instance class this size belongs to: " +
								"\"general\" for service instances, \"managed_queue\" for managed queues.",
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

func (d *InstanceSizesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	result, err := d.client.ListInstanceSizes(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read instance sizes", err.Error())
		return
	}

	state := InstanceSizesDataSourceModel{
		ID: types.StringValue("instance_sizes"),
		// An empty result must be an empty list, not a null one: a nil
		// slice decodes to null and makes length()/for_each error.
		Sizes: []InstanceSizeItemModel{},
	}

	// The API returns sizes split by instance class. Both are surfaced, tagged
	// with instance_class so a managed queue can be told from a service size --
	// they are disjoint sets and a size from the wrong one is rejected.
	appendSizes := func(sizes []client.InstanceSizeInfo, instanceClass string) {
		for _, s := range sizes {
			state.Sizes = append(state.Sizes, InstanceSizeItemModel{
				Name:          types.StringValue(s.Name),
				Label:         types.StringValue(s.Label),
				Description:   types.StringValue(s.Description),
				CPUType:       types.StringValue(s.CPUType),
				ComputeClass:  types.StringValue(s.ComputeClass),
				CPUCount:      types.Float64Value(s.CPUCount),
				MemoryMiB:     types.Int64Value(int64(s.MemoryMiB)),
				InstanceClass: types.StringValue(instanceClass),
			})
		}
	}
	appendSizes(result.Data.General, "general")
	appendSizes(result.Data.ManagedQueue, "managed_queue")

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
