package postgres

import "testing"

func TestCanonicalURLRemovesTrackingAndFragments(t *testing.T) {
	got := CanonicalURL("HTTPS://Jobs.Example/role/?utm_source=board&ref=feed&team=eng#apply")
	want := "https://jobs.example/role?team=eng"
	if got != want {
		t.Fatalf("CanonicalURL()=%q, want %q", got, want)
	}
	if got := CanonicalURL("javascript:alert(1)"); got != "" {
		t.Fatalf("unsafe URL canonicalized to %q", got)
	}
}
