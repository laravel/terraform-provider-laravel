package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

func strptr(s string) *string { return &s }

// The API returns a bucket key's credentials when the key is created, but
// GET /bucket-keys/{id} may answer null for both -- the spec types them
// string|null. They were decoded as plain strings, so a null became "" and the
// first refresh overwrote the stored secret with an empty string. Both
// attributes are Computed, so that produced no diff and no error: the secret
// simply disappeared and every reference to it silently became "".
func TestPreserveCredential(t *testing.T) {
	cases := []struct {
		name     string
		current  types.String
		returned *string
		want     types.String
	}{
		{
			name:     "a returned value wins",
			current:  types.StringValue("old-secret"),
			returned: strptr("new-secret"),
			want:     types.StringValue("new-secret"),
		},
		{
			name:     "a null response keeps what is already in state",
			current:  types.StringValue("stored-secret"),
			returned: nil,
			want:     types.StringValue("stored-secret"),
		},
		{
			name:     "an unknown prior value collapses to null, never staying unknown",
			current:  types.StringUnknown(),
			returned: nil,
			want:     types.StringNull(),
		},
		{
			name:     "a fresh create takes the created credential",
			current:  types.StringUnknown(),
			returned: strptr("created-secret"),
			want:     types.StringValue("created-secret"),
		},
		{
			name:     "an explicit empty string from the API is still a value",
			current:  types.StringValue("stored-secret"),
			returned: strptr(""),
			want:     types.StringValue(""),
		},
		{
			name:     "a null response over null state stays null",
			current:  types.StringNull(),
			returned: nil,
			want:     types.StringNull(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := preserveCredential(tc.current, tc.returned)
			if !got.Equal(tc.want) {
				t.Errorf("preserveCredential(%v, %v) = %v, want %v",
					tc.current, tc.returned, got, tc.want)
			}
		})
	}
}

// The whole mapper must not clobber credentials on a refresh whose response
// omits them, while still refreshing the fields the API does return.
func TestMapStorageBucketKeyToStateKeepsCredentialsOnRefresh(t *testing.T) {
	state := StorageBucketKeyResourceModel{
		ID:              types.StringValue("flsk-1"),
		BucketID:        types.StringValue("fls-1"),
		Name:            types.StringValue("old-name"),
		Permission:      types.StringValue("read_only"),
		AccessKeyID:     types.StringValue("AKIASTORED"),
		AccessKeySecret: types.StringValue("stored-secret"),
	}

	// A read that renames the key and returns no credentials.
	mapStorageBucketKeyToState(&client.StorageBucketKeyData{
		ID: "flsk-1",
		Attributes: client.StorageBucketKeyAttributes{
			Name:       "new-name",
			Permission: "read_write",
		},
	}, &state)

	if got := state.AccessKeyID.ValueString(); got != "AKIASTORED" {
		t.Errorf("access_key_id = %q, want the stored value to survive", got)
	}
	if got := state.AccessKeySecret.ValueString(); got != "stored-secret" {
		t.Errorf("access_key_secret = %q, want the stored value to survive", got)
	}
	if got := state.Name.ValueString(); got != "new-name" {
		t.Errorf("name = %q, want it refreshed to %q", got, "new-name")
	}
	if got := state.Permission.ValueString(); got != "read_write" {
		t.Errorf("permission = %q, want it refreshed to %q", got, "read_write")
	}
}
