package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                = &DatabaseClusterResource{}
	_ resource.ResourceWithImportState = &DatabaseClusterResource{}
)

type DatabaseClusterResource struct {
	client *client.Client
}

type DatabaseClusterResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Type         types.String `tfsdk:"type"`
	Region       types.String `tfsdk:"region"`
	Status       types.String `tfsdk:"status"`
	ClusterID    types.String `tfsdk:"cluster_id"`
	Version      types.String `tfsdk:"version"`
	Config       types.String `tfsdk:"config"`
	Connection   types.Object `tfsdk:"connection_details"`
	CreatedAt    types.String `tfsdk:"created_at"`
	ForceDestroy types.Bool   `tfsdk:"force_destroy"`
}

// databaseConnectionAttrTypes describes the connection_details object shared by
// the database cluster and database restore resources. It is modeled as a
// types.Object (not a Go pointer-struct) so the framework can represent the
// value as unknown during the create plan — a pointer-struct cannot hold
// unknown, which otherwise fails Create's Plan.Get.
var databaseConnectionAttrTypes = map[string]attr.Type{
	"hostname": types.StringType,
	"port":     types.Int64Type,
	"protocol": types.StringType,
	"driver":   types.StringType,
	"username": types.StringType,
	"password": types.StringType,
}

func mapDatabaseConnection(c *client.DatabaseConnection) types.Object {
	if c == nil {
		return types.ObjectNull(databaseConnectionAttrTypes)
	}
	obj, _ := types.ObjectValue(databaseConnectionAttrTypes, map[string]attr.Value{
		"hostname": types.StringValue(c.Hostname),
		"port":     types.Int64Value(int64(c.Port)),
		"protocol": types.StringValue(c.Protocol),
		"driver":   types.StringValue(c.Driver),
		"username": types.StringValue(c.Username),
		"password": types.StringValue(c.Password),
	})
	return obj
}

func NewDatabaseClusterResource() resource.Resource {
	return &DatabaseClusterResource{}
}

func (r *DatabaseClusterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_database_cluster"
}

