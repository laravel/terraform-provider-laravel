package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// This file maps the environment settings that sit outside the original
// attribute set: serving, start command, hibernation wake-ups, Nightwatch,
// bucket attachments, response headers and firewall toggles.
//
// Readable settings follow the same rules as the rest of the resource: Read
// overwrites them from the API, and a setting the configuration leaves unset
// takes whatever the platform reports, so upgrading the provider adopts values
// set in the dashboard instead of planning to reset them. nightwatch_token and
// filesystem_keys are never reported back, so they keep their configured value.

// isSet reports whether a planned value was configured, as opposed to null or
// still unknown because an Optional+Computed attribute was left unset.
func isSet(v interface {
	IsNull() bool
	IsUnknown() bool
}) bool {
	return !v.IsNull() && !v.IsUnknown()
}

func servingModeFromAPI(env *client.EnvironmentData) types.String {
	if env.Attributes.Serving == nil || env.Attributes.Serving.Mode == "" {
		return types.StringNull()
	}
	return types.StringValue(env.Attributes.Serving.Mode)
}

func servingRootFromAPI(env *client.EnvironmentData) types.String {
	if env.Attributes.Serving == nil {
		return types.StringNull()
	}
	return types.StringPointerValue(env.Attributes.Serving.Root)
}

// wakeUpIntervalFromAPI maps the API's interval onto the attribute. Null means
// periodic wake-ups are off, which the configuration spells 0, so a prior 0 is
// kept rather than turned into null and reported as a change.
func wakeUpIntervalFromAPI(api *int64, prior types.Int64) types.Int64 {
	if api == nil {
		if isSet(prior) && prior.ValueInt64() == 0 {
			return prior
		}
		return types.Int64Null()
	}
	return types.Int64Value(*api)
}

func hstsFromAPI(env *client.EnvironmentData) (maxAge types.Int64, includeSubdomains, preload types.Bool) {
	h := env.Attributes.NetworkSettings.ResponseHeaders.HSTS
	if h == nil {
		return types.Int64Null(), types.BoolNull(), types.BoolNull()
	}
	return types.Int64PointerValue(h.MaxAge), types.BoolValue(h.IncludeSubdomains), types.BoolValue(h.Preload)
}

// mapSettingsToState copies the readable settings from an API response.
func mapSettingsToState(env *client.EnvironmentData, state *EnvironmentResourceModel) {
	a := env.Attributes
	state.ServingMode = servingModeFromAPI(env)
	state.ServingRoot = servingRootFromAPI(env)
	state.StartCommand = types.StringPointerValue(a.StartCommand)
	state.HibernationWakeUpInterval = wakeUpIntervalFromAPI(a.HibernationWakeUpInterval, state.HibernationWakeUpInterval)
	rh := a.NetworkSettings.ResponseHeaders
	state.ResponseHeadersFrame = types.StringPointerValue(rh.Frame)
	state.ResponseHeadersContentType = types.StringPointerValue(rh.ContentType)
	state.ResponseHeadersRobotsTag = types.StringPointerValue(rh.RobotsTag)
	state.ResponseHeadersHSTSMaxAge, state.ResponseHeadersHSTSIncludeSubdomains, state.ResponseHeadersHSTSPreload = hstsFromAPI(env)
	state.FirewallBlockPath = types.BoolPointerValue(a.NetworkSettings.Firewall.BlockPath)
	state.FirewallBrowserIntegrityCheck = types.BoolPointerValue(a.NetworkSettings.Firewall.BrowserIntegrityCheck)
}

// setUnknownSettingsFromAPI resolves the readable settings the configuration
// left unset to what a freshly created (or adopted) environment reports.
func setUnknownSettingsFromAPI(plan *EnvironmentResourceModel, env *client.EnvironmentData) {
	var api EnvironmentResourceModel
	mapSettingsToState(env, &api)
	for _, f := range []struct{ plan, api *types.String }{
		{&plan.ServingMode, &api.ServingMode},
		{&plan.ServingRoot, &api.ServingRoot},
		{&plan.StartCommand, &api.StartCommand},
		{&plan.ResponseHeadersFrame, &api.ResponseHeadersFrame},
		{&plan.ResponseHeadersContentType, &api.ResponseHeadersContentType},
		{&plan.ResponseHeadersRobotsTag, &api.ResponseHeadersRobotsTag},
	} {
		if f.plan.IsUnknown() {
			*f.plan = *f.api
		}
	}
	for _, f := range []struct{ plan, api *types.Int64 }{
		{&plan.HibernationWakeUpInterval, &api.HibernationWakeUpInterval},
		{&plan.ResponseHeadersHSTSMaxAge, &api.ResponseHeadersHSTSMaxAge},
	} {
		if f.plan.IsUnknown() {
			*f.plan = *f.api
		}
	}
	for _, f := range []struct{ plan, api *types.Bool }{
		{&plan.ResponseHeadersHSTSIncludeSubdomains, &api.ResponseHeadersHSTSIncludeSubdomains},
		{&plan.ResponseHeadersHSTSPreload, &api.ResponseHeadersHSTSPreload},
		{&plan.FirewallBlockPath, &api.FirewallBlockPath},
		{&plan.FirewallBrowserIntegrityCheck, &api.FirewallBrowserIntegrityCheck},
	} {
		if f.plan.IsUnknown() {
			*f.plan = *f.api
		}
	}
}

