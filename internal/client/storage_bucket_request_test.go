package client

import (
	"encoding/json"
	"testing"
)

// TestUpdateStorageBucketRequestCORSStates pins the bucket PATCH body against
// what the live API actually honours, which is narrower than it looks:
//
//   - cors_settings is MERGED into the bucket's current rules, so a field that
//     is absent is left alone and an emptied list only takes effect when it
//     arrives as an explicit [].
//   - cors_settings: null and cors_settings: {} are both accepted with 200 and
//     silently ignored, so neither can be used to clear anything.
//   - allowed_methods: [] is rejected with 422 "At least one method must be
//     provided", so CORS can never be removed outright.
//
// The plain-slice fields this struct used to carry were dropped by omitempty
// exactly when they were emptied, which is why clearing origins did nothing.
func TestUpdateStorageBucketRequestCORSStates(t *testing.T) {
	tests := []struct {
		name string
		req  UpdateStorageBucketRequest
		want string
	}{
		{
			name: "unchanged omits the CORS fields entirely",
			req:  UpdateStorageBucketRequest{Name: strPtr("bucket")},
			want: `{"name":"bucket"}`,
		},
		{
			name: "emptied origins survive omitempty as an explicit []",
			req:  UpdateStorageBucketRequest{CorsSettings: &CorsSettings{AllowedOrigins: &[]string{}}},
			want: `{"cors_settings":{"allowed_origins":[]}}`,
		},
		{
			name: "an unset sub-field stays absent so the merge leaves it alone",
			req: UpdateStorageBucketRequest{CorsSettings: &CorsSettings{
				AllowedOrigins: &[]string{"https://example.com"},
			}},
			want: `{"cors_settings":{"allowed_origins":["https://example.com"]}}`,
		},
		{
			name: "emptied expose_headers is sent, not dropped",
			req: UpdateStorageBucketRequest{CorsSettings: &CorsSettings{
				AllowedMethods: &[]string{"GET"},
				ExposeHeaders:  &[]string{},
			}},
			want: `{"cors_settings":{"allowed_methods":["GET"],"expose_headers":[]}}`,
		},
		{
			name: "emptied deprecated allowed_origins alias is sent as []",
			req:  UpdateStorageBucketRequest{AllowedOrigins: &[]string{}},
			want: `{"allowed_origins":[]}`,
		},
		{
			name: "untouched allowed_origins alias is omitted",
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
