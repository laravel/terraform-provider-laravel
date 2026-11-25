package provider

import (
	"slices"
	"testing"

	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// TestWithoutSharedIDs: a Serverless Postgres restore shares its source's ID,
// so a sweep must skip both, whichever of them has the test prefix. Otherwise
// sweeping a test restore deletes the source's databases and the source.
func TestWithoutSharedIDs(t *testing.T) {
	cluster := func(id, name string) client.DatabaseClusterData {
		return client.DatabaseClusterData{ID: id, Attributes: client.DatabaseClusterAttributes{Name: name}}
	}
	got := withoutSharedIDs([]client.DatabaseClusterData{
		cluster("db-1", "production"),
		cluster("db-1", testAccPrefix+"restore"),
		cluster("db-2", testAccPrefix+"cluster"),
		cluster("db-3", "staging"),
	})
	var names []string
	for _, c := range got {
		names = append(names, c.Attributes.Name)
	}
	if want := []string{testAccPrefix + "cluster", "staging"}; !slices.Equal(names, want) {
		t.Errorf("clusters left to sweep = %v, want %v", names, want)
	}
}
