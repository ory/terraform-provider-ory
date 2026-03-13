package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestFetchSchemaFromURL(t *testing.T) {
	// fetchSchemaFromURL calls isPrivateHost, which blocks 127.0.0.1 (used
	// by httptest). To test the HTTP plumbing we temporarily disable the
	// host check by swapping the checker function.
	withTestServer := func(t *testing.T, handler http.HandlerFunc, fn func(t *testing.T, url string)) {
		t.Helper()
		srv := httptest.NewTLSServer(handler)
		defer srv.Close()

		origClient := schemaFetchClient
		origChecker := hostChecker
		schemaFetchClient = srv.Client()
		hostChecker = func(string) bool { return false } // allow all hosts
		defer func() {
			schemaFetchClient = origClient
			hostChecker = origChecker
		}()
		fn(t, srv.URL)
	}

	t.Run("successful HTTPS fetch", func(t *testing.T) {
		withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"type":"object","properties":{"name":{"type":"string"}}}`))
		}, func(t *testing.T, url string) {
			result, err := fetchSchemaFromURL(context.Background(), url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result["type"] != "object" {
				t.Errorf("expected type=object, got %v", result["type"])
			}
		})
	})

	t.Run("non-200 response", func(t *testing.T) {
		withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}, func(t *testing.T, url string) {
			_, err := fetchSchemaFromURL(context.Background(), url)
			if err == nil {
				t.Fatal("expected error for non-200 response")
			}
		})
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`not json`))
		}, func(t *testing.T, url string) {
			_, err := fetchSchemaFromURL(context.Background(), url)
			if err == nil {
				t.Fatal("expected error for invalid JSON")
			}
		})
	})

	t.Run("rejects non-HTTPS URL", func(t *testing.T) {
		_, err := fetchSchemaFromURL(context.Background(), "http://example.com/schema.json")
		if err == nil {
			t.Fatal("expected error for non-HTTPS URL")
		}
	})

	t.Run("rejects private host IP", func(t *testing.T) {
		_, err := fetchSchemaFromURL(context.Background(), "https://127.0.0.1/schema.json")
		if err == nil {
			t.Fatal("expected error for loopback IP")
		}
	})

	t.Run("rejects private host 10.x", func(t *testing.T) {
		_, err := fetchSchemaFromURL(context.Background(), "https://10.0.0.1/schema.json")
		if err == nil {
			t.Fatal("expected error for private IP 10.x")
		}
	})
}

func TestIsPrivateHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		// Loopback
		{"127.0.0.1", true},
		{"::1", true},
		{"localhost", true},

		// Unspecified
		{"0.0.0.0", true},

		// Private RFC 1918
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.0.1", true},
		{"192.168.255.255", true},

		// Link-local
		{"169.254.1.1", true},

		// Public IPs — must NOT be blocked
		{"172.2.0.1", false},  // 172.2.x is public, not in 172.16/12
		{"172.15.0.1", false}, // below 172.16/12
		{"172.32.0.1", false}, // above 172.16/12
		{"8.8.8.8", false},
		{"1.1.1.1", false},

		// Known Ory schema hosts
		{"storage.googleapis.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			got := isPrivateHost(tt.host)
			if got != tt.want {
				t.Errorf("isPrivateHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

func TestIsPrivateAddr(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"0.0.0.0", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.1", true},
		{"169.254.1.1", true},
		{"fe80::1", true},

		// Public
		{"8.8.8.8", false},
		{"172.2.0.1", false},
		{"172.32.0.1", false},
		{"1.1.1.1", false},
		{"2001:4860:4860::8888", false},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			addr, err := netip.ParseAddr(tt.ip)
			if err != nil {
				t.Fatalf("failed to parse %q: %v", tt.ip, err)
			}
			got := isPrivateAddr(addr)
			if got != tt.want {
				t.Errorf("isPrivateAddr(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}
