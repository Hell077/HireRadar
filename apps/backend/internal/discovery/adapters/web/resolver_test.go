package web

import (
	"context"
	"net"
	"net/url"
	"testing"

	"github.com/Hell077/HireRadar/apps/backend/internal/discovery/domain"
)

type fixedDNS []net.IPAddr

func (f fixedDNS) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) { return f, nil }

func TestDetectSupportedAndUnsupportedATS(t *testing.T) {
	tests := []struct {
		url, provider, key string
	}{
		{"https://boards.greenhouse.io/grafanalabs/jobs", "greenhouse", "grafanalabs"},
		{"https://boards.greenhouse.io/embed/job_board?for=example", "greenhouse", "example"},
		{"https://jobs.eu.lever.co/acme", "lever", "acme"},
		{"https://jobs.ashbyhq.com/linear", "ashby", "linear"},
		{"https://api.ashbyhq.com/posting-api/job-board/linear", "ashby", "linear"},
		{"https://jobs.smartrecruiters.com/Acme", "smartrecruiters", "Acme"},
		{"https://apply.workable.com/acme/j/123", "workable", "acme"},
		{"https://acme.teamtailor.com/jobs", "teamtailor", "acme"},
		{"https://apply.workable.com/huggingface/", "workable", "huggingface"},
		{"https://acme.jobs.personio.com/job/123", "personio", "acme"},
		{"https://acme.wd5.myworkdayjobs.com/en-US/careers", "workday", "acme.wd5.myworkdayjobs.com/en-US"},
	}
	for _, test := range tests {
		t.Run(test.url, func(t *testing.T) {
			parsed, err := url.Parse(test.url)
			if err != nil {
				t.Fatal(err)
			}
			got := DetectURL(parsed)
			if got == nil || got.Type != test.provider || got.Key != test.key || got.Confidence < 0.9 {
				t.Fatalf("detected %#v, want %s/%s", got, test.provider, test.key)
			}
		})
	}
}

func TestDetectDoesNotTreatWorkableJobPathAsAccountKey(t *testing.T) {
	page, _ := url.Parse("https://apply.workable.com/j/AB12CD34/apply")
	if got := Detect(page, nil); got != nil {
		t.Fatalf("job-specific apply URL yielded account key: %+v", got)
	}
}

func TestDetectURLRejectsIndividualJobLinksAsBoardIdentity(t *testing.T) {
	for _, raw := range []string{
		"https://boards.greenhouse.io/acme/jobs/123/senior-engineer",
		"https://jobs.lever.co/acme/01234567-89ab-cdef-0123-456789abcdef",
		"https://jobs.ashbyhq.com/acme/01234567-89ab-cdef-0123-456789abcdef",
	} {
		parsed, err := url.Parse(raw)
		if err != nil || DetectURL(parsed) != nil {
			t.Fatalf("individual posting %q resolved as a source: %#v err=%v", raw, DetectURL(parsed), err)
		}
	}
}

func TestDetectATSInIframeScriptAndForm(t *testing.T) {
	page, _ := url.Parse("https://example.org/careers")
	body := []byte(`<html><iframe src="https://jobs.ashbyhq.com/example"></iframe><script src="https://boards.greenhouse.io/embed/job_board/js?for=other"></script><form action="https://jobs.lever.co/acme"></form></html>`)
	got := Detect(page, body)
	if got == nil || got.Type != "ashby" || got.Key != "example" {
		t.Fatalf("unexpected provider: %#v", got)
	}
}

func TestCareersResolverRejectsPrivateAndSpecialAddresses(t *testing.T) {
	addresses := []string{"127.0.0.1", "10.2.3.4", "169.254.169.254", "192.168.1.1", "100.64.0.2", "::1", "fd00::1", "fe80::1", "2001:db8::1"}
	for _, address := range addresses {
		t.Run(address, func(t *testing.T) {
			r := &Resolver{resolver: net.DefaultResolver}
			target, _ := url.Parse("http://" + net.JoinHostPort(address, "80") + "/")
			if err := r.validateURL(context.Background(), target); err == nil {
				t.Fatalf("accepted non-public address %s", address)
			}
		})
	}
}

func TestCareersResolverRejectsDNSRebindingToPrivateNetwork(t *testing.T) {
	target, _ := url.Parse("https://public.example/careers")
	r := &Resolver{resolver: fixedDNS{{IP: net.ParseIP("93.184.216.34")}, {IP: net.ParseIP("10.0.0.5")}}}
	if err := r.validateURL(context.Background(), target); err == nil {
		t.Fatal("accepted a host with one private DNS answer")
	}
}

func TestResolutionWithoutURLsReturnsManualReview(t *testing.T) {
	result, err := NewResolver().Resolve(context.Background(), domain.Candidate{Name: "No careers data"})
	if err != nil || result.Provider != nil || result.Reason == "" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
