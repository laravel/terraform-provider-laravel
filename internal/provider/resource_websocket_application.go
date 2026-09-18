package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                = &WebsocketApplicationResource{}
	_ resource.ResourceWithImportState = &WebsocketApplicationResource{}
)

type WebsocketApplicationResource struct {
	client *client.Client
}

type WebsocketApplicationResourceModel struct {
	ID              types.String `tfsdk:"id"`
	ServerID        types.String `tfsdk:"server_id"`
	Name            types.String `tfsdk:"name"`
	AppID           types.String `tfsdk:"app_id"`
	Key             types.String `tfsdk:"key"`
	Secret          types.String `tfsdk:"secret"`
	AllowedOrigins  types.List   `tfsdk:"allowed_origins"`
	PingInterval    types.Int64  `tfsdk:"ping_interval"`
	ActivityTimeout types.Int64  `tfsdk:"activity_timeout"`
	MaxMessageSize  types.Int64  `tfsdk:"max_message_size"`
	MaxConnections  types.Int64  `tfsdk:"max_connections"`
	CreatedAt       types.String `tfsdk:"created_at"`
}

func NewWebsocketApplicationResource() resource.Resource {
	return &WebsocketApplicationResource{}
}

func (r *WebsocketApplicationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_websocket_application"
}

func (r *WebsocketApplicationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Laravel Cloud WebSocket application.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"server_id": schema.StringAttribute{
				Required:    true,
				Description: "Parent WebSocket server ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Application name.",
			},
			"app_id": schema.StringAttribute{
				Computed:    true,
				Description: "Auto-generated application ID for client connections.",
			},
			"key": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Application key.",
			},
			"secret": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Application secret.",
			},
			"allowed_origins": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Allowed origins for WebSocket connections. The API always " +
					"returns a list here, empty when no origins are restricted, so this is " +
					"Computed as well as Optional. Because it is computed, REMOVING this " +
					"attribute from the configuration leaves the existing origins in place " +
					"rather than clearing them -- assign an empty list to clear them.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"ping_interval": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Ping interval in seconds.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"activity_timeout": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Activity timeout in seconds.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"max_message_size": schema.Int64Attribute{
				Computed:    true,
				Description: "Maximum message size in bytes.",
			},
			"max_connections": schema.Int64Attribute{
				Computed:    true,
				Description: "Maximum concurrent connections.",
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *WebsocketApplicationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *WebsocketApplicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan WebsocketApplicationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateWebsocketApplicationRequest{
		Name: plan.Name.ValueString(),
	}
	// allowed_origins is Computed as well as Optional, so it is unknown here
	// whenever the user did not set it -- decoding an unknown list into
	// []string fails outright.
	if !plan.AllowedOrigins.IsNull() && !plan.AllowedOrigins.IsUnknown() {
		var origins []string
		resp.Diagnostics.Append(plan.AllowedOrigins.ElementsAs(ctx, &origins, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.AllowedOrigins = origins
	}
	if !plan.PingInterval.IsNull() && !plan.PingInterval.IsUnknown() {
		v := int(plan.PingInterval.ValueInt64())
		createReq.PingInterval = &v
	}
	if !plan.ActivityTimeout.IsNull() && !plan.ActivityTimeout.IsUnknown() {
		v := int(plan.ActivityTimeout.ValueInt64())
		createReq.ActivityTimeout = &v
	}

	wsApp, err := r.client.CreateWebsocketApplication(ctx, plan.ServerID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating websocket application", err.Error())
		return
	}

	mapWebsocketApplicationToState(ctx, wsApp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WebsocketApplicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state WebsocketApplicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wsApp, err := r.client.GetWebsocketApplication(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading websocket application", err.Error())
		return
	}

	mapWebsocketApplicationToState(ctx, wsApp, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *WebsocketApplicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan WebsocketApplicationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state WebsocketApplicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := client.UpdateWebsocketApplicationRequest{}
	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		updateReq.Name = &v
	}
	if !plan.AllowedOrigins.Equal(state.AllowedOrigins) && !plan.AllowedOrigins.IsUnknown() {
		// An explicitly empty list is how origins are cleared, so the slice is
		// non-nil even when there is nothing in it -- see the request struct,
		// where allowed_origins deliberately carries no omitempty.
		origins := []string{}
		if !plan.AllowedOrigins.IsNull() {
			resp.Diagnostics.Append(plan.AllowedOrigins.ElementsAs(ctx, &origins, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
		updateReq.AllowedOrigins = &origins
	}
	if !plan.PingInterval.IsNull() && !plan.PingInterval.IsUnknown() {
		v := int(plan.PingInterval.ValueInt64())
		updateReq.PingInterval = &v
	}
	if !plan.ActivityTimeout.IsNull() && !plan.ActivityTimeout.IsUnknown() {
		v := int(plan.ActivityTimeout.ValueInt64())
		updateReq.ActivityTimeout = &v
	}

	wsApp, err := r.client.UpdateWebsocketApplication(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating websocket application", err.Error())
		return
	}

	mapWebsocketApplicationToState(ctx, wsApp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WebsocketApplicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state WebsocketApplicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteWebsocketApplication(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting websocket application", err.Error())
	}
}

func (r *WebsocketApplicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// GET /websocket-applications/{id} does not return the owning server, and
	// server_id forces replacement, so a bare-id import would leave it null and
	// plan a destroy/recreate of a perfectly good application. Import as
	// "<server_id>:<application_id>", matching laravel_cloud_database.
	serverID, id, found := strings.Cut(req.ID, ":")
	if !found || serverID == "" || id == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"server_id:application_id\", got %q.", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func mapWebsocketApplicationToState(ctx context.Context, wsApp *client.WebsocketApplicationData, state *WebsocketApplicationResourceModel) {
	state.ID = types.StringValue(wsApp.ID)
	state.Name = types.StringValue(wsApp.Attributes.Name)
	state.AppID = types.StringValue(wsApp.Attributes.AppID)
	// key and secret are merged into the response conditionally -- in practice
	// only when the application is created. A later GET omits them, so writing
	// them unconditionally replaced the stored credentials with empty strings,
	// silently and with no error, breaking anything referencing .secret.
	state.Key = preserveWebsocketCredential(state.Key, wsApp.Attributes.Key)
	state.Secret = preserveWebsocketCredential(state.Secret, wsApp.Attributes.Secret)
	state.PingInterval = types.Int64Value(int64(wsApp.Attributes.PingInterval))
	state.ActivityTimeout = types.Int64Value(int64(wsApp.Attributes.ActivityTimeout))
	state.MaxMessageSize = types.Int64Value(int64(wsApp.Attributes.MaxMessageSize))
	state.MaxConnections = types.Int64Value(int64(wsApp.Attributes.MaxConnections))
	state.CreatedAt = types.StringPointerValue(wsApp.Attributes.CreatedAt)
	origins, diags := types.ListValueFrom(ctx, types.StringType, nonNilStrings(wsApp.Attributes.AllowedOrigins))
	if !diags.HasError() {
		state.AllowedOrigins = origins
	}
}

// preserveWebsocketCredential keeps an already-stored key or secret when the
// API does not return one. The credentials are only present in the create
// response; treating their absence as an empty value would discard them.
func preserveWebsocketCredential(current types.String, returned string) types.String {
	if returned != "" {
		return types.StringValue(returned)
	}
	if !current.IsNull() && !current.IsUnknown() && current.ValueString() != "" {
		return current
	}
	return types.StringNull()
}

// nonNilStrings normalises a nil slice to an empty one so that "no origins"
// round-trips as an empty list rather than null.
func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