// wakeUpIntervalRequest renders the interval for the PATCH body: 0 (or null)
// turns periodic wake-ups off, which the API spells null.
func wakeUpIntervalRequest(v types.Int64) *client.NullableInt {
	if v.IsNull() || v.ValueInt64() == 0 {
		return &client.NullableInt{Null: true}
	}
	return &client.NullableInt{Value: int(v.ValueInt64())}
}

func nullableStringRequest(v types.String) *client.NullableString {
	return &client.NullableString{Value: v.ValueString(), Null: v.IsNull()}
}

// filesystemKeysRequest renders the bucket attachments. A nil list (the
// attribute removed) becomes an empty one, which detaches every bucket.
func filesystemKeysRequest(keys []FilesystemKeyModel) *[]client.FilesystemKeyAttachment {
	out := make([]client.FilesystemKeyAttachment, 0, len(keys))
	for _, k := range keys {
		out = append(out, client.FilesystemKeyAttachment{
			ID:            k.KeyID.ValueString(),
			Disk:          k.Disk.ValueString(),
			IsDefaultDisk: k.IsDefaultDisk.ValueBool(),
		})
	}
	return &out
}

func filesystemKeysEqual(a, b []FilesystemKeyModel) bool {
	if (a == nil) != (b == nil) || len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].KeyID.Equal(b[i].KeyID) || !a[i].Disk.Equal(b[i].Disk) || !a[i].IsDefaultDisk.Equal(b[i].IsDefaultDisk) {
			return false
		}
	}
	return true
}

// hstsRequest renders the HSTS setting. The API takes its three fields only
// together, so all three are always sent.
func hstsRequest(maxAge types.Int64, includeSubdomains, preload types.Bool) *client.HSTSRequest {
	h := &client.HSTSRequest{
		IncludeSubdomains: includeSubdomains.ValueBool(),
		Preload:           preload.ValueBool(),
	}
	if maxAge.IsNull() || maxAge.IsUnknown() {
		h.MaxAge = client.NullableInt{Null: true}
	} else {
		h.MaxAge = client.NullableInt{Value: int(maxAge.ValueInt64())}
	}
	return h
}

// servingRequest renders a change of served directory. The mode cannot change
// after creation and must be sent as the current one.
func servingRequest(mode string, root types.String) *client.ServingRequest {
	if mode == "" {
		mode = "process"
	}
	return &client.ServingRequest{Mode: mode, Root: *nullableStringRequest(root)}
}

// addSettingsFromStateDiff adds every changed setting to an update request and
// reports whether it added any.
func addSettingsFromStateDiff(req *client.UpdateEnvironmentRequest, plan, state EnvironmentResourceModel) bool {
	changed := false
	if !plan.ServingRoot.Equal(state.ServingRoot) {
		req.Serving = servingRequest(state.ServingMode.ValueString(), plan.ServingRoot)
		changed = true
	}
	if !plan.StartCommand.Equal(state.StartCommand) {
		req.StartCommand = nullableStringRequest(plan.StartCommand)
		changed = true
	}
	if !plan.HibernationWakeUpInterval.Equal(state.HibernationWakeUpInterval) {
		req.HibernationWakeUpInterval = wakeUpIntervalRequest(plan.HibernationWakeUpInterval)
		changed = true
	}
	if !plan.NightwatchToken.Equal(state.NightwatchToken) {
		req.NightwatchToken = nullableStringRequest(plan.NightwatchToken)
		changed = true
	}
	if !filesystemKeysEqual(plan.FilesystemKeys, state.FilesystemKeys) {
		req.FilesystemKeys = filesystemKeysRequest(plan.FilesystemKeys)
		changed = true
	}
	for _, f := range []struct {
		plan, state types.String
		dst         **string
	}{
		{plan.ResponseHeadersFrame, state.ResponseHeadersFrame, &req.ResponseHeadersFrame},
		{plan.ResponseHeadersContentType, state.ResponseHeadersContentType, &req.ResponseHeadersContentType},
		{plan.ResponseHeadersRobotsTag, state.ResponseHeadersRobotsTag, &req.ResponseHeadersRobotsTag},
	} {
		if isSet(f.plan) && !f.plan.Equal(f.state) {
			*f.dst = f.plan.ValueStringPointer()
			changed = true
		}
	}
	if !plan.ResponseHeadersHSTSMaxAge.Equal(state.ResponseHeadersHSTSMaxAge) ||
		!plan.ResponseHeadersHSTSIncludeSubdomains.Equal(state.ResponseHeadersHSTSIncludeSubdomains) ||
		!plan.ResponseHeadersHSTSPreload.Equal(state.ResponseHeadersHSTSPreload) {
		req.ResponseHeadersHSTS = hstsRequest(plan.ResponseHeadersHSTSMaxAge, plan.ResponseHeadersHSTSIncludeSubdomains, plan.ResponseHeadersHSTSPreload)
		changed = true
	}
	for _, f := range []struct {
		plan, state types.Bool
		dst         **bool
	}{
		{plan.FirewallBlockPath, state.FirewallBlockPath, &req.FirewallBlockPath},
		{plan.FirewallBrowserIntegrityCheck, state.FirewallBrowserIntegrityCheck, &req.FirewallBrowserIntegrityCheck},
	} {
		if isSet(f.plan) && !f.plan.Equal(f.state) {
			*f.dst = f.plan.ValueBoolPointer()
			changed = true
		}
	}
	return changed
}

