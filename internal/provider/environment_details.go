package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// This file maps the environment's read-only details (display name,
// framework, language, hibernation, preview pull request) and its post-deploy
// command.

const (
	postDeployIgnoredSummary     = "Laravel Cloud did not apply post_deploy_command"
	postDeployUnavailableSummary = "Laravel Cloud will not apply post_deploy_command"
	postDeployIgnoredDetail      = "Laravel Cloud ignores post_deploy_command for this environment and does not report one. " +
		"Post-deploy commands are available only to organizations that have them enabled, and not on " +
		"preview environments. Remove post_deploy_command from the configuration, or ask Laravel Cloud support to " +
		"enable post-deploy commands for the organization."
)

var pullRequestAttrTypes = map[string]attr.Type{
	"number":        types.Int64Type,
	"url":           types.StringType,
	"title":         types.StringType,
	"source_branch": types.StringType,
	"target_branch": types.StringType,
	"status":        types.StringType,
	"draft":         types.BoolType,
}

func pullRequestFromAPI(pr *client.EnvironmentPullRequest) types.Object {
	if pr == nil {
		return types.ObjectNull(pullRequestAttrTypes)
	}
	return types.ObjectValueMust(pullRequestAttrTypes, map[string]attr.Value{
		"number":        types.Int64Value(pr.Number),
		"url":           types.StringPointerValue(pr.URL),
		"title":         types.StringPointerValue(pr.Title),
		"source_branch": types.StringPointerValue(pr.SourceBranch),
		"target_branch": types.StringPointerValue(pr.TargetBranch),
		"status":        types.StringValue(pr.Status),
		"draft":         types.BoolValue(pr.Draft),
	})
}

// mapDetailsToState copies the details from an API response. A post-deploy
// command the API does not report is null: the organization cannot use one,
// so a configured value shows as a change in the next plan.
func mapDetailsToState(env *client.EnvironmentData, state *EnvironmentResourceModel) {
	a := env.Attributes
	state.PostDeployCommand = types.StringPointerValue(a.PostDeployCommand.Value)
	state.DisplayName = types.StringPointerValue(a.DisplayName)
	state.Framework = types.StringPointerValue(a.Framework)
	state.Language = types.StringPointerValue(a.Language)
	state.UsesHibernation = types.BoolPointerValue(a.UsesHibernation)
	state.CreatedFromAutomation = types.BoolValue(a.CreatedFromAutomation)
	state.PullRequest = pullRequestFromAPI(a.PullRequest)
	state.ClusterID = types.StringPointerValue(a.ClusterID)
}

// setUnknownDetailsFromAPI resolves the details the plan left unknown.
func setUnknownDetailsFromAPI(plan *EnvironmentResourceModel, env *client.EnvironmentData) {
	var api EnvironmentResourceModel
	mapDetailsToState(env, &api)
	keepUnknownDetails(plan, api)
}

// keepUnknownDetails resolves the details the plan left unknown to src's.
func keepUnknownDetails(plan *EnvironmentResourceModel, src EnvironmentResourceModel) {
	for _, f := range []struct{ plan, src *types.String }{
		{&plan.PostDeployCommand, &src.PostDeployCommand},
		{&plan.DisplayName, &src.DisplayName},
		{&plan.Framework, &src.Framework},
		{&plan.Language, &src.Language},
		{&plan.ClusterID, &src.ClusterID},
	} {
		if f.plan.IsUnknown() {
			*f.plan = *f.src
		}
	}
	if plan.UsesHibernation.IsUnknown() {
		plan.UsesHibernation = src.UsesHibernation
	}
	if plan.CreatedFromAutomation.IsUnknown() {
		plan.CreatedFromAutomation = src.CreatedFromAutomation
	}
	if plan.PullRequest.IsUnknown() {
		plan.PullRequest = src.PullRequest
	}
}

// resolvedNull is v, or null while it is unknown.
func resolvedNull(v types.String) types.String {
	if v.IsUnknown() {
		return types.StringNull()
	}
	return v
}

// postDeployUnavailableKey is the private state key Read sets when Laravel
// Cloud does not report a post-deploy command for the environment, which means
// it ignores one: the organization does not have post-deploy commands, or the
// environment is a preview.
const postDeployUnavailableKey = "post_deploy_unavailable"

func setPostDeployUnavailable(ctx context.Context, p privateSetter, env *client.EnvironmentData) diag.Diagnostics {
	if env.Attributes.PostDeployCommand.Reported {
		return p.SetKey(ctx, postDeployUnavailableKey, nil)
	}
	return p.SetKey(ctx, postDeployUnavailableKey, []byte("true"))
}

// postDeployAvailable warns in the plan when post_deploy_command is configured
// for an environment that Laravel Cloud last reported without one: Cloud would
// ignore it and answer 200, so the apply fails. It is not an error here,
// because that would also block destroying the environment.
type postDeployAvailable struct{}

func (postDeployAvailable) Description(context.Context) string {
	return "Warns when Laravel Cloud does not offer post-deploy commands for the environment."
}

func (m postDeployAvailable) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (postDeployAvailable) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !isSet(req.ConfigValue) || req.Private == nil || req.ConfigValue.Equal(req.StateValue) {
		return
	}
	b, d := req.Private.GetKey(ctx, postDeployUnavailableKey)
	resp.Diagnostics.Append(d...)
	if len(b) > 0 {
		resp.Diagnostics.AddAttributeWarning(req.Path, postDeployUnavailableSummary, postDeployIgnoredDetail)
	}
}