func (r *DatabaseClusterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Laravel Cloud database cluster.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Cluster name (3-40 characters, lowercase alphanumeric).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Database type (e.g. laravel_mysql_84, aws_rds_mysql_8, neon_serverless_postgres_18, etc.).",
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
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Cluster status.",
			},
			"cluster_id": schema.StringAttribute{
				Optional: true,
				Description: "Dedicated cluster ID. Changing this forces a new cluster: " +
					"the API's update endpoint accepts config and nothing else.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"version": schema.StringAttribute{
				Optional: true,
				Description: "Database engine version (see the versions attribute of the " +
					"laravel_cloud_database_types data source). Required by the API for the " +
					"current type identifiers such as \"laravel_mysql\"; omit it only when " +
					"using a retired identifier that bakes the version into the type, " +
					"such as \"laravel_mysql_84\". Create-only: the API never reports it " +
					"back, so an imported cluster leaves it null.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"force_destroy": schema.BoolAttribute{
				Optional: true,
				Description: "Allow destroying this cluster even though it still contains " +
					"databases. The API refuses to delete a cluster while any database is " +
					"attached, and a cluster always carries at least the one the platform " +
					"creates with it, so destroying is impossible without this. Setting it " +
					"true DELETES EVERY DATABASE IN THE CLUSTER, including ones Terraform " +
					"did not create and does not manage. Defaults to false, in which case " +
					"destroy fails and names the databases that are in the way.",
			},
			"config": schema.StringAttribute{
				Required:    true,
				Description: "JSON-encoded configuration specific to the database type (required by the API).",
			},
			"connection_details": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Read-only connection details for the cluster.",
				Attributes: map[string]schema.Attribute{
					"hostname": schema.StringAttribute{Computed: true},
					"port":     schema.Int64Attribute{Computed: true},
					"protocol": schema.StringAttribute{Computed: true},
					"driver":   schema.StringAttribute{Computed: true},
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

func (r *DatabaseClusterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DatabaseClusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DatabaseClusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateDatabaseClusterRequest{
		Type:    plan.Type.ValueString(),
		Version: plan.Version.ValueString(),
		Name:    plan.Name.ValueString(),
		Region:  plan.Region.ValueString(),
	}
	if !plan.ClusterID.IsNull() {
		v := plan.ClusterID.ValueString()
		createReq.ClusterID = &v
	}
	if !plan.Config.IsNull() {
		var cfg map[string]any
		if err := json.Unmarshal([]byte(plan.Config.ValueString()), &cfg); err != nil {
			resp.Diagnostics.AddError("Invalid config JSON", err.Error())
			return
		}
		createReq.Config = cfg
	}

	cluster, err := r.client.CreateDatabaseCluster(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating database cluster", err.Error())
		return
	}

	plan.ID = types.StringValue(cluster.ID)
	plan.Status = types.StringValue(cluster.Attributes.Status)
	plan.Connection = mapDatabaseConnection(cluster.Attributes.Connection)
	plan.CreatedAt = types.StringPointerValue(cluster.Attributes.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatabaseClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DatabaseClusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cluster, err := r.client.GetDatabaseCluster(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading database cluster", err.Error())
		return
	}

	state.Name = types.StringValue(cluster.Attributes.Name)
	state.Type = reconcileDatabaseType(state.Type, cluster.Attributes.DBType)
	state.Region = types.StringValue(cluster.Attributes.Region)
	state.Status = types.StringValue(cluster.Attributes.Status)
	state.Connection = mapDatabaseConnection(cluster.Attributes.Connection)
	state.CreatedAt = types.StringPointerValue(cluster.Attributes.CreatedAt)

	state.Config = reconcileDatabaseConfig(state.Config, cluster.Attributes.Config)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DatabaseClusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DatabaseClusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state DatabaseClusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := client.UpdateDatabaseClusterRequest{}
	if !plan.Config.IsNull() {
		var cfg map[string]any
		if err := json.Unmarshal([]byte(plan.Config.ValueString()), &cfg); err != nil {
			resp.Diagnostics.AddError("Invalid config JSON", err.Error())
			return
		}
		updateReq.Config = cfg
	}

	cluster, err := r.client.UpdateDatabaseCluster(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating database cluster", err.Error())
		return
	}

	plan.ID = state.ID
	plan.Status = types.StringValue(cluster.Attributes.Status)
	plan.Connection = mapDatabaseConnection(cluster.Attributes.Connection)
	plan.CreatedAt = types.StringPointerValue(cluster.Attributes.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatabaseClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DatabaseClusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()

	// A cluster always carries at least the database the platform creates
	// alongside it, and the API refuses to delete a cluster while any database
	// is attached -- so a destroy cannot succeed without removing them first.
	//
	// Removing them is destructive well beyond what Terraform manages: the
	// cluster may also hold databases created in the dashboard or by another
	// tool, and this resource's state does not track them. Doing it implicitly
	// would turn a failed destroy, which is recoverable, into silent data loss,
	// which is not -- and lifecycle.prevent_destroy could not guard it, because
	// those databases are not resources in state. So it is opt-in, on the same
	// reasoning as force_destroy on a storage bucket.
	if !state.ForceDestroy.ValueBool() {
		names, err := clusterDatabaseNames(ctx, r.client, id)
		if err != nil {
			resp.Diagnostics.AddError("Error listing databases before deleting cluster", err.Error())
			return
		}
		if len(names) > 0 {
			resp.Diagnostics.AddError(
				"Database cluster still contains databases",
				fmt.Sprintf(
					"The cluster %q cannot be deleted because it still contains %d database(s): %s.\n\n"+
						"The API refuses to delete a cluster with databases attached, and a cluster always "+
						"carries at least the one created with it. Either remove them first, or set "+
						"force_destroy = true on this resource to have Terraform delete every database in "+
						"the cluster as part of the destroy.\n\n"+
						"force_destroy deletes ALL of them, including any this configuration does not manage.",
					id, len(names), strings.Join(names, ", "),
				),
			)
			return
		}
	} else if diags := deleteClusterSchemas(ctx, r.client, id); diags != nil {
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// The cluster may still be settling after the schema deletions.
	err := client.RetryOnConflict(ctx, databaseDeleteAttempts, databaseDeleteInterval, func() error {
		return r.client.DeleteDatabaseCluster(ctx, id)
	}, func(err error) bool {
		msg := err.Error()
		return strings.Contains(msg, "schemas attached") ||
			strings.Contains(msg, "operation is already in progress") ||
			strings.Contains(msg, "update operation is already in progress")
	})
	if err != nil {
		resp.Diagnostics.AddError("Error deleting database cluster", err.Error())
	}
}

// deleteClusterSchemas removes every schema in the cluster so the cluster
// itself can be deleted. A schema that is already gone is not an error; a
// cluster that is already gone means there is nothing to do.
func deleteClusterSchemas(ctx context.Context, c *client.Client, clusterID string) diag.Diagnostics {
	var diags diag.Diagnostics

	schemas, err := c.ListDatabases(ctx, clusterID)
	if err != nil {
		if client.IsNotFound(err) {
			return nil
		}
		diags.AddError(
			"Error listing databases before deleting cluster",
			"The cluster's databases must be removed before the cluster can be deleted, "+
				"but listing them failed: "+err.Error(),
		)
		return diags
	}

	for _, schema := range schemas {
		schemaID := schema.ID
		err := client.RetryOnConflict(ctx, databaseDeleteAttempts, databaseDeleteInterval, func() error {
			if err := c.DeleteDatabase(ctx, clusterID, schemaID); err != nil {
				if client.IsNotFound(err) {
					return nil
				}
				return err
			}
			return nil
		}, func(err error) bool {
			return strings.Contains(err.Error(), "operation is already in progress")
		})
		if err != nil {
			diags.AddError(
				"Error deleting database before deleting cluster",
				fmt.Sprintf("Could not delete database %q in cluster %q: %s", schemaID, clusterID, err),
			)
			return diags
		}
	}
	return diags
}

func (r *DatabaseClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// reconcileDatabaseType keeps the configured database type when the API answers
// with the equivalent base identifier.
//
// The retired type identifiers (laravel_mysql_84, aws_rds_mysql_8,
// neon_serverless_postgres_18, ...) bake the engine version into the type and
// are still accepted on create, but the API always reports the *base* type
// back: DatabaseType is enum ["laravel_mysql", "aws_rds_mysql",
// "aws_rds_postgres", "neon_serverless_postgres"]. Writing that base value into
// state made it differ from the configured value on every read, and because
// type carries RequiresReplace the next plan proposed destroying and recreating
// a live database cluster.
//
// When the configured value is the returned value plus a version suffix, the
// two denote the same type and the configured spelling is preserved.
func reconcileDatabaseType(configured types.String, returned string) types.String {
	if configured.IsNull() || configured.IsUnknown() || returned == "" {
		return types.StringValue(returned)
	}
	if strings.HasPrefix(configured.ValueString(), returned+"_") {
		return configured
	}
	return types.StringValue(returned)
}

// reconcileDatabaseConfig keeps the configured config JSON when the API has
// merely added its own defaults to it.
//
// config is a Required, user-authored JSON string, but the API echoes back the
// full effective configuration -- including keys the caller never set
// (storage_autoscale_max_gb, suspend_seconds and so on). Writing that whole
// object into state made every subsequent plan propose deleting those keys, a
// diff that could never converge because the API always adds them back.
//
// Only the keys the user actually specified are compared. When all of them
// still match, the configured string is preserved verbatim; when any has
// drifted, the drifted values are written back so the change is visible.
func reconcileDatabaseConfig(configured types.String, apiConfig map[string]any) types.String {
	if apiConfig == nil {
		return configured
	}
	if configured.IsNull() || configured.IsUnknown() {
		b, err := json.Marshal(apiConfig)
		if err != nil {
			return configured
		}
		return types.StringValue(string(b))
	}

	var want map[string]any
	if err := json.Unmarshal([]byte(configured.ValueString()), &want); err != nil {
		// Not an object we can reason about; fall back to the API's view.
		b, err := json.Marshal(apiConfig)
		if err != nil {
			return configured
		}
		return types.StringValue(string(b))
	}

	drifted := false
	merged := make(map[string]any, len(want))
	for k, v := range want {
		actual, present := apiConfig[k]
		if !present {
			merged[k] = v
			continue
		}
		merged[k] = actual
		if !jsonEqual(v, actual) {
			drifted = true
		}
	}
	if !drifted {
		return configured
	}
	b, err := json.Marshal(merged)
	if err != nil {
		return configured
	}
	return types.StringValue(string(b))
}

// jsonEqual compares two decoded JSON values. It exists because numbers decode
// as float64 on one side and may arrive as int on the other.
func jsonEqual(a, b any) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ab) == string(bb)
}

// Deleting anything in a database cluster fails while the cluster is still
// provisioning, and provisioning a cluster routinely takes longer than ten
// minutes. The previous five-minute budget was shorter than the operation it
// was waiting on, so a create-then-destroy cycle -- an acceptance test, or a
// user correcting a mistake -- gave up and left a billable database behind.
const (
	databaseDeleteAttempts = 120
	databaseDeleteInterval = 10 * time.Second
)

// clusterDatabaseNames returns the names of every database in the cluster, so a
// refused destroy can say what is actually in the way. A cluster that is
// already gone has nothing in it.
func clusterDatabaseNames(ctx context.Context, c *client.Client, clusterID string) ([]string, error) {
	schemas, err := c.ListDatabases(ctx, clusterID)
	if err != nil {
		if client.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(schemas))
	for _, s := range schemas {
		name := s.Attributes.Name
		if name == "" {
			name = s.ID
		}
		names = append(names, name)
	}
	return names, nil
}
