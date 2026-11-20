package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// Laravel Cloud refuses to create an environment on a branch missing from its
// own copy of the repository's branch list, with a 422 "The selected branch is
// no longer available." That copy is filled by a queued job, so a
// non-default branch of a newly connected repository is missing for a short
// while. The provider retries for up to two minutes.
const branchUnavailableAttempts = 24

var branchUnavailableInterval = 5 * time.Second

// appRepositories holds each application's configured repository ("owner/name")
// by application ID, as the application resource last created, read or updated
// it in this provider process.
//
// Deleting the last application on a repository deletes Cloud's repository
// record, and an application being created on the same repository at that
// moment is left with no repository at all. Every environment create on it then
// fails with the same 422, however long it waits, until the repository is set
// again. The environment resource does not know the repository, so it takes it
// from here.
var appRepositories sync.Map

func rememberAppRepository(appID string, repository types.String) {
	if appID != "" && !repository.IsNull() && !repository.IsUnknown() {
		appRepositories.Store(appID, repository.ValueString())
	}
}

// createEnvironment creates an environment, waiting out a branch Laravel Cloud
// has not synced yet and re-attaching the application's repository if Cloud
// dropped it. Any other error is returned at once.
func createEnvironment(ctx context.Context, c *client.Client, appID string, req client.CreateEnvironmentRequest) (*client.EnvironmentData, error) {
	var env *client.EnvironmentData
	var lostRepository, restored bool
	err := client.RetryOnConflict(ctx, branchUnavailableAttempts, branchUnavailableInterval, func() error {
		var err error
		env, err = c.CreateEnvironment(ctx, appID, req)
		return err
	}, func(err error) bool {
		if !client.IsBranchUnavailable(err) {
			return false
		}
		if restored {
			return true
		}
		app, getErr := c.GetApplication(ctx, appID)
		if getErr != nil {
			return false
		}
		if app.Attributes.Repository != nil {
			return true // The branch is not synced yet.
		}
		repository, ok := appRepositories.Load(appID)
		if !ok {
			lostRepository = true
			return false
		}
		name := repository.(string)
		if _, err := c.UpdateApplication(ctx, appID, client.UpdateApplicationRequest{Repository: &name}); err != nil {
			lostRepository = true
			return false
		}
		restored = true
		return true
	})
	if err != nil && lostRepository {
		return nil, fmt.Errorf("application %s has no repository: Laravel Cloud deletes its record of a repository "+
			"when the last application on it is deleted, and an application created on the same repository at that "+
			"moment is left without one. Set the application's repository again (the laravel_cloud_application "+
			"resource does so on the next apply) and retry. %w", appID, err)
	}
	return env, err
}
