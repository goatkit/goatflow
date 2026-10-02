package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/httpcookie"
)

// ssoBrowserCookie binds an SSO login to the browser that started it. The
// redirect handler sets it to a random nonce and stores the nonce's hash with
// the state token; the callback accepts that state only from a browser that
// presents the nonce. Without it an attacker could start a login with their
// own IdP account and make a victim's browser load the captured callback URL,
// logging the victim into the attacker's account (login CSRF).
const (
	ssoBrowserCookie       = "sso_login"
	ssoBrowserCookieMaxAge = 10 * 60
)

// bindSSOToBrowser sets the binding cookie and returns the value to store
// with the state token. post is true when the IdP returns the browser with a
// cross-site POST (SAML) rather than a redirect (OIDC).
func bindSSOToBrowser(c *gin.Context, post bool) string {
	nonce := generateState()
	httpcookie.SetIdPReturn(c, ssoBrowserCookie, nonce, ssoBrowserCookieMaxAge, post)
	return hashSSONonce(nonce)
}

// ssoBrowserMatches clears the binding cookie and reports whether the request
// carries the nonce the stored binding was made from.
func ssoBrowserMatches(c *gin.Context, binding string) bool {
	nonce, err := c.Cookie(ssoBrowserCookie)
	httpcookie.ClearAuth(c, ssoBrowserCookie)
	if err != nil || nonce == "" || binding == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(hashSSONonce(nonce)), []byte(binding)) == 1
}

func hashSSONonce(nonce string) string {
	sum := sha256.Sum256([]byte(nonce))
	return hex.EncodeToString(sum[:])
}
