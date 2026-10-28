package registry

import "testing"

func TestMembership(t *testing.T) {
	for _, eco := range []string{"npm", "crates.io", "Go", "PyPI"} {
		t.Run(eco, func(t *testing.T) {
			a := Affected{Ranges: []Range{{Type: "ECOSYSTEM", Events: []Event{{Introduced: "0"}, {Fixed: "2.0.0"}}}}}
			for _, tc := range []struct{ version, want string }{{"1.0.0", "affected"}, {"2.0.0", "unaffected"}} {
				if got := membership(eco, tc.version, a); got != tc.want {
					t.Fatalf("%s: got %s want %s", tc.version, got, tc.want)
				}
			}
			a.Ranges = nil
			a.Versions = []string{"3.0.0"}
			if membership(eco, "3.0.0", a) != "affected" {
				t.Fatal("explicit version lost")
			}
		})
	}
}

func TestConservativeMatching(t *testing.T) {
	for _, tc := range []struct{ eco, version, fixed, want string }{
		{"npm", "1.2.0-beta.1", "1.2.0", "affected"},
		{"crates.io", "^1.0.0", "2.0.0", "unknown"},
		{"Go", "v1.2.1-0.20240101000000-abcdefabcdef", "1.3.0", "affected"},
		{"PyPI", "1!1.0", "2.0", "unknown"},
		{"PyPI", "1.0rc1", "2.0", "unknown"},
		{"PyPI", "1.0.post1", "2.0", "unknown"},
	} {
		a := Affected{Ranges: []Range{{Type: "ECOSYSTEM", Events: []Event{{Introduced: "0"}, {Fixed: tc.fixed}}}}}
		if got := membership(tc.eco, tc.version, a); got != tc.want {
			t.Errorf("%s %s: %s != %s", tc.eco, tc.version, got, tc.want)
		}
	}
	p, err := Canonical(Identity{Ecosystem: "PyPI", Name: "Foo_Bar.BAZ"})
	if err != nil || p.Name != "foo-bar-baz" {
		t.Fatal(p, err)
	}
	a := Affected{Versions: []string{"1.0.0"}}
	if membership("PyPI", "01.0", a) != "affected" {
		t.Fatal("PEP440 release normalization")
	}
	a.Ranges = []Range{{Type: "GIT", Events: []Event{{Introduced: "0"}}}}
	a.Versions = nil
	if membership("npm", "1.0.0", a) != "unknown" {
		t.Fatal("GIT cleared")
	}
}
