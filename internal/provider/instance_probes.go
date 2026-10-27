package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// InstanceProbesModel is the probes attribute: the health checks of an app
// instance's web process.
type InstanceProbesModel struct {
	Startup   *InstanceProbeModel `tfsdk:"startup"`
	Readiness *InstanceProbeModel `tfsdk:"readiness"`
	Liveness  *InstanceProbeModel `tfsdk:"liveness"`
}

// InstanceProbeModel is one health check. A field left unset takes the
// platform default.
type InstanceProbeModel struct {
	Path     types.String `tfsdk:"path"`
	Port     types.Int64  `tfsdk:"port"`
	Delay    types.Int64  `tfsdk:"delay"`
	Interval types.Int64  `tfsdk:"interval"`
	Timeout  types.Int64  `tfsdk:"timeout"`
	Tries    types.Int64  `tfsdk:"tries"`
}

func instanceProbeSchema(name string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "The " + name + " check. Fields left unset take the platform default.",
		Attributes: map[string]schema.Attribute{
			"path": schema.StringAttribute{
				Optional:    true,
				Description: "HTTP path to request, starting with \"/\" (max 255 chars).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
				},
			},
			"port": schema.Int64Attribute{
				Optional:    true,
				Description: "Port to request (1-65535).",
				Validators:  []validator.Int64{int64validator.Between(1, 65535)},
			},
			"delay": schema.Int64Attribute{
				Optional:    true,
				Description: "Seconds to wait before the first check (0-600).",
				Validators:  []validator.Int64{int64validator.Between(0, 600)},
			},
			"interval": schema.Int64Attribute{
				Optional:    true,
				Description: "Seconds between checks (1-300).",
				Validators:  []validator.Int64{int64validator.Between(1, 300)},
			},
			"timeout": schema.Int64Attribute{
				Optional:    true,
				Description: "Seconds a check may take (1-120).",
				Validators:  []validator.Int64{int64validator.Between(1, 120)},
			},
			"tries": schema.Int64Attribute{
				Optional:    true,
				Description: "Failed checks before the check fails (1-1000).",
				Validators:  []validator.Int64{int64validator.Between(1, 1000)},
			},
		},
	}
}

func instanceProbesSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Description: "Custom health checks for an app instance's web process. Only app instances accept them, " +
			"and only on plans with health checks; manage one by importing the environment's app instance. " +
			"The checks configured here replace all existing ones, and a check left out uses the platform " +
			"default. When unset, Terraform leaves the instance's health checks alone, including any the " +
			"platform set up itself; removing the attribute once set resets every check to the platform default.",
		Attributes: map[string]schema.Attribute{
			"startup":   instanceProbeSchema("startup"),
			"readiness": instanceProbeSchema("readiness"),
			"liveness":  instanceProbeSchema("liveness"),
		},
	}
}

func probeRequest(m *InstanceProbeModel) *client.InstanceProbe {
	if m == nil {
		return nil
	}
	return &client.InstanceProbe{
		Path:     m.Path.ValueStringPointer(),
		Port:     m.Port.ValueInt64Pointer(),
		Delay:    m.Delay.ValueInt64Pointer(),
		Interval: m.Interval.ValueInt64Pointer(),
		Timeout:  m.Timeout.ValueInt64Pointer(),
		Tries:    m.Tries.ValueInt64Pointer(),
	}
}

// probesRequest renders the probes for an update. Nil -- the attribute
// removed -- becomes an empty object, which resets every check.
func probesRequest(m *InstanceProbesModel) *client.InstanceProbes {
	if m == nil {
		return &client.InstanceProbes{}
	}
	return &client.InstanceProbes{
		Startup:   probeRequest(m.Startup),
		Readiness: probeRequest(m.Readiness),
		Liveness:  probeRequest(m.Liveness),
	}
}

func probeFromAPI(p *client.InstanceProbe) *InstanceProbeModel {
	if p == nil {
		return nil
	}
	return &InstanceProbeModel{
		Path:     types.StringPointerValue(p.Path),
		Port:     types.Int64PointerValue(p.Port),
		Delay:    types.Int64PointerValue(p.Delay),
		Interval: types.Int64PointerValue(p.Interval),
		Timeout:  types.Int64PointerValue(p.Timeout),
		Tries:    types.Int64PointerValue(p.Tries),
	}
}

