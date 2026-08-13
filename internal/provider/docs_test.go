package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// The docs under docs/ are the provider's published reference — the release
// workflow uploads that directory verbatim, and there is no `go generate` step,
// so nothing keeps them in step with the schemas. A documented attribute that
// does not exist sends users to write config the provider rejects; an
// undocumented one is invisible. Both have shipped before.
//
// These tests compare each schema's top-level attributes and blocks against the
// tfplugindocs sections in its markdown page. Nested schemas are not walked;
// that is a deliberate limit, not an assertion that they are correct.

// docAttrRe matches a tfplugindocs attribute bullet: - `name` (Type) Description
var docAttrRe = regexp.MustCompile("^- `([a-z0-9_]+)` \\(([^)]+)\\)")

// docSectionRe matches the section headings tfplugindocs emits.
var docSectionRe = regexp.MustCompile(`^### (Required|Optional|Read-Only)\s*$`)

// nestedSectionRe matches the start of a nested schema block, after which
// bullets describe a nested object rather than the top level.
var nestedSectionRe = regexp.MustCompile(`^### Nested Schema for`)

// docCategory is where tfplugindocs files an attribute.
type docCategory string

const (
	catRequired docCategory = "Required"
	catOptional docCategory = "Optional"
	catReadOnly docCategory = "Read-Only"
)

// parseDocAttributes reads the top-level attribute names and their sections out
// of a tfplugindocs page. Bullets after the first "Nested Schema for" heading
// belong to nested objects and are skipped.
func parseDocAttributes(t *testing.T, path string) map[string]docCategory {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	out := map[string]docCategory{}
	var section docCategory
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		if nestedSectionRe.MatchString(line) {
			break
		}
		if m := docSectionRe.FindStringSubmatch(line); m != nil {
			section = docCategory(m[1])
			continue
		}
		if section == "" {
			continue
		}
		if m := docAttrRe.FindStringSubmatch(line); m != nil {
			out[m[1]] = section
		}
	}
	return out
}

// wantCategory is the section an attribute belongs in, from its schema flags.
func wantCategory(required, optional bool) docCategory {
	switch {
	case required:
		return catRequired
	case optional:
		return catOptional
	default:
		return catReadOnly
	}
}

// compare reports drift between a schema's attributes and its doc page.
func compare(t *testing.T, typeName, docPath string, schemaAttrs map[string]docCategory) {
	t.Helper()

	if _, err := os.Stat(docPath); err != nil {
		t.Errorf("%s: no documentation page at %s", typeName, docPath)
		return
	}
	documented := parseDocAttributes(t, docPath)

	var undocumented, phantom, miscategorised []string

	for name, want := range schemaAttrs {
		got, ok := documented[name]
		if !ok {
			undocumented = append(undocumented, name)
			continue
		}
		if got != want {
			miscategorised = append(miscategorised,
				fmt.Sprintf("%s (documented %s, schema says %s)", name, got, want))
		}
	}
	for name := range documented {
		if _, ok := schemaAttrs[name]; !ok {
			phantom = append(phantom, name)
		}
	}

	sort.Strings(undocumented)
	sort.Strings(phantom)
	sort.Strings(miscategorised)

	rel := filepath.Base(docPath)
	if len(undocumented) > 0 {
		t.Errorf("%s: %s does not document schema attributes: %s",
			typeName, rel, strings.Join(undocumented, ", "))
	}
	if len(phantom) > 0 {
		t.Errorf("%s: %s documents attributes that do not exist: %s",
			typeName, rel, strings.Join(phantom, ", "))
	}
	if len(miscategorised) > 0 {
		t.Errorf("%s: %s files attributes in the wrong section: %s",
			typeName, rel, strings.Join(miscategorised, ", "))
	}
}

func TestResourceDocsMatchSchemas(t *testing.T) {
	ctx := context.Background()
	p := New("test")().(*LaravelCloudProvider)

	for _, newResource := range p.Resources(ctx) {
		r := newResource()

		var meta resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "laravel"}, &meta)

		var resp resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &resp)
		if resp.Diagnostics.HasError() {
			continue // covered by TestResourceSchemasValid
		}

		attrs := map[string]docCategory{}
		for name, a := range resp.Schema.Attributes {
			attrs[name] = wantCategory(a.IsRequired(), a.IsOptional())
		}
		for name, b := range resp.Schema.Blocks {
			// Blocks have no Required/Optional flags; tfplugindocs files a
			// block with no required nested attributes under Optional.
			_ = b
			attrs[name] = catOptional
		}

		// laravel_cloud_application -> docs/resources/cloud_application.md
		docPath := filepath.Join("..", "..", "docs", "resources",
			strings.TrimPrefix(meta.TypeName, "laravel_")+".md")

		t.Run(meta.TypeName, func(t *testing.T) {
			compare(t, meta.TypeName, docPath, attrs)
		})
	}
}

func TestDataSourceDocsMatchSchemas(t *testing.T) {
	ctx := context.Background()
	p := New("test")().(*LaravelCloudProvider)

	for _, newDataSource := range p.DataSources(ctx) {
		d := newDataSource()

		var meta datasource.MetadataResponse
		d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "laravel"}, &meta)

		var resp datasource.SchemaResponse
		d.Schema(ctx, datasource.SchemaRequest{}, &resp)
		if resp.Diagnostics.HasError() {
			continue // covered by TestDataSourceSchemasValid
		}

		attrs := map[string]docCategory{}
		for name, a := range resp.Schema.Attributes {
			attrs[name] = wantCategory(a.IsRequired(), a.IsOptional())
		}
		for name := range resp.Schema.Blocks {
			attrs[name] = catOptional
		}

		docPath := filepath.Join("..", "..", "docs", "data-sources",
			strings.TrimPrefix(meta.TypeName, "laravel_")+".md")

		t.Run(meta.TypeName, func(t *testing.T) {
			compare(t, meta.TypeName, docPath, attrs)
		})
	}
}

// Keep the unused-import guards honest if the framework schema types are ever
// needed directly here.
var (
	_ = rschema.StringAttribute{}
	_ = dschema.StringAttribute{}
)
