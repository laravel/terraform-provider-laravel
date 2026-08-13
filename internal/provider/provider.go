package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ provider.Provider = &LaravelCloudProvider{}

type LaravelCloudProvider struct {
	version string
}

type LaravelCloudProviderModel struct {
	Token   types.String `tfsdk:"token"`
	BaseURL types.String `tfsdk:"base_url"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &LaravelCloudProvider{version: version}
	}
}

func (p *LaravelCloudProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "laravel"
	resp.Version = p.version
}

func (p *LaravelCloudProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Laravel Cloud resources.",
		Attributes: map[string]schema.Attribute{
			"token": schema.StringAttribute{
				Description: "Laravel Cloud API token. Can also be set via the LARAVEL_CLOUD_API_TOKEN environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"base_url": schema.StringAttribute{
				Description: "Override the Laravel Cloud API base URL. Defaults to https://cloud.laravel.com/api.",
				Optional:    true,
			},
		},
	}
}

func (p *LaravelCloudProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config LaravelCloudProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token := os.Getenv("LARAVEL_CLOUD_API_TOKEN")
	if !config.Token.IsNull() {
		token = config.Token.ValueString()
	}

	if token == "" {
		resp.Diagnostics.AddError(
			"Missing API Token",
			"The Laravel Cloud API token must be set in the provider configuration or via the LARAVEL_CLOUD_API_TOKEN environment variable.",
		)
		return
	}

	var opts []client.ClientOption
	if !config.BaseURL.IsNull() && !config.BaseURL.IsUnknown() {
		opts = append(opts, client.WithBaseURL(config.BaseURL.ValueString()))
	}

	c := client.NewClient(token, opts...)
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *LaravelCloudProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewApplicationResource,
		NewEnvironmentResource,
		NewInstanceResource,
		NewDomainResource,
		NewDatabaseClusterResource,
		NewDatabaseResource,
		NewCacheResource,
		NewStorageBucketResource,
		NewStorageBucketKeyResource,
		NewBackgroundProcessResource,
		NewEnvironmentVariableResource,
		NewDatabaseSnapshotResource,
		NewWebsocketServerResource,
		NewWebsocketApplicationResource,
		NewDeploymentResource,
		NewCommandResource,
		NewDatabaseRestoreResource,
	}
}

func (p *LaravelCloudProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewOrganizationDataSource,
		NewDedicatedClustersDataSource,
		NewInstanceSizesDataSource,
		NewDatabaseTypesDataSource,
		NewCacheTypesDataSource,
		NewRegionsDataSource,
		NewIPAddressesDataSource,
	}
}
