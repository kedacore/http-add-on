package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
)

func TestNewSessionCookie(t *testing.T) {
	cookieType := httpv1beta1.SessionPersistenceTypeCookie

	tests := map[string]struct {
		sp          httpv1beta1.SessionPersistence
		wantName    string
		wantTimeout time.Duration
	}{
		"defaults": {
			sp:       httpv1beta1.SessionPersistence{Type: cookieType},
			wantName: "keda-session-03a6829c",
		},
		"custom name": {
			sp:       httpv1beta1.SessionPersistence{Type: cookieType, Cookie: httpv1beta1.SessionCookie{Name: "my-session"}},
			wantName: "my-session",
		},
		"absolute timeout": {
			sp:          httpv1beta1.SessionPersistence{Type: cookieType, AbsoluteTimeout: metav1.Duration{Duration: time.Minute}},
			wantName:    "keda-session-03a6829c",
			wantTimeout: time.Minute,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ir := &httpv1beta1.InterceptorRoute{
				ObjectMeta: metav1.ObjectMeta{Name: "my-route", Namespace: "my-ns"},
				Spec:       httpv1beta1.InterceptorRouteSpec{SessionPersistence: tt.sp},
			}

			sc := newSessionCookie(ir)

			if sc.name != tt.wantName {
				t.Errorf("name = %q, want %q", sc.name, tt.wantName)
			}
			if sc.absoluteTimeout != tt.wantTimeout {
				t.Errorf("absoluteTimeout = %v, want %v", sc.absoluteTimeout, tt.wantTimeout)
			}
		})
	}
}

func TestNewSessionCookie_DisabledReturnsNil(t *testing.T) {
	ir := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "my-route", Namespace: "my-ns"},
	}

	if sc := newSessionCookie(ir); sc != nil {
		t.Errorf("newSessionCookie() = %+v, want nil", sc)
	}
}

func TestNewSessionCookie_DefaultNameDiffersPerRoute(t *testing.T) {
	newRoute := func(namespace, name string) *httpv1beta1.InterceptorRoute {
		return &httpv1beta1.InterceptorRoute{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       httpv1beta1.InterceptorRouteSpec{SessionPersistence: httpv1beta1.SessionPersistence{Type: httpv1beta1.SessionPersistenceTypeCookie}},
		}
	}

	a := newSessionCookie(newRoute("ns", "route-a")).name
	b := newSessionCookie(newRoute("ns", "route-b")).name
	c := newSessionCookie(newRoute("other-ns", "route-a")).name

	if a == b || a == c || b == c {
		t.Errorf("default names must differ per route, got %q, %q, %q", a, b, c)
	}
}

func TestSessionCookie_PodID(t *testing.T) {
	// Cookies are issued relative to the synctest fake clock, which starts at
	// midnight UTC 2000-01-01.
	now := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	issuedAt := func(age time.Duration) string {
		return strconv.FormatInt(now.Add(-age).Unix(), 10)
	}

	tests := map[string]struct {
		absoluteTimeout time.Duration
		cookieHeader    string
		want            string
	}{
		"no cookie": {
			want: "",
		},
		"other cookie only": {
			cookieHeader: "JSESSIONID=abc",
			want:         "",
		},
		"valid cookie": {
			cookieHeader: "session=0123456789abcdef." + issuedAt(time.Hour),
			want:         "0123456789abcdef",
		},
		"valid cookie next to app cookie": {
			cookieHeader: "JSESSIONID=abc; session=0123456789abcdef." + issuedAt(time.Hour),
			want:         "0123456789abcdef",
		},
		"missing timestamp": {
			cookieHeader: "session=0123456789abcdef",
			want:         "",
		},
		"empty id": {
			cookieHeader: "session=." + issuedAt(0),
			want:         "",
		},
		"malformed timestamp": {
			cookieHeader: "session=0123456789abcdef.soon",
			want:         "",
		},
		"overflowing timestamp": {
			cookieHeader: "session=0123456789abcdef.99999999999999999999",
			want:         "",
		},
		"within absolute timeout": {
			absoluteTimeout: time.Minute,
			cookieHeader:    "session=0123456789abcdef." + issuedAt(59*time.Second),
			want:            "0123456789abcdef",
		},
		"exactly at absolute timeout": {
			absoluteTimeout: time.Minute,
			cookieHeader:    "session=0123456789abcdef." + issuedAt(time.Minute),
			want:            "0123456789abcdef",
		},
		"beyond absolute timeout": {
			absoluteTimeout: time.Minute,
			cookieHeader:    "session=0123456789abcdef." + issuedAt(time.Minute+time.Second),
			want:            "",
		},
		"far past timestamp with absolute timeout": {
			absoluteTimeout: time.Minute,
			cookieHeader:    "session=0123456789abcdef.-9223372036854775808",
			want:            "",
		},
		"issued in the future": {
			absoluteTimeout: time.Minute,
			cookieHeader:    "session=0123456789abcdef." + issuedAt(-time.Second),
			want:            "0123456789abcdef",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				sc := sessionCookie{name: "session", absoluteTimeout: tt.absoluteTimeout}
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				if tt.cookieHeader != "" {
					req.Header.Set("Cookie", tt.cookieHeader)
				}

				if got := sc.podID(req); got != tt.want {
					t.Errorf("podID() = %q, want %q", got, tt.want)
				}
			})
		})
	}
}

