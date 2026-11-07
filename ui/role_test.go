package ui

import "testing"

func TestRoleNamesRoundTrip(t *testing.T) {
	for role := RoleButton; role <= RoleSeparator; role++ {
		name := role.String()
		if name == "" || name == "region" {
			t.Fatalf("role %d has no name", role)
		}
		if got, ok := RoleNamed(name); !ok || got != role {
			t.Errorf("RoleNamed(%q) = %v, %v; want %v", name, got, ok, role)
		}
	}
	if Role(0).String() != "region" || Role(200).String() != "region" {
		t.Error("a role that was not declared is not a plain region")
	}
	if _, ok := RoleNamed("region"); ok {
		t.Error("\"region\" names no role")
	}
}
