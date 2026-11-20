package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                   = &CacheResource{}
	_ resource.ResourceWithImportState    = &CacheResource{}
	_ resource.ResourceWithValidateConfig = &CacheResource{}
)

type CacheResource struct {
	client *client.Client
}

type CacheResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Type               types.String `tfsdk:"type"`
	Region             types.String `tfsdk:"region"`
	Size               types.String `tfsdk:"size"`
	Status             types.String `tfsdk:"status"`
	AutoUpgradeEnabled types.Bool   `tfsdk:"auto_upgrade_enabled"`
	IsPublic           types.Bool   `tfsdk:"is_public"`
	EvictionPolicy     types.String `tfsdk:"eviction_policy"`
	UsesHibernation    types.Bool   `tfsdk:"uses_hibernation"`
	HibernationTimeout types.Int64  `tfsdk:"hibernation_timeout"`
	ClusterID          types.String `tfsdk:"cluster_id"`
	Connection         types.Object `tfsdk:"connection_details"`
	CreatedAt          types.String `tfsdk:"created_at"`
}

// cacheConnectionAttrTypes describes the connection_details object. It is
// modeled as a types.Object (not a Go pointer-struct) so the framework can
// represent the value as unknown during the create plan — a pointer-struct
// cannot hold unknown, which otherwise fails Create's Plan.Get.
var cacheConnectionAttrTypes = map[string]attr.Type{
	"hostname": types.StringType,
	"port":     types.Int64Type,
	"protocol": types.StringType,
	"username": types.StringType,
	"password": types.StringType,
}

func mapCacheConnection(c *client.CacheConnection) types.Object {
	if c == nil {
		return types.ObjectNull(cacheConnectionAttrTypes)
	}
	obj, _ := types.ObjectValue(cacheConnectionAttrTypes, map[string]attr.Value{
		"hostname": types.StringPointerValue(c.Hostname),
		"port":     types.Int64PointerValue(c.Port),
		"protocol": types.StringPointerValue(c.Protocol),
		"username": types.StringPointerValue(c.Username),
		"password": types.StringPointerValue(c.Password),
	})
	return obj
}

func NewCacheResource() resource.Resource {
	return &CacheResource{}
}

func (r *CacheResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_cache"
}

