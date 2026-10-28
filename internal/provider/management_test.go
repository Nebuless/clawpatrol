package provider

import (
	"path/filepath"
	"testing"
)

func TestCaseAssignmentVisibility(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "provider.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err = s.AddOrganization("org", "Organization"); err != nil {
		t.Fatal(err)
	}
	if err = s.RegisterInstallation(Installation{ID: "gateway", OrganizationID: "org", Name: "Gateway", Hosting: "customer"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"admin", "one", "two"} {
		if err = s.AddTechnician(id, "https://idp.example", id, id, id == "admin"); err != nil {
			t.Fatal(err)
		}
		if err = s.AssignOrganization(id, "org"); err != nil {
			t.Fatal(err)
		}
	}
	admin, err := s.Technician("https://idp.example", "admin")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"unassigned", "one", "two"} {
		c, err := s.SubmitCase("org", "gateway", id, "account-"+id, "Synthetic case", "configuration")
		if err != nil {
			t.Fatal(err)
		}
		if id != "unassigned" {
			if err = s.AssignCase(admin, "org", "gateway", c.ID, id); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, id := range []string{"admin", "one", "two"} {
		tech, err := s.Technician("https://idp.example", id)
		if err != nil {
			t.Fatal(err)
		}
		cases, err := s.Cases(tech, "org", "gateway")
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if id == "admin" {
			want = 3
		}
		if len(cases) != want {
			t.Fatalf("%s sees %d cases, want %d", id, len(cases), want)
		}
		for _, c := range cases {
			if id != "admin" && c.TechnicianID != id {
				t.Fatalf("%s sees other assignment: %#v", id, c)
			}
		}
	}
}
