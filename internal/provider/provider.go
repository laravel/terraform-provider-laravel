package provider

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ provider.Provider = &LaravelCloudProvider{}

type LaravelCloudProvider struct {
	version string
}

type LaravelCloudProviderModel struct {
	Token                      types.String `tfsdk:"token"`
	BaseURL                    types.String `tfsdk:"base_url"`
	MaxRequestsPerMinute       types.Int64  `tfsdk:"max_requests_per_minute"`
	MaxCreateRequestsPerMinute types.Int64  `tfsdk:"max_create_requests_per_minute"`
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
				Description: "Laravel Cloud API token. You can also set it with the LARAVEL_CLOUD_API_TOKEN environment " +
					"variable. If neither is set, the token saved by the Laravel Cloud CLI (`cloud auth`) " +
					"in ~/.config/cloud is used.",
				Optional:  true,
				Sensitive: true,
			},
			"base_url": schema.StringAttribute{
				Description: "Override the Laravel Cloud API base URL. You can also set it with the " +
					"LARAVEL_CLOUD_BASE_URL environment variable. Defaults to " +
					"https://cloud.laravel.com/api.",
				Optional: true,
			},
			"max_requests_per_minute": schema.Int64Attribute{
				Description: fmt.Sprintf("The most API requests the provider sends a minute. Laravel Cloud allows "+
					"120 a minute per organization, shared by every token and every run, and refuses the rest. "+
					"The provider spaces its requests to stay under this, so a large apply slows down instead "+
					"of failing. You can also set it with the LARAVEL_CLOUD_MAX_REQUESTS_PER_MINUTE environment "+
					"variable. Defaults to %d. Set 0 to turn pacing off. The budget is per provider process: "+
					"lower it when several Terraform runs use the same organization at once, since their "+
					"requests together can still exceed Laravel Cloud's limit.", client.DefaultMaxRequestsPerMinute),
				Optional:   true,
				Validators: []validator.Int64{int64validator.AtLeast(0)},
			},
			"max_create_requests_per_minute": schema.Int64Attribute{
				Description: fmt.Sprintf("The most requests a minute the provider sends to the routes Laravel "+
					"Cloud limits to 30 a minute per organization: creating applications, environments, "+
					"instances, database clusters, snapshots, restores, buckets, caches, WebSocket servers and "+
					"secrets, and updating instances. You can also set it with the "+
					"LARAVEL_CLOUD_MAX_CREATE_REQUESTS_PER_MINUTE environment variable. Defaults to %d. "+
					"Set 0 to turn this limit off.", client.DefaultMaxCreateRequestsPerMinute),
				Optional:   true,
				Validators: []validator.Int64{int64validator.AtLeast(0)},
			},
		},
	}
}

// rateLimit resolves one pacing setting: the provider attribute if set,
// otherwise the environment variable, otherwise def.
func rateLimit(attr types.Int64, env string, def int) (int, error) {
	if !attr.IsNull() && !attr.IsUnknown() {
		return int(attr.ValueInt64()), nil
	}
	s := os.Getenv(env)
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%s must be a whole number of requests a minute, 0 or more; got %q", env, s)
	}
	return n, nil
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
			"Unknown API token",
			"The Laravel Cloud API token is not known until apply, because it is derived "+
				"from a resource that has not been created yet. Apply that resource first, "+
				"or set the token with the LARAVEL_CLOUD_API_TOKEN environment variable.",
		)
		return
	}

	// The base URL follows the same precedence as the token: an explicit
	// provider attribute wins, otherwise the environment variable. Without the
	// env var there is no way to point acceptance tests at a non-production
	// host, which is what TESTING.md has always documented. It is resolved
	// first because it decides which CLI config file a token is read from.
	baseURL := os.Getenv("LARAVEL_CLOUD_BASE_URL")
	if !config.BaseURL.IsNull() && !config.BaseURL.IsUnknown() {
		baseURL = config.BaseURL.ValueString()
	}

	token := os.Getenv("LARAVEL_CLOUD_API_TOKEN")
	if !config.Token.IsNull() {
		token = config.Token.ValueString()
	}

	// Last resort: the token `cloud auth` saved. Only consulted when nothing
	// was configured, so an explicit token can never be overridden by a login
	// on the machine running Terraform.
	if token == "" {
		cliTok, cliPath, err := cliToken(baseURL)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to read the Laravel Cloud CLI token",
				"No API token was set in the provider configuration or in the LARAVEL_CLOUD_API_TOKEN "+
					"environment variable, so the provider looked for one saved by the Laravel Cloud CLI "+
					"and could not use it: "+err.Error()+". Set the token in the provider configuration "+
					"or in LARAVEL_CLOUD_API_TOKEN.",
			)
			return
		}
		if cliTok != "" {
			tflog.Debug(ctx, "Using Laravel Cloud API token saved by the Laravel Cloud CLI", map[string]any{"path": cliPath})
		}
		token = cliTok
	}

	if token == "" {
		resp.Diagnostics.AddError(
			"Missing API token",
			"Set the Laravel Cloud API token in the provider configuration, in the "+
				"LARAVEL_CLOUD_API_TOKEN environment variable, or by logging in with the Laravel "+
				"Cloud CLI (`cloud auth`).",
		)
		return
	}

	var opts []client.ClientOption
	if baseURL != "" {
		opts = append(opts, client.WithBaseURL(baseURL))
	}
	// Pace requests under Laravel Cloud's rate limits. Writes cannot be
	// retried when throttled, so spacing them up front is what keeps a large
	// apply from failing.
	perMinute, err := rateLimit(config.MaxRequestsPerMinute, "LARAVEL_CLOUD_MAX_REQUESTS_PER_MINUTE", client.DefaultMaxRequestsPerMinute)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("max_requests_per_minute"), "Invalid rate limit", err.Error())
		return
	}
	createPerMinute, err := rateLimit(config.MaxCreateRequestsPerMinute, "LARAVEL_CLOUD_MAX_CREATE_REQUESTS_PER_MINUTE", client.DefaultMaxCreateRequestsPerMinute)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("max_create_requests_per_minute"), "Invalid rate limit", err.Error())
		return
	}
	opts = append(opts, client.WithRateLimits(perMinute, createPerMinute))

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
		NewDatabaseUserResource,
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
		NewScheduledCommandResource,
		NewSecretResource,
		NewEnvironmentSecretResource,
		NewEnvironmentResendResource,
		NewEdgeNetworkRuleResource,
		NewEnvironmentInternalNetworkingResource,
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
		NewWebsocketServerTypesDataSource,
		NewDatabasePresetsDataSource,
		NewResendDomainsDataSource,
		NewResendReusableKeysDataSource,
	}
}
