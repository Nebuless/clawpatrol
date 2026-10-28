package provider

import (
	"path/filepath"
	"testing"
)

func TestCaseCustomerBinding(t *testing.T) {
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
	if err = s.RegisterInstallation(Installation{ID: "installation", OrganizationID: "org", Name: "Gateway", Hosting: "customer"}); err != nil {
		t.Fatal(err)
	}
	c, err := s.SubmitCase("org", "installation", "intent", "account-a", "Synthetic case", "configuration")
	if err != nil || c.AccountID != "account-a" {
		t.Fatalf("binding lost: %#v %v", c, err)
	}
	if _, err = s.SubmitCase("org", "installation", "intent", "account-b", "Synthetic case", "configuration"); err == nil {
		t.Fatal("intent rebound to other customer")
	}
}

func TestOrganizationIsolation(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "provider.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := s.AddOrganization("org-a", "Customer A"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddOrganization("org-b", "Customer B"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddTechnician("tech", "https://idp.example", "subject", "Technician", false); err != nil {
		t.Fatal(err)
	}
	if err := s.AssignOrganization("tech", "org-a"); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterInstallation(Installation{ID: "gateway-b", OrganizationID: "org-b", Name: "B", Hosting: "customer"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InstallationFor("tech", "org-a", "gateway-b"); err == nil {
		t.Fatal("cross-org installation accepted")
	}
	if _, err := s.InstallationFor("tech", "org-b", "gateway-b"); err == nil {
		t.Fatal("unassigned organization accepted")
	}
	if err := s.AssignOrganization("tech", "org-b"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InstallationFor("tech", "org-b", "gateway-b"); err != nil {
		t.Fatal(err)
	}
}