func (r *CacheResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Laravel Cloud cache (Valkey/Redis).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Cache name (3-40 characters, lowercase alphanumeric with hyphens/underscores).",
			},
			"type": schema.StringAttribute{
				Required: true,
				Description: "Cache type: laravel_valkey, or aws_elasticache_valkey or aws_elasticache_redis, which " +
					"run on one of the organization's dedicated (Private Cloud) ElastiCache clusters. The " +
					"laravel_cloud_cache_types data source lists the types, their regions and sizes. " +
					"upstash_redis is deprecated and the API no longer creates it.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"region": schema.StringAttribute{
				Required:    true,
				Description: "Cloud region.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"size": schema.StringAttribute{
				Required: true,
				Description: "Cache size, which depends on the type. laravel_valkey takes valkey-flex-* sizes " +
					"(250mb to 2.5gb, which can scale to zero) or valkey-pro.* sizes (250mb to 50gb); the " +
					"ElastiCache types take an elasticache.* node type, such as elasticache.cache.t4g.micro. " +
					"The laravel_cloud_cache_types data source lists the sizes each type accepts.",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Cache status.",
			},
			"auto_upgrade_enabled": schema.BoolAttribute{
				Required:    true,
				Description: "Enable automatic upgrades. The ElastiCache types do not support it, so set it to false for them.",
			},
			"is_public": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Whether the cache is publicly accessible. Required for laravel_valkey. ElastiCache " +
					"caches are always private: leave it out or set it to false, because the API stores " +
					"false whatever is sent and refuses to change it.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"eviction_policy": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Eviction policy, for cache types that support one (supports_eviction_policy in " +
					"the laravel_cloud_cache_types data source). When omitted, the API uses allkeys-lru " +
					"for those types and reports null for the rest.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"uses_hibernation": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Scale to zero: the cache sleeps after hibernation_timeout idle minutes. Only " +
					"sizes whose supports_hibernation is true in the laravel_cloud_cache_types data source " +
					"accept it (the Laravel Valkey Flex sizes). When omitted, the API's default applies, " +
					"which is false: unlike the dashboard, the API does not turn it on for Flex caches.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"hibernation_timeout": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Idle minutes before the cache sleeps, 1 to 60. Required when uses_hibernation " +
					"is true, and not allowed otherwise: the API clears it while scale to zero is off.",
				Validators: []validator.Int64{
					int64validator.Between(1, 60),
				},
				PlanModifiers: []planmodifier.Int64{
					hibernationTimeoutFollowsUsesHibernation{},
				},
			},
			"cluster_id": schema.StringAttribute{
				Optional: true,
				Description: "Dedicated ElastiCache cluster ID, from the laravel_cloud_dedicated_clusters data " +
					"source (type elasticache-clusters). Only the ElastiCache types accept it, and the cluster " +
					"must be active and in the cache's region. When omitted, the API picks one of the " +
					"organization's dedicated ElastiCache clusters in that region, in no defined order, so " +
					"set it when the organization has more than one there. Create-only: no API endpoint " +
					"reports a cache's cluster, so Terraform cannot read it back, detect a cache on another " +
					"cluster, or fill it in on import. An imported cache leaves it null, and adding it to " +
					"the configuration afterwards records it without replacing the cache. Changing a " +
					"recorded value forces a new cache, as the update endpoint does not accept it.",
				PlanModifiers: []planmodifier.String{
					requiresReplaceUnlessImported(),
				},
			},
			"connection_details": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Read-only connection details for the cache.",
				Attributes: map[string]schema.Attribute{
					"hostname": schema.StringAttribute{Computed: true},
					"port":     schema.Int64Attribute{Computed: true},
					"protocol": schema.StringAttribute{Computed: true},
					"username": schema.StringAttribute{Computed: true},
					"password": schema.StringAttribute{Computed: true, Sensitive: true},
				},
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *CacheResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected resource configure type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *CacheResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CacheResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateCacheRequest{
		Type:               plan.Type.ValueString(),
		Name:               plan.Name.ValueString(),
		Region:             plan.Region.ValueString(),
		Size:               plan.Size.ValueString(),
		AutoUpgradeEnabled: plan.AutoUpgradeEnabled.ValueBool(),
		IsPublic:           plan.IsPublic.ValueBool(),
	}
	if !plan.EvictionPolicy.IsNull() && !plan.EvictionPolicy.IsUnknown() {
		v := plan.EvictionPolicy.ValueString()
		createReq.EvictionPolicy = &v
	}
	if !plan.ClusterID.IsNull() {
		v := plan.ClusterID.ValueString()
		createReq.ClusterID = &v
	}
	if !plan.UsesHibernation.IsNull() && !plan.UsesHibernation.IsUnknown() {
		createReq.UsesHibernation = plan.UsesHibernation.ValueBoolPointer()
		createReq.HibernationTimeout = plan.HibernationTimeout.ValueInt64Pointer()
	}

	cache, err := r.client.CreateCache(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating cache", err.Error())
		return
	}

	// The create response describes a cache that is still provisioning: its
	// connection block comes back empty (null hostname, zero port, no
	// password). Storing that would hand every downstream reference an empty
	// password with no error, so wait for the API to fill it in. ElastiCache
	// is the exception: the API never reports its hostname, even once the
	// cache is available, so waiting would only add five minutes.
	elasticache := isElastiCacheType(plan.Type.ValueString())
	if !elasticache && !cache.Attributes.Connection.IsReady() {
		if ready := waitForCacheConnection(ctx, r.client, cache.ID); ready != nil {
			cache = ready
		} else {
			resp.Diagnostics.AddWarning(
				"Cache connection details not available yet",
				"The cache was created but is still provisioning, so its connection "+
					"details are not populated. Run 'terraform refresh' or the next "+
					"'terraform apply' to pick them up.",
			)
		}
	}

	// Save the cache before waiting, so it is not lost if the wait fails.
	mapCacheToState(cache, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The connection details arrive about a minute before the cache leaves
	// "creating", and until then the API refuses to attach it to an
	// environment, which is usually the next thing in the same apply. An
	// ElastiCache cache provisions an AWS replication group, which takes
	// over ten minutes.
	attempts := cacheBusyAttempts
	if elasticache {
		attempts = elastiCacheCreateAttempts
	}
	cache, err = waitForCacheAvailable(ctx, r.client, cache.ID, attempts)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for cache to become available", err.Error())
		return
	}

	mapCacheToState(cache, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CacheResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CacheResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cache, err := r.client.GetCache(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading cache", err.Error())
		return
	}

	mapCacheToState(cache, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *CacheResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan CacheResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state CacheResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := client.UpdateCacheRequest{}
	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		updateReq.Name = &v
	}
	if !plan.Size.Equal(state.Size) {
		v := plan.Size.ValueString()
		updateReq.Size = &v
	}
	if !plan.AutoUpgradeEnabled.Equal(state.AutoUpgradeEnabled) {
		v := plan.AutoUpgradeEnabled.ValueBool()
		updateReq.AutoUpgradeEnabled = &v
	}
	if !plan.IsPublic.Equal(state.IsPublic) {
		v := plan.IsPublic.ValueBool()
		updateReq.IsPublic = &v
	}
	if !plan.EvictionPolicy.Equal(state.EvictionPolicy) && !plan.EvictionPolicy.IsNull() {
		v := plan.EvictionPolicy.ValueString()
		updateReq.EvictionPolicy = &v
	}
	// The API requires hibernation_timeout whenever uses_hibernation is sent
	// as true, so a change to either sends both.
	if !plan.UsesHibernation.Equal(state.UsesHibernation) || !plan.HibernationTimeout.Equal(state.HibernationTimeout) {
		updateReq.UsesHibernation = plan.UsesHibernation.ValueBoolPointer()
		updateReq.HibernationTimeout = plan.HibernationTimeout.ValueInt64Pointer()
	}

	// Nothing the API can update changed -- the change is to cluster_id being
	// recorded after an import. Send nothing and keep the computed values.
	if updateReq == (client.UpdateCacheRequest{}) {
		plan.ID = state.ID
		plan.Status = state.Status
		plan.Connection = state.Connection
		plan.CreatedAt = state.CreatedAt
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	// The API refuses an update while the cache is busy ("An update is
	// already in progress for this cache"), which includes the first minute
	// or so after create: the connection details Create waits for arrive
	// before the cache leaves "creating".
	var cache *client.CacheData
	err := client.RetryOnConflict(ctx, cacheBusyAttempts, cacheBusyInterval, func() error {
		var updateErr error
		cache, updateErr = r.client.UpdateCache(ctx, state.ID.ValueString(), updateReq)
		return updateErr
	}, client.IsBusy)
	if err != nil {
		resp.Diagnostics.AddError("Error updating cache", err.Error())
		return
	}

	mapCacheToState(cache, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CacheResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CacheResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A cache that was only just created is still provisioning, and the API
	// rejects a delete while any operation is in progress ("An operation is
	// already in progress for this cache, please wait a few seconds"). Without
	// a retry this fails the destroy and leaves a billable cache behind --
	// observed against a live API, where the create/destroy cycle of an
	// acceptance test is fast enough to hit it every time.
	id := state.ID.ValueString()
	err := client.RetryOnConflict(ctx, 30, 10*time.Second, func() error {
		return r.client.DeleteCache(ctx, id)
	}, func(err error) bool {
		return strings.Contains(err.Error(), "operation is already in progress")
	})
	if err != nil {
		resp.Diagnostics.AddError("Error deleting cache", err.Error())
	}
}

// ValidateConfig turns the API's rules for is_public, and its pairing of
// uses_hibernation and hibernation_timeout, into plan-time errors. The API rejects
// uses_hibernation = true without a timeout, and with scale to zero off it
// stores no timeout, so a configured one would never be read back.
func (r *CacheResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config CacheResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// ElastiCache caches are always private: the API stores is_public =
	// false whatever create sends, and its update endpoint refuses to change
	// it. Every other type requires it.
	if !config.Type.IsUnknown() && !config.Type.IsNull() {
		switch {
		case isElastiCacheType(config.Type.ValueString()) && config.IsPublic.ValueBool():
			resp.Diagnostics.AddAttributeError(
				path.Root("is_public"),
				"is_public must be false for ElastiCache caches",
				"AWS ElastiCache caches are always private: the API stores is_public = false and "+
					"refuses to change it. Remove is_public or set it to false.",
			)
		case !isElastiCacheType(config.Type.ValueString()) && config.IsPublic.IsNull():
			resp.Diagnostics.AddAttributeError(
				path.Root("is_public"),
				"Missing required argument",
				fmt.Sprintf("is_public is required for %s caches. Only the ElastiCache types, which are "+
					"always private, may leave it out.", config.Type.ValueString()),
			)
		}
	}

	if config.UsesHibernation.IsUnknown() || config.HibernationTimeout.IsUnknown() {
		return
	}
	hibernates := config.UsesHibernation.ValueBool()
	switch {
	case hibernates && config.HibernationTimeout.IsNull():
		resp.Diagnostics.AddAttributeError(
			path.Root("hibernation_timeout"),
			"hibernation_timeout is required when uses_hibernation is true",
			"Set hibernation_timeout to the idle minutes, 1 to 60, before the cache sleeps.",
		)
	case !hibernates && !config.HibernationTimeout.IsNull():
		resp.Diagnostics.AddAttributeError(
			path.Root("hibernation_timeout"),
			"hibernation_timeout requires uses_hibernation = true",
			"The API clears the timeout while scale to zero is off. Set uses_hibernation = true, "+
				"or remove hibernation_timeout.",
		)
	}
}

// hibernationTimeoutFollowsUsesHibernation plans an omitted hibernation_timeout
// from uses_hibernation: null once scale to zero is planned off, because the
// API clears the timeout then, and otherwise the stored timeout, so removing
// both attributes from config keeps the cache as it is instead of sending
// uses_hibernation = true without the timeout the API requires with it.
type hibernationTimeoutFollowsUsesHibernation struct{}

func (hibernationTimeoutFollowsUsesHibernation) Description(context.Context) string {
	return "Null while uses_hibernation is false; otherwise keeps the stored value when omitted."
}

func (m hibernationTimeoutFollowsUsesHibernation) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (hibernationTimeoutFollowsUsesHibernation) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if !req.ConfigValue.IsNull() || req.State.Raw.IsNull() {
		return
	}
	// Read uses_hibernation from config, falling back to state as its
	// UseStateForUnknown does: the plan passed here does not have that
	// modifier's result yet.
	var uses types.Bool
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("uses_hibernation"), &uses)...)
	if uses.IsNull() {
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("uses_hibernation"), &uses)...)
	}
	switch {
	case uses.IsUnknown():
		return
	case uses.ValueBool():
		resp.PlanValue = req.StateValue
	default:
		resp.PlanValue = types.Int64Null()
	}
}

