package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ datasource.DataSource = &EnvironmentsDataSource{}

type EnvironmentsDataSource struct {
	client *client.Client
}

type EnvironmentsDataSourceModel struct {
	ID            types.String           `tfsdk:"id"`
	ApplicationID types.String           `tfsdk:"application_id"`
	Name          types.String           `tfsdk:"name"`
	Slug          types.String           `tfsdk:"slug"`
	Status        types.String           `tfsdk:"status"`
	Environments  []EnvironmentItemModel `tfsdk:"environments"`
}

type EnvironmentItemModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Slug             types.String `tfsdk:"slug"`
	DisplayName      types.String `tfsdk:"display_name"`
	Status           types.String `tfsdk:"status"`
	ClusterID        types.String `tfsdk:"cluster_id"`
	PHPMajorVersion  types.String `tfsdk:"php_major_version"`
	NodeVersion      types.String `tfsdk:"node_version"`
	VanityHostname   types.String `tfsdk:"vanity_hostname"`
	Framework        types.String `tfsdk:"framework"`
	Language         types.String `tfsdk:"language"`
	UsesPushToDeploy types.Bool   `tfsdk:"uses_push_to_deploy"`
	CreatedAt        types.String `tfsdk:"created_at"`
}

func NewEnvironmentsDataSource() datasource.DataSource {
	return &EnvironmentsDataSource{}
}

func (d *EnvironmentsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_environments"
}

func (d *EnvironmentsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists an application's environments, optionally filtered by name, slug and status. " +
			"Use it to reference an environment that is not managed in this configuration, such as one " +
			"created in the dashboard, a preview environment, or one managed by another Terraform workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier for this data source: the application ID.",
			},
			"application_id": schema.StringAttribute{
				Required:    true,
				Description: "ID of the application whose environments to list.",
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Description: "Only list environments whose name contains this value. The API matches it as a substring.",
			},
			"slug": schema.StringAttribute{
				Optional:    true,
				Description: "Only list environments whose slug contains this value. The API matches it as a substring.",
			},
			"status": schema.StringAttribute{
				Optional:    true,
				Description: "Only list environments whose status contains this value, such as `running` or `stopped`. The API matches it as a substring.",
			},
			"environments": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Matching environments.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Environment ID.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "Environment name.",
						},
						"slug": schema.StringAttribute{
							Computed:    true,
							Description: "URL-friendly slug.",
						},
						"display_name": schema.StringAttribute{
							Computed:    true,
							Description: "Name shown in the dashboard: the name, or a preview environment's pull request title.",
						},
						"status": schema.StringAttribute{
							Computed:    true,
							Description: "Current environment status.",
						},
						"cluster_id": schema.StringAttribute{
							Computed:    true,
							Description: "ID of the dedicated cluster the environment runs on; null on shared compute.",
						},
						"php_major_version": schema.StringAttribute{
							Computed:    true,
							Description: "Major PHP version (such as \"8.4\").",
						},
						"node_version": schema.StringAttribute{
							Computed:    true,
							Description: "Node.js version.",
						},
						"vanity_hostname": schema.StringAttribute{
							Computed:    true,
							Description: "Full vanity domain hostname, such as \"my-app.laravel.cloud\". Null when there is none.",
						},
						"framework": schema.StringAttribute{
							Computed:    true,
							Description: "Framework Laravel Cloud detected in the branch (such as laravel, nextjs).",
						},
						"language": schema.StringAttribute{
							Computed:    true,
							Description: "Language of the environment (such as php, javascript).",
						},
						"uses_push_to_deploy": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether a push to the environment's branch deploys it.",
						},
						"created_at": schema.StringAttribute{
							Computed:    true,
							Description: "When the environment was created.",
						},
					},
				},
			},
		},
	}
}

func (d *EnvironmentsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected data source configure type",
			fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *EnvironmentsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state EnvironmentsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envs, err := d.client.ListEnvironments(ctx, state.ApplicationID.ValueString(), map[string]string{
		"name":   state.Name.ValueString(),
		"slug":   state.Slug.ValueString(),
		"status": state.Status.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to read environments", err.Error())
		return
	}

	state.ID = state.ApplicationID
	// An empty result must be an empty list, not a null one: a nil
	// slice decodes to null and makes length()/for_each error.
	state.Environments = []EnvironmentItemModel{}
	for _, e := range envs {
		item := EnvironmentItemModel{
			ID:               types.StringValue(e.ID),
			Name:             types.StringValue(e.Attributes.Name),
			Slug:             types.StringValue(e.Attributes.Slug),
			DisplayName:      types.StringPointerValue(e.Attributes.DisplayName),
			Status:           types.StringValue(e.Attributes.Status),
			ClusterID:        types.StringPointerValue(e.Attributes.ClusterID),
			PHPMajorVersion:  types.StringValue(e.Attributes.PHPMajorVersion),
			NodeVersion:      types.StringValue(e.Attributes.NodeVersion),
			VanityHostname:   types.StringNull(),
			Framework:        types.StringPointerValue(e.Attributes.Framework),
			Language:         types.StringPointerValue(e.Attributes.Language),
			UsesPushToDeploy: types.BoolValue(e.Attributes.UsesPushToDeploy),
			CreatedAt:        types.StringPointerValue(e.Attributes.CreatedAt),
		}
		if host := e.Attributes.VanityDomain; host != nil && *host != "" {
			item.VanityHostname = types.StringValue(*host)
		}
		state.Environments = append(state.Environments, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
