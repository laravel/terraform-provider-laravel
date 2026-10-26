package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// After upgrading from a version without location_hint or root_directory, the
// prior state has them null. With `apply -refresh=false` nothing fills them in
// before an update, so the plan carries that null; the update must keep it
// rather than adopt the value the API reports, which Terraform would reject
// as an inconsistent result. The next refresh records the real value.
//
// The plan tests always refresh first, so these call Update directly with the
// state such an upgrade leaves behind.

func updateWith[M any](t *testing.T, r resource.Resource, state, plan M) M {
	t.Helper()
	ctx := context.Background()
	var sch resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sch)

	st := tfsdk.State{Schema: sch.Schema, Raw: tftypes.NewValue(sch.Schema.Type().TerraformType(ctx), nil)}
	if d := st.Set(ctx, &state); d.HasError() {
		t.Fatalf("building state: %v", d)
	}
	pl := tfsdk.Plan{Schema: sch.Schema, Raw: tftypes.NewValue(sch.Schema.Type().TerraformType(ctx), nil)}
	if d := pl.Set(ctx, &plan); d.HasError() {
		t.Fatalf("building plan: %v", d)
	}

	resp := resource.UpdateResponse{State: tfsdk.State{Schema: sch.Schema, Raw: pl.Raw}}
	r.Update(ctx, resource.UpdateRequest{State: st, Plan: pl}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("update failed: %v", resp.Diagnostics)
	}
	var out M
	if d := resp.State.Get(ctx, &out); d.HasError() {
		t.Fatalf("reading state: %v", d)
	}
	return out
}

func TestStorageBucketUpdateKeepsUnrefreshedLocationHint(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	c := client.NewClient("test-token", client.WithBaseURL(baseURL))
	bucket, err := c.CreateStorageBucket(context.Background(), client.CreateStorageBucketRequest{
		Name: "assets", Visibility: "private", Jurisdiction: "default", KeyName: "primary", KeyPermission: "read_write",
	})
	if err != nil {
		t.Fatal(err)
	}
	if bucket.Attributes.LocationHint == nil {
		t.Fatal("the fake should report a location hint")
	}

	state := StorageBucketResourceModel{
		ID: types.StringValue(bucket.ID), Name: types.StringValue("assets"),
		Visibility: types.StringValue("private"), Jurisdiction: types.StringValue("default"),
		LocationHint: types.StringNull(), // written by a version without the attribute
		KeyName:      types.StringValue("primary"), KeyPermission: types.StringValue("read_write"),
		AllowedOrigins: types.ListNull(types.StringType),
		Endpoint:       types.StringNull(), URL: types.StringNull(),
		Status: types.StringValue("available"), CreatedAt: types.StringNull(),
	}
	plan := state
	plan.Visibility = types.StringValue("public")

	r := &StorageBucketResource{client: c}
	got := updateWith(t, r, state, plan)
	if !got.LocationHint.IsNull() {
		t.Fatalf("location_hint = %s, want the planned null", got.LocationHint)
	}
}

func TestApplicationUpdateKeepsUnrefreshedRootDirectory(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	c := client.NewClient("test-token", client.WithBaseURL(baseURL))
	app, err := c.CreateApplication(context.Background(), client.CreateApplicationRequest{
		Name: "monorepo", Repository: "acme/monorepo", Region: "us-east-2", RootDirectory: ptr("apps/web"),
	})
	if err != nil {
		t.Fatal(err)
	}

	state := ApplicationResourceModel{
		ID: types.StringValue(app.ID), Name: types.StringValue("monorepo"),
		Repository: types.StringValue("acme/monorepo"), Region: types.StringValue("us-east-2"),
		Slug: types.StringValue(app.Attributes.Slug), ClusterID: types.StringNull(),
		SourceControlProviderType: types.StringNull(), SlackChannel: types.StringNull(),
		AvatarURL: types.StringNull(), CreatedAt: types.StringNull(),
		StarterKit:    types.StringNull(),
		RootDirectory: types.StringNull(), // written by a version without the attribute
	}
	plan := state
	plan.Name = types.StringValue("monorepo-renamed")

	r := &ApplicationResource{client: c}
	got := updateWith(t, r, state, plan)
	if !got.RootDirectory.IsNull() {
		t.Fatalf("root_directory = %s, want the planned null", got.RootDirectory)
	}
}
