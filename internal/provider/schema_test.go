package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// Terraform reserves these top-level names as meta-arguments; using one as a
// schema attribute or block fails provider schema validation at runtime (and
// is not caught by `go build`/`go vet`).
var reservedResourceNames = map[string]bool{
	"connection":  true,
	"count":       true,
	"depends_on":  true,
	"for_each":    true,
	"lifecycle":   true,
	"provider":    true,
	"provisioner": true,
}

var reservedDataSourceNames = map[string]bool{
	"count":      true,
	"depends_on": true,
	"for_each":   true,
	"lifecycle":  true,
	"provider":   true,
}

// TestResourceSchemasValid asserts every resource produces an error-free schema
// and never uses a reserved top-level attribute/block name.
func TestResourceSchemasValid(t *testing.T) {
	ctx := context.Background()
	p := New("test")().(*LaravelCloudProvider)

	for _, newResource := range p.Resources(ctx) {
		r := newResource()

		var meta resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "laravel"}, &meta)

		var resp resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &resp)
		if resp.Diagnostics.HasError() {
			t.Errorf("%s: schema has errors: %v", meta.TypeName, resp.Diagnostics.Errors())
			continue
		}

		for name := range resp.Schema.Attributes {
			if reservedResourceNames[name] {
				t.Errorf("%s: attribute %q is a reserved Terraform name", meta.TypeName, name)
			}
		}
		for name := range resp.Schema.Blocks {
			if reservedResourceNames[name] {
				t.Errorf("%s: block %q is a reserved Terraform name", meta.TypeName, name)
			}
		}
	}
}

// TestDataSourceSchemasValid does the same for data sources.
func TestDataSourceSchemasValid(t *testing.T) {
	ctx := context.Background()
	p := New("test")().(*LaravelCloudProvider)

	for _, newDataSource := range p.DataSources(ctx) {
		d := newDataSource()

		var meta datasource.MetadataResponse
		d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "laravel"}, &meta)

		var resp datasource.SchemaResponse
		d.Schema(ctx, datasource.SchemaRequest{}, &resp)
		if resp.Diagnostics.HasError() {
			t.Errorf("%s: schema has errors: %v", meta.TypeName, resp.Diagnostics.Errors())
			continue
		}

		for name := range resp.Schema.Attributes {
			if reservedDataSourceNames[name] {
				t.Errorf("%s: attribute %q is a reserved Terraform name", meta.TypeName, name)
			}
		}
	}
}