// isElastiCacheType reports whether a cache type runs on AWS ElastiCache
// (aws_elasticache_valkey, aws_elasticache_redis).
func isElastiCacheType(t string) bool {
	return strings.HasPrefix(t, "aws_elasticache_")
}

func (r *CacheResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mapCacheToState(c *client.CacheData, state *CacheResourceModel) {
	state.ID = types.StringValue(c.ID)
	state.Name = types.StringValue(c.Attributes.Name)
	state.Type = types.StringValue(c.Attributes.CacheType)
	state.Region = types.StringValue(c.Attributes.Region)
	state.Size = types.StringValue(c.Attributes.Size)
	state.Status = types.StringValue(c.Attributes.Status)
	state.AutoUpgradeEnabled = types.BoolValue(c.Attributes.AutoUpgradeEnabled)
	state.IsPublic = types.BoolValue(c.Attributes.IsPublic)
	state.EvictionPolicy = types.StringPointerValue(c.Attributes.EvictionPolicy)
	state.UsesHibernation = types.BoolValue(c.Attributes.UsesHibernation)
	state.HibernationTimeout = types.Int64PointerValue(c.Attributes.HibernationTimeout)
	state.CreatedAt = types.StringPointerValue(c.Attributes.CreatedAt)

	// The API reports an empty connection block whenever an operation is in
	// progress -- right after create, and again while a resize is applying. If
	// that were mapped straight through, a refresh or an update would null out
	// connection_details.password in state, silently, and anything referencing
	// it would start reading an empty credential. Keep what is already stored
	// until the API has something real to replace it with.
	if c.Attributes.Connection.IsReady() || state.Connection.IsNull() || state.Connection.IsUnknown() {
		state.Connection = mapCacheConnection(c.Attributes.Connection)
	}
}

// cacheBusy bounds how long Create waits for a new cache to become
// available, and Update for a busy one. The interval is a variable so the
// plan tests need not wait.
const cacheBusyAttempts = 30

// elastiCacheCreateAttempts bounds Create's wait for a new ElastiCache cache,
// 30 minutes at cacheBusyInterval.
const elastiCacheCreateAttempts = 180

var cacheBusyInterval = 10 * time.Second

var errCacheBusy = errors.New("cache is still provisioning")

// waitForCacheAvailable polls the cache until it is no longer creating or
// updating, and returns it.
func waitForCacheAvailable(ctx context.Context, c *client.Client, id string, attempts int) (*client.CacheData, error) {
	var cache *client.CacheData
	err := client.RetryOnConflict(ctx, attempts, cacheBusyInterval, func() error {
		var err error
		cache, err = c.GetCache(ctx, id)
		if err != nil {
			return err
		}
		if s := cache.Attributes.Status; s == "creating" || s == "updating" {
			return fmt.Errorf("%w: status is %q", errCacheBusy, s)
		}
		return nil
	}, func(err error) bool { return errors.Is(err, errCacheBusy) })
	return cache, err
}

// cacheConnectionPoll bounds how long Create waits for a new cache to report
// its connection details.
const (
	cacheConnectionPollInterval = 5 * time.Second
	cacheConnectionPollAttempts = 60
)

// waitForCacheConnection polls the cache until its connection block is
// populated, returning nil if it never becomes ready within the budget or the
// context is cancelled. A read error is treated as "not ready yet" rather than
// fatal: the cache does exist, and the caller falls back to the create
// response.
func waitForCacheConnection(ctx context.Context, c *client.Client, id string) *client.CacheData {
	for attempt := 0; attempt < cacheConnectionPollAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(cacheConnectionPollInterval):
		}

		cache, err := c.GetCache(ctx, id)
		if err != nil {
			continue
		}
		if cache.Attributes.Connection.IsReady() {
			return cache
		}
	}
	return nil
}
