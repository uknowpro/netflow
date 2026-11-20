package netflow

import "testing"

func TestPathValidation(t *testing.T) {
	base := Flow{ID: "test", Protocol: "dns", Host: "localhost"}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{"external_firewall", "external_waf", "nsg", "application_gateway", "on_prem_firewall", "network_policy", "other"} {
		f := base
		f.ExpectedPath = []ControlPoint{{Type: typ, Name: "device"}}
		if err := f.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, modify := range []func(*Flow){
		func(f *Flow) { f.SourceEnvironment = "unknown" },
		func(f *Flow) { f.ExpectedPath = []ControlPoint{{Type: "typo", Name: "edge"}} },
		func(f *Flow) { f.ExpectedPath = []ControlPoint{{Type: "nsg", Name: " "}} },
		func(f *Flow) { f.ExpectedPath = make([]ControlPoint, 33) },
	} {
		f := base
		modify(&f)
		if f.Validate() == nil {
			t.Fatalf("accepted invalid declaration: %+v", f)
		}
	}
}