func TestSessionCookie_Issue(t *testing.T) {
	tests := map[string]struct {
		tls            bool
		forwardedProto string
		wantSecure     bool
	}{
		"plain http":                   {},
		"tls":                          {tls: true, wantSecure: true},
		"forwarded https":              {forwardedProto: "https", wantSecure: true},
		"forwarded https mixed case":   {forwardedProto: "HTTPS", wantSecure: true},
		"forwarded http":               {forwardedProto: "http"},
		"tls overrides forwarded http": {tls: true, forwardedProto: "http", wantSecure: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			sc := sessionCookie{name: "session", absoluteTimeout: time.Minute}
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}
			if tt.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
			}

			c := sc.issue(req, "0123456789abcdef")

			if c.Name != "session" {
				t.Errorf("Name = %q, want %q", c.Name, "session")
			}
			if id, issued, _ := strings.Cut(c.Value, "."); id != "0123456789abcdef" || issued == "" {
				t.Errorf("Value = %q, want %q followed by the issue time", c.Value, "0123456789abcdef.")
			}
			if c.Path != "/" {
				t.Errorf("Path = %q, want %q", c.Path, "/")
			}
			if !c.HttpOnly {
				t.Error("HttpOnly = false, want true")
			}
			if c.SameSite != http.SameSiteLaxMode {
				t.Errorf("SameSite = %v, want Lax", c.SameSite)
			}
			if c.Secure != tt.wantSecure {
				t.Errorf("Secure = %v, want %v", c.Secure, tt.wantSecure)
			}
			if c.MaxAge != 0 || !c.Expires.IsZero() {
				t.Errorf("cookie must be a session cookie, got MaxAge=%d Expires=%v", c.MaxAge, c.Expires)
			}
		})
	}
}

func TestSessionCookie_IssuedCookieRoundTrips(t *testing.T) {
	// synctest provides a fake clock so the session can age instantly.
	synctest.Test(t, func(t *testing.T) {
		sc := sessionCookie{name: "session", absoluteTimeout: time.Minute}

		issued := sc.issue(httptest.NewRequest(http.MethodGet, "/", nil), "0123456789abcdef")
		if want := "0123456789abcdef." + strconv.FormatInt(time.Now().Unix(), 10); issued.Value != want {
			t.Fatalf("Value = %q, want %q", issued.Value, want)
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(issued)

		time.Sleep(time.Minute)
		if got := sc.podID(req); got != "0123456789abcdef" {
			t.Errorf("podID() = %q, want %q", got, "0123456789abcdef")
		}
		time.Sleep(time.Second)
		if got := sc.podID(req); got != "" {
			t.Errorf("podID() after timeout = %q, want empty", got)
		}
	})
}
