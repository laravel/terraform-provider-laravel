package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                = &DatabaseResource{}
	_ resource.ResourceWithImportState = &DatabaseResource{}
)

type DatabaseResource struct {
	client *client.Client
}

type DatabaseResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ClusterID types.String `tfsdk:"cluster_id"`
	Name      types.String `tfsdk:"name"`
	Status    types.String `tfsdk:"status"`
	CreatedAt types.String `tfsdk:"created_at"`
}

func NewDatabaseResource() resource.Resource {
	return &DatabaseResource{}
}

func (r *DatabaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_database"
}

func (r *DatabaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a database (schema) within a Laravel Cloud database cluster.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cluster_id": schema.StringAttribute{
				Required:    true,
				Description: "Database cluster ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Database name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Database status.",
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *DatabaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DatabaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DatabaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := plan.ClusterID.ValueString()
	wantName := plan.Name.ValueString()

	// Laravel Cloud auto-creates a default database when a cluster is
	// provisioned. Check whether a database with the requested name
	// already exists and adopt it instead of creating a duplicate.
	var db *client.DatabaseData
	if existing, err := r.client.ListDatabases(ctx, clusterID); err == nil {
		if db = findDatabaseByName(existing, wantName); db != nil {
			tflog.Info(ctx, "Adopting existing database",
				map[string]any{"database_id": db.ID, "name": wantName})
		}
	}

	// If no match was found, create a new one with retries.
	if db == nil {
		createReq := client.CreateDatabaseRequest{Name: wantName}
		err := client.RetryOnConflict(ctx, 30, 10*time.Second, func() error {
			// The create response carries the *cluster* id, not the new schema
			// id, so its return value is intentionally discarded — the canonical
			// id is resolved from the cluster listing below (issue #30).
			_, createErr := r.client.CreateDatabase(ctx, clusterID, createReq)
			return createErr
		}, func(err error) bool {
			return strings.Contains(err.Error(), "update operation is already in progress")
		})
		if err != nil {
			resp.Diagnostics.AddError("Error creating database", err.Error())
			return
		}

		// Resolve the authoritative schema id by name from the cluster listing.
		list, err := r.client.ListDatabases(ctx, clusterID)
		if err != nil {
			resp.Diagnostics.AddError("Error creating database", err.Error())
			return
		}
		if db = findDatabaseByName(list, wantName); db == nil {
			resp.Diagnostics.AddError("Error creating database",
				fmt.Sprintf("database %q was not found in cluster %s after creation", wantName, clusterID))
			return
		}
	}

	plan.ID = types.StringValue(db.ID)
	plan.Status = types.StringValue(db.Attributes.Status)
	plan.CreatedAt = types.StringPointerValue(db.Attributes.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// findDatabaseByName returns the database in list whose name matches, or nil.
func findDatabaseByName(list []client.DatabaseData, name string) *client.DatabaseData {
	for i := range list {
		if list[i].Attributes.Name == name {
			return &list[i]
		}
	}
	return nil
}

func (r *DatabaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DatabaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The cluster-nested route is the real per-database route, and state carries
	// both ids (import takes "<cluster_id>:<database_id>"), so it is always
	// constructible. A 404 here means either the database or its cluster is gone.
	db, err := r.client.GetDatabase(ctx, state.ClusterID.ValueString(), state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading database", err.Error())
		return
	}

	state.Name = types.StringValue(db.Attributes.Name)
	state.Status = types.StringValue(db.Attributes.Status)
	state.CreatedAt = types.StringPointerValue(db.Attributes.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DatabaseResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "Databases are immutable. Delete and re-create to change.")
}

func (r *DatabaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DatabaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The cluster may have an operation in progress. Retry for up to 5 minutes.
	clusterID := state.ClusterID.ValueString()
	id := state.ID.ValueString()
	err := client.RetryOnConflict(ctx, 30, 10*time.Second, func() error {
		err := r.client.DeleteDatabase(ctx, clusterID, id)
		if client.IsNotFound(err) {
			return nil // Already gone — nothing to do.
		}
		return err
	}, func(err error) bool {
		return strings.Contains(err.Error(), "update operation is already in progress")
	})
	if err != nil {
		resp.Diagnostics.AddError("Error deleting database", err.Error())
	}
}

func (r *DatabaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Databases are addressed through the cluster-nested routes (Read and
	// Delete both need the cluster id), so import as "<cluster_id>:<database_id>".
	clusterID, id, found := strings.Cut(req.ID, ":")
	if !found || clusterID == "" || id == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"cluster_id:database_id\", got %q.", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_id"), clusterID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