func probesFromAPI(p *client.InstanceProbes) *InstanceProbesModel {
	if p == nil || (p.Startup == nil && p.Readiness == nil && p.Liveness == nil) {
		return nil
	}
	return &InstanceProbesModel{
		Startup:   probeFromAPI(p.Startup),
		Readiness: probeFromAPI(p.Readiness),
		Liveness:  probeFromAPI(p.Liveness),
	}
}

// probes is stored on the resource model as a types.Object rather than as
// *InstanceProbesModel: a Go struct pointer cannot hold an unknown value, and
// probes set from another resource's output is unknown at plan time, which
// would fail every plan with a value conversion error. The helpers below
// convert between the two once the value is known.

var probeAttrTypes = map[string]attr.Type{
	"path":     types.StringType,
	"port":     types.Int64Type,
	"delay":    types.Int64Type,
	"interval": types.Int64Type,
	"timeout":  types.Int64Type,
	"tries":    types.Int64Type,
}

var probesAttrTypes = map[string]attr.Type{
	"startup":   types.ObjectType{AttrTypes: probeAttrTypes},
	"readiness": types.ObjectType{AttrTypes: probeAttrTypes},
	"liveness":  types.ObjectType{AttrTypes: probeAttrTypes},
}

func probeToObject(m *InstanceProbeModel) types.Object {
	if m == nil {
		return types.ObjectNull(probeAttrTypes)
	}
	return types.ObjectValueMust(probeAttrTypes, map[string]attr.Value{
		"path":     m.Path,
		"port":     m.Port,
		"delay":    m.Delay,
		"interval": m.Interval,
		"timeout":  m.Timeout,
		"tries":    m.Tries,
	})
}

// probesToObject renders probes for the resource model; nil becomes null.
func probesToObject(m *InstanceProbesModel) types.Object {
	if m == nil {
		return types.ObjectNull(probesAttrTypes)
	}
	return types.ObjectValueMust(probesAttrTypes, map[string]attr.Value{
		"startup":   probeToObject(m.Startup),
		"readiness": probeToObject(m.Readiness),
		"liveness":  probeToObject(m.Liveness),
	})
}

func probeFromObject(v attr.Value) (*InstanceProbeModel, bool) {
	o, ok := v.(types.Object)
	if !ok || o.IsNull() {
		return nil, true
	}
	if o.IsUnknown() {
		return nil, false
	}
	a := o.Attributes()
	str := func(k string) types.String { s, _ := a[k].(types.String); return s }
	num := func(k string) types.Int64 { n, _ := a[k].(types.Int64); return n }
	return &InstanceProbeModel{
		Path: str("path"), Port: num("port"), Delay: num("delay"),
		Interval: num("interval"), Timeout: num("timeout"), Tries: num("tries"),
	}, true
}

// probesFromObject reads probes off the resource model. It reports false when
// the value, or one of its checks, is not known yet.
func probesFromObject(o types.Object) (*InstanceProbesModel, bool) {
	if o.IsNull() {
		return nil, true
	}
	if o.IsUnknown() {
		return nil, false
	}
	a := o.Attributes()
	startup, ok1 := probeFromObject(a["startup"])
	readiness, ok2 := probeFromObject(a["readiness"])
	liveness, ok3 := probeFromObject(a["liveness"])
	return &InstanceProbesModel{Startup: startup, Readiness: readiness, Liveness: liveness}, ok1 && ok2 && ok3
}

func probeEqual(a, b *InstanceProbeModel) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Path.Equal(b.Path) && a.Port.Equal(b.Port) && a.Delay.Equal(b.Delay) &&
		a.Interval.Equal(b.Interval) && a.Timeout.Equal(b.Timeout) && a.Tries.Equal(b.Tries)
}

func probesEqual(a, b *InstanceProbesModel) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return probeEqual(a.Startup, b.Startup) && probeEqual(a.Readiness, b.Readiness) && probeEqual(a.Liveness, b.Liveness)
}
