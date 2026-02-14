package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/session"
)

func init() {
	log.Init(core.Options{
		Debug:    &[]bool{true}[0],
		Verbose:  &[]bool{false}[0],
		NoColors: &[]bool{true}[0],
	}, false, "")
}

func newTestMuraenaProxy(bypassURLs []string, redirects []session.Redirect) *MuraenaProxy {
	s := &session.Session{
		Config: &session.Configuration{},
	}
	s.Config.Transform.BypassURLs = bypassURLs
	s.Config.Redirects = redirects
	return &MuraenaProxy{Session: s}
}

func TestGetSenderIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		want       string
	}{
		{
			name: "True-Client-IP set",
			headers: map[string]string{
				"True-Client-IP": "10.0.0.1",
			},
			remoteAddr: "192.168.1.1:12345",
			want:       "10.0.0.1",
		},
		{
			name: "CF-Connecting-IP set without True-Client-IP",
			headers: map[string]string{
				"CF-Connecting-IP": "172.16.0.1",
			},
			remoteAddr: "192.168.1.1:12345",
			want:       "172.16.0.1",
		},
		{
			name: "X-Forwarded-For set without others",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.50",
			},
			remoteAddr: "192.168.1.1:12345",
			want:       "203.0.113.50",
		},
		{
			name: "X-Forwarded-For with comma-separated list returns first IP",
			headers: map[string]string{
				"X-Forwarded-For": "1.2.3.4, 5.6.7.8",
			},
			remoteAddr: "192.168.1.1:12345",
			want:       "1.2.3.4",
		},
		{
			name:       "No headers falls back to RemoteAddr",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.1:12345",
			want:       "192.168.1.1",
		},
		{
			name: "All headers set returns True-Client-IP as highest priority",
			headers: map[string]string{
				"True-Client-IP":   "10.0.0.1",
				"CF-Connecting-IP": "172.16.0.1",
				"X-Forwarded-For":  "203.0.113.50",
			},
			remoteAddr: "192.168.1.1:12345",
			want:       "10.0.0.1",
		},
		{
			name: "CF-Connecting-IP and X-Forwarded-For set returns CF-Connecting-IP",
			headers: map[string]string{
				"CF-Connecting-IP": "172.16.0.1",
				"X-Forwarded-For":  "203.0.113.50",
			},
			remoteAddr: "192.168.1.1:12345",
			want:       "172.16.0.1",
		},
		{
			name: "X-Forwarded-For with spaces around commas",
			headers: map[string]string{
				"X-Forwarded-For": "  9.8.7.6 , 5.4.3.2 ",
			},
			remoteAddr: "192.168.1.1:12345",
			want:       "9.8.7.6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header:     http.Header{},
				RemoteAddr: tt.remoteAddr,
			}
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := GetSenderIP(req)
			if got != tt.want {
				t.Errorf("GetSenderIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedirectToHTTPS(t *testing.T) {
	tests := []struct {
		name           string
		port           int
		host           string
		path           string
		wantStatusCode int
		wantLocation   string
	}{
		{
			name:           "Port 443 redirects without port in URL",
			port:           443,
			host:           "example.com",
			path:           "/path",
			wantStatusCode: http.StatusMovedPermanently,
			wantLocation:   "https://example.com/path",
		},
		{
			name:           "Non-443 port includes port in URL",
			port:           8443,
			host:           "example.com",
			path:           "/path",
			wantStatusCode: http.StatusMovedPermanently,
			wantLocation:   "https://example.com:8443/path",
		},
		{
			name:           "Host with existing port gets port stripped and new port added",
			port:           8443,
			host:           "example.com:80",
			path:           "/path",
			wantStatusCode: http.StatusMovedPermanently,
			wantLocation:   "https://example.com:8443/path",
		},
		{
			name:           "Host with existing port and default HTTPS port 443",
			port:           443,
			host:           "example.com:80",
			path:           "/path",
			wantStatusCode: http.StatusMovedPermanently,
			wantLocation:   "https://example.com/path",
		},
		{
			name:           "Root path",
			port:           443,
			host:           "example.com",
			path:           "/",
			wantStatusCode: http.StatusMovedPermanently,
			wantLocation:   "https://example.com/",
		},
		{
			name:           "Path with query string",
			port:           443,
			host:           "example.com",
			path:           "/search?q=test",
			wantStatusCode: http.StatusMovedPermanently,
			wantLocation:   "https://example.com/search?q=test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RedirectToHTTPS(tt.port)

			// Parse only the path (and query) portion, mimicking how Go's
			// HTTP server populates req.URL for incoming requests.
			reqURL, err := url.Parse(tt.path)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			req := &http.Request{
				Method: http.MethodGet,
				Host:   tt.host,
				URL:    reqURL,
				Header: http.Header{},
			}
			rr := httptest.NewRecorder()

			handler(rr, req)

			if rr.Code != tt.wantStatusCode {
				t.Errorf("status code = %d, want %d", rr.Code, tt.wantStatusCode)
			}

			location := rr.Header().Get("Location")
			if location != tt.wantLocation {
				t.Errorf("Location = %q, want %q", location, tt.wantLocation)
			}
		})
	}
}

