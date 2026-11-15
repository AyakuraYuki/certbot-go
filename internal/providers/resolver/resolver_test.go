package resolver

import "testing"

func TestResolve(t *testing.T) {
	r := New(map[string]string{
		"example.com": "example.proxy-acme.com",
		"other.com":   "proxy-acme.com",
	})

	cases := []struct {
		domain, rr, zone string
	}{
		{"example.com", "_acme-challenge.example", "proxy-acme.com"},
		{"*.example.com", "_acme-challenge.example", "proxy-acme.com"},
		{"api.example.com", "_acme-challenge.api.example", "proxy-acme.com"},
		{"other.com", "_acme-challenge", "proxy-acme.com"},
		{"direct.com", "_acme-challenge", "direct.com"},
		{"*.direct.com", "_acme-challenge", "direct.com"},
		{"a.b.direct.com", "_acme-challenge.a.b", "direct.com"},
	}

	for _, c := range cases {
		rr, zone, err := r.Resolve(c.domain)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", c.domain, err)
		}
		if rr != c.rr || zone != c.zone {
			t.Errorf("%s: got (%s, %s), want (%s, %s)", c.domain, rr, zone, c.rr, c.zone)
		}
	}

	if _, _, err := r.Resolve("localhost"); err == nil {
		t.Error("localhost: expected error")
	}
}
