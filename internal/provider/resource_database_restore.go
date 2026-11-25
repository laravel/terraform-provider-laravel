package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var _ resource.Resource = &DatabaseRestoreResource{}

type DatabaseRestoreResource struct {
	client *client.Client
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
	ForceDestroy         types.Bool   `tfsdk:"force_destroy"`
}

func NewDatabaseRestoreResource() resource.Resource {
	return &DatabaseRestoreResource{}
}

func (r *DatabaseRestoreResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_database_restore"
}

func (r *DatabaseRestoreResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Restores a Laravel Cloud database cluster from a snapshot or point-in-time, into a new cluster. " +
			"The API gives a restored Serverless Postgres cluster the same ID as its source, so every request for it " +
			"reaches the source: destroying such a restore only removes it from state, and the restored cluster has " +
			"to be deleted in the Laravel Cloud dashboard.",
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
			"force_destroy": schema.BoolAttribute{
				Optional: true,
				Description: "Allow destroying the restored cluster even though it still contains " +
					"databases. A restore copies the source cluster's databases into the new cluster, " +
					"and the API refuses to delete a cluster while any database is attached, so " +
					"destroying is usually impossible without this. Setting it true DELETES EVERY " +
					"DATABASE IN THE RESTORED CLUSTER, including ones Terraform does not manage. " +
					"Defaults to false, in which case destroy fails and names the databases that are in the way.",
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

func (r *DatabaseRestoreResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	// The source cluster refuses restores while it is busy.
	var restored *client.DatabaseClusterData
	err := client.RetryOnConflict(ctx, databaseBusyAttempts, databaseBusyInterval, func() error {
		var err error
		restored, err = r.client.CreateDatabaseRestore(ctx, plan.DatabaseClusterID.ValueString(), createReq)
		return err
	}, client.IsBusy)
	if err != nil {
		resp.Diagnostics.AddError("Error restoring database", err.Error())
		return
	}

	plan.ID = types.StringValue(restored.ID)
	if sharesSourceID(plan) {
		resp.Diagnostics.AddWarning("Restored cluster cannot be managed through the API", sharedSourceIDDetail)
	}
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

	// A restore that shares its source's ID cannot be read: the request
	// would return the source, and record its name and connection details,
	// so anything using this restore's hostname would reach the source. What
	// the restore's create response recorded stays as it is.
	if sharesSourceID(state) {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
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

// Update only ever changes force_destroy: every other argument forces a new
// restore. Nothing is sent to the API.
func (r *DatabaseRestoreResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state DatabaseRestoreResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ForceDestroy = plan.ForceDestroy
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DatabaseRestoreResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DatabaseRestoreResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if sharesSourceID(state) {
		// Removed from state without a request, which would reach the source.
		resp.Diagnostics.AddWarning("Restored cluster was not deleted", sharedSourceIDDetail+
			" Delete it in the Laravel Cloud dashboard.")
		return
	}

	// The restored cluster is an ordinary cluster holding copies of the
	// source's databases, so it is deleted the same way.
	resp.Diagnostics.Append(deleteDatabaseCluster(ctx, r.client, state.ID.ValueString(), state.ForceDestroy.ValueBool())...)
}

// sharesSourceID reports whether the API gave the restored cluster the same ID
// as the cluster it was restored from, which it does for Serverless Postgres:
// both keep the Neon project's ID. The API looks clusters up by that ID and
// takes the first match, the source, so every request for the restored
// cluster -- deleting it, too -- acts on the source instead.
func sharesSourceID(m DatabaseRestoreResourceModel) bool {
	return m.ID.ValueString() == m.DatabaseClusterID.ValueString()
}

const sharedSourceIDDetail = "The API gave the restored cluster the same ID as the cluster it was restored from, " +
	"as it does for every Serverless Postgres restore, so any request for it reaches the source cluster instead. " +
	"Terraform does not delete it, because that would delete the source and its databases."
