package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// An environment is running unless it was stopped: deploying and hibernating
// environments are running too.
func environmentRunning(status string) bool {
	return status != "stopped"
}

// applyRunning starts or stops the environment when plan.running differs from
// what it is now, and records its status. It reports whether that succeeded.
func applyRunning(ctx context.Context, c *client.Client, id string, plan *EnvironmentResourceModel, running bool, diags *diag.Diagnostics) bool {
	if !isSet(plan.Running) || plan.Running.ValueBool() == running {
		plan.Running = types.BoolValue(running)
		return true
	}
	var env *client.EnvironmentData
	var err error
	if plan.Running.ValueBool() {
		env, err = startEnvironment(ctx, c, id)
	} else {
		env, err = c.StopEnvironment(ctx, id)
	}
	if err != nil {
		diags.AddAttributeError(path.Root("running"), "Error starting or stopping the environment", err.Error())
		return false
	}
	plan.Status = types.StringValue(env.Attributes.Status)
	plan.Running = types.BoolValue(environmentRunning(env.Attributes.Status))
	return true
}

// modifyRunningPlan leaves status to the API when running changes.
func modifyRunningPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var planRunning, stateRunning types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("running"), &planRunning)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("running"), &stateRunning)...)
	if !resp.Diagnostics.HasError() && !planRunning.Equal(stateRunning) {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("status"), types.StringUnknown())...)
	}
}

// Starting an environment deploys it, and it reports itself running only once
// that deployment succeeds. These bound how long that is waited for. The
// interval is a variable so the plan tests need not wait.
const environmentStartAttempts = 360

var environmentStartInterval = 10 * time.Second

var errStillStarting = errors.New("environment is still starting")

// startEnvironment starts a stopped environment and waits until its
// deployment succeeds and it reports itself running.
func startEnvironment(ctx context.Context, c *client.Client, id string) (*client.EnvironmentData, error) {
	deployment, queued, err := c.StartEnvironment(ctx, id)
	if err != nil {
		return nil, err
	}
	if queued != nil {
		if deployment, err = waitForQueuedDeployment(ctx, c, queued.ID); err != nil {
			return nil, err
		}
	}
	var env *client.EnvironmentData
	err = client.RetryOnConflict(ctx, environmentStartAttempts, environmentStartInterval, func() error {
		d, err := c.GetDeployment(ctx, deployment.ID)
		if err != nil {
			return err
		}
		switch s := d.Attributes.Status; {
		case s == "cancelled" || strings.HasSuffix(s, "failed"):
			reason := s
			if d.Attributes.FailureReason != nil && *d.Attributes.FailureReason != "" {
				reason = *d.Attributes.FailureReason
			}
			return fmt.Errorf("deployment %s, which starts the environment, did not succeed: %s", d.ID, reason)
		case s != "deployment.succeeded":
			return fmt.Errorf("%w: deployment %s is %s", errStillStarting, d.ID, s)
		}
		if env, err = c.GetEnvironment(ctx, id); err != nil {
			return err
		}
		if !environmentRunning(env.Attributes.Status) {
			return fmt.Errorf("%w: it is %s", errStillStarting, env.Attributes.Status)
		}
		return nil
	}, func(err error) bool { return errors.Is(err, errStillStarting) })
	if err != nil {
		return nil, fmt.Errorf("starting environment %s: %w", id, err)
	}
	return env, nil
}