func TestShouldBypassURL(t *testing.T) {
	tests := []struct {
		name       string
		bypassURLs []string
		path       string
		want       bool
	}{
		{
			name:       "Path matches bypass prefix",
			bypassURLs: []string{"/api/v1", "/health"},
			path:       "/api/v1/users",
			want:       true,
		},
		{
			name:       "Path does not match any bypass prefix",
			bypassURLs: []string{"/api/v1", "/health"},
			path:       "/login",
			want:       false,
		},
		{
			name:       "Empty bypass list returns false",
			bypassURLs: []string{},
			path:       "/anything",
			want:       false,
		},
		{
			name:       "Nil bypass list returns false",
			bypassURLs: nil,
			path:       "/anything",
			want:       false,
		},
		{
			name:       "Exact match counts as prefix match",
			bypassURLs: []string{"/health"},
			path:       "/health",
			want:       true,
		},
		{
			name:       "Partial prefix /api matches /api/v1/users",
			bypassURLs: []string{"/api"},
			path:       "/api/v1/users",
			want:       true,
		},
		{
			name:       "Path shorter than bypass prefix does not match",
			bypassURLs: []string{"/api/v1/users"},
			path:       "/api",
			want:       false,
		},
		{
			name:       "Multiple bypass URLs with second matching",
			bypassURLs: []string{"/static", "/assets", "/health"},
			path:       "/health/check",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			muraena := newTestMuraenaProxy(tt.bypassURLs, nil)
			got := muraena.shouldBypassURL(tt.path)
			if got != tt.want {
				t.Errorf("shouldBypassURL(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestGetHTTPRedirect(t *testing.T) {
	tests := []struct {
		name               string
		redirects          []session.Redirect
		reqHost            string
		reqPath            string
		reqQuery           string
		wantNil            bool
		wantRedirectTo     string
		wantHTTPStatusCode int
	}{
		{
			name: "Hostname match returns redirect",
			redirects: []session.Redirect{
				{
					Hostname:       "evil.com",
					RedirectTo:     "https://legit.com",
					HTTPStatusCode: 301,
				},
			},
			reqHost:            "evil.com",
			reqPath:            "/",
			reqQuery:           "",
			wantNil:            false,
			wantRedirectTo:     "https://legit.com",
			wantHTTPStatusCode: 301,
		},
		{
			name: "Path match returns redirect",
			redirects: []session.Redirect{
				{
					Path:           "/old-page",
					RedirectTo:     "https://example.com/new-page",
					HTTPStatusCode: 302,
				},
			},
			reqHost:            "any.com",
			reqPath:            "/old-page",
			reqQuery:           "",
			wantNil:            false,
			wantRedirectTo:     "https://example.com/new-page",
			wantHTTPStatusCode: 302,
		},
		{
			name: "Query match returns redirect",
			redirects: []session.Redirect{
				{
					Query:          "action=logout",
					RedirectTo:     "https://example.com/goodbye",
					HTTPStatusCode: 302,
				},
			},
			reqHost:            "any.com",
			reqPath:            "/",
			reqQuery:           "action=logout",
			wantNil:            false,
			wantRedirectTo:     "https://example.com/goodbye",
			wantHTTPStatusCode: 302,
		},
		{
			name: "Hostname + Path + Query all match returns redirect",
			redirects: []session.Redirect{
				{
					Hostname:       "evil.com",
					Path:           "/target",
					Query:          "ref=phish",
					RedirectTo:     "https://legit.com/safe",
					HTTPStatusCode: 307,
				},
			},
			reqHost:            "evil.com",
			reqPath:            "/target",
			reqQuery:           "ref=phish",
			wantNil:            false,
			wantRedirectTo:     "https://legit.com/safe",
			wantHTTPStatusCode: 307,
		},
		{
			name: "No match returns nil",
			redirects: []session.Redirect{
				{
					Hostname:       "evil.com",
					Path:           "/specific",
					RedirectTo:     "https://legit.com",
					HTTPStatusCode: 302,
				},
			},
			reqHost:  "other.com",
			reqPath:  "/different",
			reqQuery: "",
			wantNil:  true,
		},
		{
			name: "Hostname mismatch skips rule even if path matches",
			redirects: []session.Redirect{
				{
					Hostname:       "evil.com",
					Path:           "/page",
					RedirectTo:     "https://legit.com",
					HTTPStatusCode: 302,
				},
			},
			reqHost:  "other.com",
			reqPath:  "/page",
			reqQuery: "",
			wantNil:  true,
		},
		{
			name: "HTTPStatusCode 0 defaults to 302",
			redirects: []session.Redirect{
				{
					Hostname:       "evil.com",
					RedirectTo:     "https://legit.com",
					HTTPStatusCode: 0,
				},
			},
			reqHost:            "evil.com",
			reqPath:            "/",
			reqQuery:           "",
			wantNil:            false,
			wantRedirectTo:     "https://legit.com",
			wantHTTPStatusCode: 302,
		},
		{
			name:      "Empty redirects list returns nil",
			redirects: []session.Redirect{},
			reqHost:   "any.com",
			reqPath:   "/",
			reqQuery:  "",
			wantNil:   true,
		},
		{
			name: "First matching redirect is returned",
			redirects: []session.Redirect{
				{
					Path:           "/page",
					RedirectTo:     "https://first.com",
					HTTPStatusCode: 301,
				},
				{
					Path:           "/page",
					RedirectTo:     "https://second.com",
					HTTPStatusCode: 302,
				},
			},
			reqHost:            "any.com",
			reqPath:            "/page",
			reqQuery:           "",
			wantNil:            false,
			wantRedirectTo:     "https://first.com",
			wantHTTPStatusCode: 301,
		},
		{
			name: "Query mismatch skips rule",
			redirects: []session.Redirect{
				{
					Query:          "action=login",
					RedirectTo:     "https://legit.com",
					HTTPStatusCode: 302,
				},
			},
			reqHost:  "any.com",
			reqPath:  "/",
			reqQuery: "action=logout",
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			muraena := newTestMuraenaProxy(nil, tt.redirects)
			req := &http.Request{
				Host:   tt.reqHost,
				URL:    &url.URL{Path: tt.reqPath, RawQuery: tt.reqQuery},
				Header: http.Header{},
			}

			got := muraena.getHTTPRedirect(req)

			if tt.wantNil {
				if got != nil {
					t.Errorf("getHTTPRedirect() = %+v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("getHTTPRedirect() = nil, want non-nil redirect")
			}

			if got.RedirectTo != tt.wantRedirectTo {
				t.Errorf("RedirectTo = %q, want %q", got.RedirectTo, tt.wantRedirectTo)
			}

			if got.HTTPStatusCode != tt.wantHTTPStatusCode {
				t.Errorf("HTTPStatusCode = %d, want %d", got.HTTPStatusCode, tt.wantHTTPStatusCode)
			}
		})
	}
}
