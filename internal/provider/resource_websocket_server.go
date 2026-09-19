package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                = &WebsocketServerResource{}
	_ resource.ResourceWithImportState = &WebsocketServerResource{}
)

type WebsocketServerResource struct {
	resourceWithClient
}

type WebsocketServerResourceModel struct {
	ID                             types.String `tfsdk:"id"`
	Name                           types.String `tfsdk:"name"`
	Type                           types.String `tfsdk:"type"`
	Region                         types.String `tfsdk:"region"`
	MaxConnections                 types.Int64  `tfsdk:"max_connections"`
	Status                         types.String `tfsdk:"status"`
	Hostname                       types.String `tfsdk:"hostname"`
	ConnectionDistributionStrategy types.String `tfsdk:"connection_distribution_strategy"`
	CreatedAt                      types.String `tfsdk:"created_at"`
}

func NewWebsocketServerResource() resource.Resource {
	return &WebsocketServerResource{}
}

func (r *WebsocketServerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_websocket_server"
}

func (r *WebsocketServerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Laravel Cloud WebSocket server (cluster).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Server name.",
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Server type (e.g. reverb).",
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
			"max_connections": schema.Int64Attribute{
				Required:    true,
				Description: "Maximum concurrent connections.",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Server status.",
			},
			"hostname": schema.StringAttribute{
				Computed:    true,
				Description: "Server hostname.",
			},
			"connection_distribution_strategy": schema.StringAttribute{
				Computed:    true,
				Description: "Strategy used to distribute connections across the server.",
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *WebsocketServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan WebsocketServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateWebsocketServerRequest{
		Name:           plan.Name.ValueString(),
		Type:           plan.Type.ValueString(),
		Region:         plan.Region.ValueString(),
		MaxConnections: int(plan.MaxConnections.ValueInt64()),
	}

	ws, err := r.client.CreateWebsocketServer(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating websocket server", err.Error())
		return
	}

	mapWebsocketServerToState(ws, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WebsocketServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state WebsocketServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ws, err := r.client.GetWebsocketServer(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading websocket server", err.Error())
		return
	}

	mapWebsocketServerToState(ws, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *WebsocketServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan WebsocketServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state WebsocketServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := client.UpdateWebsocketServerRequest{}
	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		updateReq.Name = &v
	}
	if !plan.MaxConnections.Equal(state.MaxConnections) {
		v := int(plan.MaxConnections.ValueInt64())
		updateReq.MaxConnections = &v
	}

	ws, err := r.client.UpdateWebsocketServer(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating websocket server", err.Error())
		return
	}

	mapWebsocketServerToState(ws, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WebsocketServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state WebsocketServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteWebsocketServer(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting websocket server", err.Error())
	}
}

func (r *WebsocketServerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mapWebsocketServerToState(ws *client.WebsocketServerData, state *WebsocketServerResourceModel) {
	state.ID = types.StringValue(ws.ID)
	state.Name = types.StringValue(ws.Attributes.Name)
	state.Type = types.StringValue(ws.Attributes.ServerType)
	state.Region = types.StringValue(ws.Attributes.Region)
	state.MaxConnections = types.Int64Value(int64(ws.Attributes.MaxConnections))
	state.Status = types.StringValue(ws.Attributes.Status)
	state.Hostname = types.StringValue(ws.Attributes.Hostname)
	state.ConnectionDistributionStrategy = types.StringValue(ws.Attributes.ConnectionDistributionStrategy)
	state.CreatedAt = types.StringPointerValue(ws.Attributes.CreatedAt)
}
