package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ resource.Resource = &DatabaseRestoreResource{}

type DatabaseRestoreResource struct {
	resourceWithClient
}

type DatabaseRestoreResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	DatabaseClusterID    types.String `tfsdk:"database_cluster_id"`
	Name                 types.String `tfsdk:"name"`
	RestoreTime          types.String `tfsdk:"restore_time"`
	DatabaseSnapshotID   types.String `tfsdk:"database_snapshot_id"`
	RestoredClusterName  types.String `tfsdk:"restored_cluster_name"`
	RestoredClusterType  types.String `tfsdk:"restored_cluster_type"`
	RestoredClusterState types.String `tfsdk:"restored_cluster_status"`
	Region               types.String `tfsdk:"region"`
	Config               types.String `tfsdk:"config"`
	Connection           types.Object `tfsdk:"connection_details"`
}

func NewDatabaseRestoreResource() resource.Resource {
	return &DatabaseRestoreResource{}
}

func (r *DatabaseRestoreResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_database_restore"
}

func (r *DatabaseRestoreResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Restores a Laravel Cloud database cluster from a snapshot or point-in-time.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "ID of the restored database cluster.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"database_cluster_id": schema.StringAttribute{
				Required:    true,
				Description: "Source database cluster ID to restore from.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name for the restored database cluster.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"restore_time": schema.StringAttribute{
				Optional:    true,
				Description: "Point-in-time to restore to (ISO 8601). Mutually exclusive with database_snapshot_id.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"database_snapshot_id": schema.StringAttribute{
				Optional:    true,
				Description: "Snapshot ID to restore from. Mutually exclusive with restore_time.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"restored_cluster_name": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the restored cluster.",
			},
			"restored_cluster_type": schema.StringAttribute{
				Computed:    true,
				Description: "Type of the restored cluster.",
			},
			"restored_cluster_status": schema.StringAttribute{
				Computed:    true,
				Description: "Status of the restored cluster.",
			},
			"region": schema.StringAttribute{
				Computed:    true,
				Description: "Region of the restored cluster.",
			},
			"config": schema.StringAttribute{
				Computed:    true,
				Description: "JSON-encoded configuration of the restored cluster.",
			},
			"connection_details": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Read-only connection details for the restored cluster.",
				Attributes: map[string]schema.Attribute{
					"hostname": schema.StringAttribute{Computed: true},
					"port":     schema.Int64Attribute{Computed: true},
					"protocol": schema.StringAttribute{Computed: true},
					"driver":   schema.StringAttribute{Computed: true},
					"username": schema.StringAttribute{Computed: true},
					"password": schema.StringAttribute{Computed: true, Sensitive: true},
				},
			},
		},
	}
}

func (r *DatabaseRestoreResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DatabaseRestoreResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateDatabaseRestoreRequest{
		Name: plan.Name.ValueString(),
	}
	if !plan.RestoreTime.IsNull() {
		v := plan.RestoreTime.ValueString()
		createReq.RestoreTime = &v
	}
	if !plan.DatabaseSnapshotID.IsNull() {
		v := plan.DatabaseSnapshotID.ValueString()
		createReq.DatabaseSnapshotID = &v
	}

	restored, err := r.client.CreateDatabaseRestore(ctx, plan.DatabaseClusterID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error restoring database", err.Error())
		return
	}

	plan.ID = types.StringValue(restored.ID)
	plan.RestoredClusterName = types.StringValue(restored.Attributes.Name)
	plan.RestoredClusterType = types.StringValue(restored.Attributes.DBType)
	plan.RestoredClusterState = types.StringValue(restored.Attributes.Status)
	plan.Region = types.StringValue(restored.Attributes.Region)
	plan.Connection = mapDatabaseConnection(restored.Attributes.Connection)
	if restored.Attributes.Config != nil {
		cfgJSON, _ := json.Marshal(restored.Attributes.Config)
		plan.Config = types.StringValue(string(cfgJSON))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatabaseRestoreResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DatabaseRestoreResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The restored cluster is a regular database cluster; read it directly.
	cluster, err := r.client.GetDatabaseCluster(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading restored database cluster", err.Error())
		return
	}

	state.RestoredClusterName = types.StringValue(cluster.Attributes.Name)
	state.RestoredClusterType = types.StringValue(cluster.Attributes.DBType)
	state.RestoredClusterState = types.StringValue(cluster.Attributes.Status)
	state.Region = types.StringValue(cluster.Attributes.Region)
	state.Connection = mapDatabaseConnection(cluster.Attributes.Connection)
	if cluster.Attributes.Config != nil {
		cfgJSON, _ := json.Marshal(cluster.Attributes.Config)
		state.Config = types.StringValue(string(cfgJSON))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DatabaseRestoreResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "Database restores are immutable. Create a new restore instead.")
}

func (r *DatabaseRestoreResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DatabaseRestoreResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the restored cluster.
	if err := r.client.DeleteDatabaseCluster(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting restored database cluster", err.Error())
	}
}
