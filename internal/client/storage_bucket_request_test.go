package client

import (
	"encoding/json"
	"testing"
)

// TestUpdateStorageBucketRequestCORSStates pins the three things a bucket PATCH
// body has to be able to say about CORS.
//
// The field used to be a *CorsSettings with omitempty, which can only express
// "absent" and "an object". Removing the cors_settings block from a
// configuration therefore sent nothing at all: Terraform recorded the rules as
// gone while the bucket kept serving them, and no later apply could take them
// off, because a cleared value always marshalled to the same empty body.
func TestUpdateStorageBucketRequestCORSStates(t *testing.T) {
	origins := []string{"https://example.com"}

	tests := []struct {
		name string
		req  UpdateStorageBucketRequest
		want string
	}{
		{
			name: "unchanged omits both CORS fields",
			req:  UpdateStorageBucketRequest{Name: strPtr("bucket")},
			want: `{"name":"bucket"}`,
		},
		{
			name: "cleared sends an explicit null",
			req:  UpdateStorageBucketRequest{CorsSettings: ClearJSON()},
			want: `{"cors_settings":null}`,
		},
		{
			name: "set sends the object",
			req:  UpdateStorageBucketRequest{CorsSettings: JSONValue(&CorsSettings{AllowedOrigins: origins})},
			want: `{"cors_settings":{"allowed_origins":["https://example.com"]}}`,
		},
		{
			name: "emptied allowed_origins sends an empty list, not nothing",
			req:  UpdateStorageBucketRequest{AllowedOrigins: &[]string{}},
			want: `{"allowed_origins":[]}`,
		},
		{
			name: "untouched allowed_origins is omitted",
			req:  UpdateStorageBucketRequest{AllowedOrigins: nil},
			want: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatalf("marshaling request: %v", err)
			}
			if string(b) != tt.want {
				t.Errorf("got  %s\nwant %s", b, tt.want)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
