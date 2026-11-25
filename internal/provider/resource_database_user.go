package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

var (
	_ resource.Resource                = &DatabaseUserResource{}
	_ resource.ResourceWithImportState = &DatabaseUserResource{}
)

type DatabaseUserResource struct {
	client *client.Client
}

type DatabaseUserResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ClusterID types.String `tfsdk:"cluster_id"`
	Username  types.String `tfsdk:"username"`
	Type      types.String `tfsdk:"type"`
	Databases types.Set    `tfsdk:"databases"`
	Password  types.String `tfsdk:"password"`
	Status    types.String `tfsdk:"status"`
	CreatedAt types.String `tfsdk:"created_at"`
}

func NewDatabaseUserResource() resource.Resource {
	return &DatabaseUserResource{}
}

func (r *DatabaseUserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_database_user"
}

func (r *DatabaseUserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a user in a Laravel Cloud database cluster, with read and write or read-only access to some of its databases. " +
			"Laravel MySQL and AWS RDS clusters support database users; Serverless Postgres does not.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Database user ID.",
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
			"username": schema.StringAttribute{
				Required: true,
				Description: "Username. Starts with a lowercase letter or underscore and contains only lowercase letters, digits and underscores; " +
					"3 to 32 characters on MySQL clusters, 3 to 63 on Postgres. It must be unused in the cluster, and engine-reserved names are refused. " +
					"Changing it replaces the user.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(3, 63),
					stringvalidator.RegexMatches(regexp.MustCompile(`^[a-z_][a-z0-9_]*$`),
						"must start with a lowercase letter or underscore and contain only lowercase letters, digits and underscores"),
				},
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "`read_write` or `read_only`. Changed in place.",
				Validators: []validator.String{
					stringvalidator.OneOf("read_write", "read_only"),
				},
			},
			"databases": schema.SetAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "IDs of the databases in the cluster the user may access, such as `laravel_cloud_database.example.id`. " +
					"An empty set grants access to none. Changed in place. Deleting one of these databases removes it from the user's access.",
			},
			"password": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
				Description: "Generated password. The API returns it only when the user is created, so it is kept in state from then on; " +
					"an imported user has none.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Status: `creating`, `available`, `failed` or `deleted`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *DatabaseUserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DatabaseUserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DatabaseUserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	databases := []string{}
	resp.Diagnostics.Append(plan.Databases.ElementsAs(ctx, &databases, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := plan.ClusterID.ValueString()
	createReq := client.CreateDatabaseUserRequest{
		Username:  plan.Username.ValueString(),
		Type:      plan.Type.ValueString(),
		Databases: databases,
	}

	// The cluster refuses new users while it is busy: still being created in
	// the same apply, or updating after a database or user changed.
	var created *client.DatabaseUserData
	err := client.RetryOnConflict(ctx, databaseBusyAttempts, databaseBusyInterval, func() error {
		var err error
		created, err = r.client.CreateDatabaseUser(ctx, clusterID, createReq)
		return err
	}, client.IsBusy)
	if err != nil {
		resp.Diagnostics.AddError("Error creating database user", err.Error())
		return
	}

	// The password is in this response only; keep it before anything else
	// can fail, so the user is never left in state without it.
	plan.ID = types.StringValue(created.ID)
	plan.Password = types.StringPointerValue(created.Attributes.Password)
	plan.Status = types.StringValue(created.Attributes.Status)
	plan.CreatedAt = types.StringPointerValue(created.Attributes.CreatedAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := waitForDatabaseUser(ctx, r.client, clusterID, created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error creating database user", err.Error())
		return
	}
	resp.Diagnostics.Append(r.setFromAPI(ctx, &plan, user)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatabaseUserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DatabaseUserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.client.GetDatabaseUser(ctx, state.ClusterID.ValueString(), state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading database user", err.Error())
		return
	}
	if user.Attributes.Status == "deleted" {
		resp.State.RemoveResource(ctx)
		return
	}
	// The cluster's own superuser is listed with the others, but the API
	// refuses to change or delete it, so Terraform cannot manage it.
	if user.Attributes.Type == "superuser" {
		resp.Diagnostics.AddError("Cannot manage the superuser",
			fmt.Sprintf("Database user %s (%s) is the cluster's superuser, which cannot be updated or deleted. Manage another user instead.",
				user.ID, user.Attributes.Username))
		return
	}

	resp.Diagnostics.Append(r.setFromAPI(ctx, &state, user)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DatabaseUserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state DatabaseUserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	databases := []string{}
	resp.Diagnostics.Append(plan.Databases.ElementsAs(ctx, &databases, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	id := state.ID.ValueString()
	// Every update replaces both the type and the databases.
	updateReq := client.UpdateDatabaseUserRequest{
		Type:      plan.Type.ValueString(),
		Databases: databases,
	}
	err := client.RetryOnConflict(ctx, databaseBusyAttempts, databaseBusyInterval, func() error {
		_, err := r.client.UpdateDatabaseUser(ctx, clusterID, id, updateReq)
		return err
	}, client.IsBusy)
	if err != nil {
		resp.Diagnostics.AddError("Error updating database user", err.Error())
		return
	}

	user, err := waitForDatabaseUser(ctx, r.client, clusterID, id)
	if err != nil {
		resp.Diagnostics.AddError("Error updating database user", err.Error())
		return
	}
	plan.ID = state.ID
	plan.Password = state.Password
	resp.Diagnostics.Append(r.setFromAPI(ctx, &plan, user)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatabaseUserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DatabaseUserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	id := state.ID.ValueString()
	err := client.RetryOnConflict(ctx, databaseBusyAttempts, databaseBusyInterval, func() error {
		err := r.client.DeleteDatabaseUser(ctx, clusterID, id)
		if client.IsNotFound(err) {
			return nil // Already gone, with the user or its cluster.
		}
		return err
	}, client.IsBusy)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting database user", err.Error())
	}
}

func (r *DatabaseUserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Every database user route is nested under its cluster.
	clusterID, id, found := strings.Cut(req.ID, ":")
	if !found || clusterID == "" || id == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"cluster_id:user_id\", got %q.", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_id"), clusterID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

// setFromAPI copies what the API reports into the model. The password is
// left alone: no response after the create one carries it.
func (r *DatabaseUserResource) setFromAPI(ctx context.Context, m *DatabaseUserResourceModel, user *client.DatabaseUserData) (diags diag.Diagnostics) {
	m.Username = types.StringValue(user.Attributes.Username)
	m.Type = types.StringValue(user.Attributes.Type)
	m.Status = types.StringValue(user.Attributes.Status)
	m.CreatedAt = types.StringPointerValue(user.Attributes.CreatedAt)
	m.Databases, diags = types.SetValueFrom(ctx, types.StringType, user.DatabaseIDs())
	return diags
}

var errDatabaseUserCreating = errors.New("database user is still being created")

// waitForDatabaseUser polls a user until the cluster has finished creating
// it, which takes about ten seconds, and fails if it could not.
func waitForDatabaseUser(ctx context.Context, c *client.Client, clusterID, id string) (*client.DatabaseUserData, error) {
	var user *client.DatabaseUserData
	err := client.RetryOnConflict(ctx, databaseBusyAttempts, databaseBusyInterval, func() error {
		var err error
		user, err = c.GetDatabaseUser(ctx, clusterID, id)
		if err != nil {
			return err
		}
		if user.Attributes.Status == "creating" {
			return errDatabaseUserCreating
		}
		return nil
	}, func(err error) bool { return errors.Is(err, errDatabaseUserCreating) })
	if err != nil {
		return nil, err
	}
	if user.Attributes.Status == "failed" {
		return nil, fmt.Errorf("database user %s failed to be created in cluster %s", id, clusterID)
	}
	return user, nil
}
