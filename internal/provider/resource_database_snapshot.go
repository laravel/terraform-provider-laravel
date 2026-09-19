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
	_ resource.Resource                = &DatabaseSnapshotResource{}
	_ resource.ResourceWithImportState = &DatabaseSnapshotResource{}
)

type DatabaseSnapshotResource struct {
	resourceWithClient
}

type DatabaseSnapshotResourceModel struct {
	ID           types.String `tfsdk:"id"`
	ClusterID    types.String `tfsdk:"cluster_id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	SnapshotType types.String `tfsdk:"type"`
	Status       types.String `tfsdk:"status"`
	StorageBytes types.Int64  `tfsdk:"storage_bytes"`
	PITREnabled  types.Bool   `tfsdk:"pitr_enabled"`
	PITREndsAt   types.String `tfsdk:"pitr_ends_at"`
	CompletedAt  types.String `tfsdk:"completed_at"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func NewDatabaseSnapshotResource() resource.Resource {
	return &DatabaseSnapshotResource{}
}

func (r *DatabaseSnapshotResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_database_snapshot"
}

func (r *DatabaseSnapshotResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a snapshot of a Laravel Cloud database cluster.",
		Attributes: map[string]schema.Attribute{
			"id": computedIDAttribute(),
			"cluster_id": schema.StringAttribute{
				Required:    true,
				Description: "Database cluster ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Snapshot name (1-100 characters).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Snapshot description (3-40 characters).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Computed:    true,
				Description: "Snapshot type.",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Snapshot status.",
			},
			"storage_bytes": schema.Int64Attribute{
				Computed:    true,
				Description: "Storage size in bytes.",
			},
			"pitr_enabled": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether point-in-time recovery is enabled for this snapshot.",
			},
			"pitr_ends_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when point-in-time recovery coverage ends.",
			},
			"completed_at": schema.StringAttribute{
				Computed:    true,
				Description: "Completion timestamp.",
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *DatabaseSnapshotResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DatabaseSnapshotResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateDatabaseSnapshotRequest{
		Name: plan.Name.ValueString(),
	}
	if !plan.Description.IsNull() {
		v := plan.Description.ValueString()
		createReq.Description = &v
	}

	snapshot, err := r.client.CreateDatabaseSnapshot(ctx, plan.ClusterID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating database snapshot", err.Error())
		return
	}

	mapDatabaseSnapshotToState(snapshot, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatabaseSnapshotResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DatabaseSnapshotResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	snapshot, err := r.client.GetDatabaseSnapshot(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading database snapshot", err.Error())
		return
	}

	mapDatabaseSnapshotToState(snapshot, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DatabaseSnapshotResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "Database snapshots are immutable. Delete and re-create to change.")
}

func (r *DatabaseSnapshotResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DatabaseSnapshotResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteDatabaseSnapshot(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting database snapshot", err.Error())
	}
}

func (r *DatabaseSnapshotResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mapDatabaseSnapshotToState(s *client.DatabaseSnapshotData, state *DatabaseSnapshotResourceModel) {
	// cluster_id is Required and forces replacement; recover it from the
	// relationship so an imported snapshot is not destroyed on the next plan.
	if clusterID := s.Relationships.Database.RelatedID(); clusterID != "" {
		state.ClusterID = types.StringValue(clusterID)
	}
	state.ID = types.StringValue(s.ID)
	state.Name = types.StringPointerValue(s.Attributes.Name)
	state.SnapshotType = types.StringValue(s.Attributes.SnapshotType)
	state.Status = types.StringValue(s.Attributes.Status)
	state.CreatedAt = types.StringPointerValue(s.Attributes.CreatedAt)
	state.CompletedAt = types.StringPointerValue(s.Attributes.CompletedAt)
	state.PITREnabled = types.BoolValue(s.Attributes.PITREnabled)
	state.PITREndsAt = types.StringPointerValue(s.Attributes.PITREndsAt)
	if s.Attributes.Description != nil {
		state.Description = types.StringValue(*s.Attributes.Description)
	}
	// storage_bytes is Computed and nullable, and a freshly created snapshot is
	// still pending with no size yet. Assigning it only when non-nil left the
	// attribute unknown after apply, which Terraform rejects outright with
	// "provider produced inconsistent result after apply" -- so it is always
	// set, to null when the API has no size to report.
	state.StorageBytes = types.Int64PointerValue(s.Attributes.StorageBytes)
}
