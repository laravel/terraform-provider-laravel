package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ datasource.DataSource = &ApplicationsDataSource{}

type ApplicationsDataSource struct {
	client *client.Client
}

type ApplicationsDataSourceModel struct {
	ID           types.String           `tfsdk:"id"`
	Name         types.String           `tfsdk:"name"`
	Region       types.String           `tfsdk:"region"`
	Slug         types.String           `tfsdk:"slug"`
	Applications []ApplicationItemModel `tfsdk:"applications"`
}

type ApplicationItemModel struct {
	ID                        types.String `tfsdk:"id"`
	Name                      types.String `tfsdk:"name"`
	Slug                      types.String `tfsdk:"slug"`
	Region                    types.String `tfsdk:"region"`
	Repository                types.String `tfsdk:"repository"`
	RepositoryDefaultBranch   types.String `tfsdk:"repository_default_branch"`
	SourceControlProviderType types.String `tfsdk:"source_control_provider_type"`
	RootDirectory             types.String `tfsdk:"root_directory"`
	DefaultEnvironmentID      types.String `tfsdk:"default_environment_id"`
	Framework                 types.String `tfsdk:"framework"`
	Language                  types.String `tfsdk:"language"`
	CreatedAt                 types.String `tfsdk:"created_at"`
}

func NewApplicationsDataSource() datasource.DataSource {
	return &ApplicationsDataSource{}
}

func (d *ApplicationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_applications"
}

func (d *ApplicationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the organization's applications, optionally filtered by name, region and slug. " +
			"Use it to reference an application that is not managed in this configuration, such as one " +
			"created in the dashboard or by another Terraform workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Placeholder identifier.",
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Description: "Only list applications whose name contains this value. The API matches it as a substring.",
			},
			"region": schema.StringAttribute{
				Optional:    true,
				Description: "Only list applications whose region contains this value, such as `us-east-2`. The API matches it as a substring.",
			},
			"slug": schema.StringAttribute{
				Optional:    true,
				Description: "Only list applications whose slug contains this value. The API matches it as a substring.",
			},
			"applications": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Matching applications.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Application ID.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "Application name.",
						},
						"slug": schema.StringAttribute{
							Computed:    true,
							Description: "URL-friendly slug.",
						},
						"region": schema.StringAttribute{
							Computed:    true,
							Description: "Cloud region (such as us-east-1).",
						},
						"repository": schema.StringAttribute{
							Computed:    true,
							Description: "Source code repository, in owner/name form. Null when the application has no repository.",
						},
						"repository_default_branch": schema.StringAttribute{
							Computed:    true,
							Description: "Default branch of the repository.",
						},
						"source_control_provider_type": schema.StringAttribute{
							Computed: true,
							Description: "Source control provider of the repository (github, gitlab, gitlab_self_hosted " +
								"or bitbucket). Null without a connected repository.",
						},
						"root_directory": schema.StringAttribute{
							Computed:    true,
							Description: "Subdirectory of the repository the application lives in. Null for the repository root.",
						},
						"default_environment_id": schema.StringAttribute{
							Computed:    true,
							Description: "ID of the application's default (production) environment.",
						},
						"framework": schema.StringAttribute{
							Computed:    true,
							Description: "Framework of the default environment, as Laravel Cloud detected it (such as laravel, nextjs).",
						},
						"language": schema.StringAttribute{
							Computed:    true,
							Description: "Language of the default environment (such as php, javascript).",
						},
						"created_at": schema.StringAttribute{
							Computed:    true,
							Description: "When the application was created.",
						},
					},
				},
			},
		},
	}
}

func (d *ApplicationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ApplicationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ApplicationsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apps, err := d.client.ListApplications(ctx, map[string]string{
		"name":   state.Name.ValueString(),
		"region": state.Region.ValueString(),
		"slug":   state.Slug.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to read applications", err.Error())
		return
	}

	state.ID = types.StringValue("applications")
	// An empty result must be an empty list, not a null one: a nil
	// slice decodes to null and makes length()/for_each error.
	state.Applications = []ApplicationItemModel{}
	for _, a := range apps {
		item := ApplicationItemModel{
			ID:                        types.StringValue(a.ID),
			Name:                      types.StringValue(a.Attributes.Name),
			Slug:                      types.StringValue(a.Attributes.Slug),
			Region:                    types.StringValue(a.Attributes.Region),
			Repository:                types.StringNull(),
			RepositoryDefaultBranch:   types.StringNull(),
			SourceControlProviderType: types.StringPointerValue(a.Attributes.SourceControlProviderType),
			RootDirectory:             types.StringPointerValue(a.Attributes.RootDirectory),
			DefaultEnvironmentID:      types.StringNull(),
			Framework:                 types.StringPointerValue(a.Attributes.Framework),
			Language:                  types.StringPointerValue(a.Attributes.Language),
			CreatedAt:                 types.StringPointerValue(a.Attributes.CreatedAt),
		}
		if repo := a.Attributes.Repository; repo != nil {
			item.Repository = types.StringValue(repo.FullName)
			item.RepositoryDefaultBranch = types.StringPointerValue(repo.DefaultBranch)
		}
		if id := a.Relationships.DefaultEnvironment.RelatedID(); id != "" {
			item.DefaultEnvironmentID = types.StringValue(id)
		}
		state.Applications = append(state.Applications, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
