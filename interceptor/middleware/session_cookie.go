package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
)

const defaultSessionCookiePrefix = "keda-session-"

// sessionCookie reads and issues the session persistence cookie of an
// InterceptorRoute. The cookie value is "<pod ID>.<issued unix seconds>".
type sessionCookie struct {
	name string
	// absoluteTimeout is the maximum session age; zero means no expiry.
	absoluteTimeout time.Duration
}

// newSessionCookie returns the session cookie for ir, or nil when session
// persistence is disabled.
func newSessionCookie(ir *httpv1beta1.InterceptorRoute) *sessionCookie {
	sp := ir.Spec.SessionPersistence
	if sp.Type != httpv1beta1.SessionPersistenceTypeCookie {
		return nil
	}

	return &sessionCookie{
		name:            sessionCookieName(ir),
		absoluteTimeout: sp.AbsoluteTimeout.Duration,
	}
}

// sessionCookieName returns the configured cookie name, or else a name unique
// to the route, so routes sharing a host don't overwrite each other's sessions.
func sessionCookieName(ir *httpv1beta1.InterceptorRoute) string {
	if name := ir.Spec.SessionPersistence.Cookie.Name; name != "" {
		return name
	}
	sum := sha256.Sum256([]byte(ir.Namespace + "/" + ir.Name))
	return defaultSessionCookiePrefix + hex.EncodeToString(sum[:4])
}

// podID returns the pod ID of the session cookie on r, or "" when the cookie
// is missing, malformed or expired.
func (sc sessionCookie) podID(r *http.Request) string {
	c, err := r.Cookie(sc.name)
	if err != nil {
		return ""
	}

	id, issuedStr, ok := strings.Cut(c.Value, ".")
	if !ok || id == "" {
		return ""
	}
	issued, err := strconv.ParseInt(issuedStr, 10, 64)
	if err != nil {
		return ""
	}
	// time.Since saturates instead of overflowing for far-off timestamps.
	if sc.absoluteTimeout > 0 && time.Since(time.Unix(issued, 0)) > sc.absoluteTimeout {
		return ""
	}
	return id
}

// issue returns a session cookie assigning the session to podID. The cookie is
// Secure only when the client connection is HTTPS, so it keeps working behind
// plain-HTTP ingresses.
func (sc sessionCookie) issue(r *http.Request, podID string) *http.Cookie {
	return &http.Cookie{ //nolint:gosec // G124: Secure depends on the client connection, see above.
		Name:     sc.name,
		Value:    podID + "." + strconv.FormatInt(time.Now().Unix(), 10),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
	}
}