// addSettingsFromCreateDiff adds every configured setting that differs from a
// freshly created (or adopted) environment to the follow-up update request.
// plan is the plan as configured, before unknowns were resolved, so an
// Optional+Computed setting the user left unset is unknown and not sent.
func addSettingsFromCreateDiff(req *client.UpdateEnvironmentRequest, plan EnvironmentResourceModel, env *client.EnvironmentData) bool {
	var api EnvironmentResourceModel
	mapSettingsToState(env, &api)
	changed := false

	if isSet(plan.ServingRoot) && !plan.ServingRoot.Equal(api.ServingRoot) {
		req.Serving = servingRequest(api.ServingMode.ValueString(), plan.ServingRoot)
		changed = true
	}
	if isSet(plan.StartCommand) && !plan.StartCommand.Equal(api.StartCommand) {
		req.StartCommand = nullableStringRequest(plan.StartCommand)
		changed = true
	}
	if isSet(plan.HibernationWakeUpInterval) && !plan.HibernationWakeUpInterval.Equal(wakeUpIntervalFromAPI(env.Attributes.HibernationWakeUpInterval, plan.HibernationWakeUpInterval)) {
		req.HibernationWakeUpInterval = wakeUpIntervalRequest(plan.HibernationWakeUpInterval)
		changed = true
	}
	// Never reported, so sent whenever configured.
	if isSet(plan.NightwatchToken) {
		req.NightwatchToken = nullableStringRequest(plan.NightwatchToken)
		changed = true
	}
	if plan.FilesystemKeys != nil {
		req.FilesystemKeys = filesystemKeysRequest(plan.FilesystemKeys)
		changed = true
	}
	for _, f := range []struct {
		plan, api types.String
		dst       **string
	}{
		{plan.ResponseHeadersFrame, api.ResponseHeadersFrame, &req.ResponseHeadersFrame},
		{plan.ResponseHeadersContentType, api.ResponseHeadersContentType, &req.ResponseHeadersContentType},
		{plan.ResponseHeadersRobotsTag, api.ResponseHeadersRobotsTag, &req.ResponseHeadersRobotsTag},
	} {
		if isSet(f.plan) && !f.plan.Equal(f.api) {
			*f.dst = f.plan.ValueStringPointer()
			changed = true
		}
	}
	// HSTS goes out as a whole, so a field left unset keeps the reported value.
	if isSet(plan.ResponseHeadersHSTSMaxAge) || isSet(plan.ResponseHeadersHSTSIncludeSubdomains) || isSet(plan.ResponseHeadersHSTSPreload) {
		maxAge, sub, pre := api.ResponseHeadersHSTSMaxAge, api.ResponseHeadersHSTSIncludeSubdomains, api.ResponseHeadersHSTSPreload
		if isSet(plan.ResponseHeadersHSTSMaxAge) {
			maxAge = plan.ResponseHeadersHSTSMaxAge
		}
		if isSet(plan.ResponseHeadersHSTSIncludeSubdomains) {
			sub = plan.ResponseHeadersHSTSIncludeSubdomains
		}
		if isSet(plan.ResponseHeadersHSTSPreload) {
			pre = plan.ResponseHeadersHSTSPreload
		}
		if !maxAge.Equal(api.ResponseHeadersHSTSMaxAge) || !sub.Equal(api.ResponseHeadersHSTSIncludeSubdomains) || !pre.Equal(api.ResponseHeadersHSTSPreload) {
			req.ResponseHeadersHSTS = hstsRequest(maxAge, sub, pre)
			changed = true
		}
	}
	for _, f := range []struct {
		plan, api types.Bool
		dst       **bool
	}{
		{plan.FirewallBlockPath, api.FirewallBlockPath, &req.FirewallBlockPath},
		{plan.FirewallBrowserIntegrityCheck, api.FirewallBrowserIntegrityCheck, &req.FirewallBrowserIntegrityCheck},
	} {
		if isSet(f.plan) && !f.plan.Equal(f.api) {
			*f.dst = f.plan.ValueBoolPointer()
			changed = true
		}
	}
	return changed
}
