package domain

import (
	sdk "2026_2_PinPals/pkg/onlineconf"
	"testing"
	"time"
)

func TestValueTypes(t *testing.T) {
	r := Resource{Kind: "config", Key: "ui.example", Title: "Example", Scopes: []string{"frontend"}, Visibility: "public"}
	for kind, value := range map[string]string{"boolean": "false", "string": "\"ok\"", "integer": "0", "number": "1.5", "array": "[]", "object": "{}", "null": "null"} {
		r.ValueType = kind
		r.Value = []byte(value)
		if err := r.Validate(); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		r.Value = []byte("broken")
		if r.Validate() == nil {
			t.Fatalf("invalid %s accepted", kind)
		}
	}
	r.Key = sdk.PageSizeKey
	r.ValueType = "integer"
	r.Scopes = []string{"frontend", "pin_service"}
	for _, value := range []string{"101", "-1", "null", "2.5", "0"} {
		r.Value = []byte(value)
		if r.Validate() == nil {
			t.Fatalf("accepted %s", value)
		}
	}
}
func TestApprovalsAndRollback(t *testing.T) {
	var r Resource
	start := time.Now()
	p := &Plan{Name: "trial", Stages: []int{5, 10, 25}, Duration: 10, ApproverRole: "reviewer"}
	if err := r.Rollout("create", p, nil, start); err != nil {
		t.Fatal(err)
	}
	roles := Rights{{Name: "reviewer"}}
	if r.Rollout("approve", nil, roles, start) != ErrConflict {
		t.Fatal("early transition")
	}
	later := start.Add(20 * time.Second)
	if r.Rollout("approve", nil, nil, later) != ErrDenied {
		t.Fatal("wrong approver")
	}
	if err := r.Rollout("approve", nil, roles, later); err != nil || r.Percent != 10 {
		t.Fatal(err)
	}
	if err := r.Rollout("rollback", nil, roles, later); err != nil || r.Percent != 5 || r.Plan.Status != "paused" {
		t.Fatal(err)
	}
}
func TestRightsAcrossScopes(t *testing.T) {
	r := Rights{{Name: "frontend", Scopes: []string{"frontend"}, Permissions: []string{"flag.update"}},
		{Name: "reader", Scopes: []string{"pin_service"}, Permissions: []string{"flag.read"}}}
	if r.Allows("flag.update", []string{"frontend", "pin_service"}) {
		t.Fatal("permission leaked between scopes")
	}
}
