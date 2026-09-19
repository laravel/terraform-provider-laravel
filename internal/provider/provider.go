package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
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
				Description: "Override the Laravel Cloud API base URL. Can also be set via the " +
					"LARAVEL_CLOUD_BASE_URL environment variable. Defaults to " +
					"https://cloud.laravel.com/api.",
				Optional: true,
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

	// An unknown token is not the same as an unset one. Treating it as unset
	// reads ValueString() as "", which both discards a token supplied via the
	// environment and reports it as missing -- telling the user to set
	// something they did in fact set. base_url below already made this
	// distinction; token did not.
	if config.Token.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Unknown API Token",
			"The Laravel Cloud API token is not known until apply, because it is derived "+
				"from a resource that has not been created yet. Apply that resource first, "+
				"or supply the token via the LARAVEL_CLOUD_API_TOKEN environment variable.",
		)
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

	// The base URL follows the same precedence as the token: an explicit
	// provider attribute wins, otherwise the environment variable. Without the
	// env var there is no way to point acceptance tests at a non-production
	// host, which is what TESTING.md has always documented.
	baseURL := os.Getenv("LARAVEL_CLOUD_BASE_URL")
	if !config.BaseURL.IsNull() && !config.BaseURL.IsUnknown() {
		baseURL = config.BaseURL.ValueString()
	}

	var opts []client.ClientOption
	if baseURL != "" {
		opts = append(opts, client.WithBaseURL(baseURL))
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
		NewEdgeNetworksDataSource,
	}
}
