package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRouterWithMirrorsRegistersMirrorRoutes(t *testing.T) {
	router := NewRouterWithMirrors(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v2/library/alpine/manifests/latest" {
				t.Fatalf("docker path = %q, want %q", r.URL.Path, "/v2/library/alpine/manifests/latest")
			}
			_, _ = w.Write([]byte("docker"))
		}),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/left-pad" {
				t.Fatalf("npm path = %q, want %q", r.URL.Path, "/left-pad")
			}
			_, _ = w.Write([]byte("npm"))
		}),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/simple/pip/" {
				t.Fatalf("pypi path = %q, want %q", r.URL.Path, "/simple/pip/")
			}
			_, _ = w.Write([]byte("pypi"))
		}),
	)

	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "docker route", url: "/docker/v2/library/alpine/manifests/latest", want: "docker"},
		{name: "npm route", url: "/npm/left-pad", want: "npm"},
		{name: "pypi route", url: "/pypi/simple/pip/", want: "pypi"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if rec.Body.String() != tc.want {
				t.Fatalf("body = %q, want %q", rec.Body.String(), tc.want)
			}
		})
	}
}
