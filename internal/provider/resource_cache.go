package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                = &CacheResource{}
	_ resource.ResourceWithImportState = &CacheResource{}
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
				Required:    true,
				Description: "Cache type (upstash_redis, laravel_valkey).",
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
				Required:    true,
				Description: "Cache size (250mb to 500gb).",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Cache status.",
			},
			"auto_upgrade_enabled": schema.BoolAttribute{
				Required:    true,
				Description: "Enable automatic upgrades.",
			},
			"is_public": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the cache is publicly accessible.",
			},
			"eviction_policy": schema.StringAttribute{
				Optional:    true,
				Description: "Eviction policy (Valkey only, defaults to allkeys-lru).",
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
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
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
	if !plan.EvictionPolicy.IsNull() {
		v := plan.EvictionPolicy.ValueString()
		createReq.EvictionPolicy = &v
	}

	cache, err := r.client.CreateCache(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating cache", err.Error())
		return
	}

	// The create response describes a cache that is still provisioning: its
	// connection block comes back empty (null hostname, zero port, no
	// password). Storing that would hand every downstream reference an empty
	// password with no error, so wait for the API to fill it in.
	if !cache.Attributes.Connection.IsReady() {
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

	cache, err := r.client.UpdateCache(ctx, state.ID.ValueString(), updateReq)
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
	state.Connection = mapCacheConnection(c.Attributes.Connection)
	state.CreatedAt = types.StringPointerValue(c.Attributes.CreatedAt)
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
