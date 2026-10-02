package selfservice

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/config"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/platform/sysconfig"
)

// Field length limits from the customer_user / gk_registration_request schema.
const (
	maxIdentifierLen = 200
	maxEmailLen      = 150
	maxNameLen       = 100
)

// portal describes the agent or customer side of the self-service pages.
type portal struct {
	Type        string // UserAgent or UserCustomer
	LoginURL    string
	ForgotURL   string
	ResetURL    string
	RegisterURL string
}

var (
	agentPortal = portal{
		Type:      UserAgent,
		LoginURL:  "/login",
		ForgotURL: "/forgot-password",
		ResetURL:  "/reset-password",
	}
	customerPortal = portal{
		Type:        UserCustomer,
		LoginURL:    "/customer/login",
		ForgotURL:   "/customer/forgot-password",
		ResetURL:    "/customer/reset-password",
		RegisterURL: "/customer/register",
	}
)

const registerCompleteURL = "/customer/register/complete"

func lostPasswordEnabled() bool {
	cfg := config.Get()
	return cfg != nil && cfg.Features.LostPassword
}

func registrationEnabled() bool {
	cfg := config.Get()
	return cfg != nil && cfg.Features.Registration
}

// requireFeature answers 404 while the feature switch is off, exactly as if
// the route did not exist.
func requireFeature(enabled func() bool, h gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled() {
			c.String(http.StatusNotFound, "404 page not found")
			return
		}
		h(c)
	}
}

func render(c *gin.Context, code int, tpl string, ctx pongo2.Context) {
	// Pages carrying tokens must not be cached or leak the token via Referer.
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	r := shared.GetGlobalRenderer()
	if r == nil {
		c.String(http.StatusInternalServerError, "template renderer unavailable")
		return
	}
	r.HTML(c, code, tpl, ctx)
}

func storeFor(c *gin.Context) (store, bool) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		slog.Error("selfservice: database unavailable", "error", err)
		c.String(http.StatusServiceUnavailable, "service unavailable")
		return store{}, false
	}
	return store{db: db}, true
}

// allowIP spends one unit of the client IP's hourly budget for public form posts.
func allowIP(c *gin.Context) bool {
	return middleware.GlobalRateLimiter().Allow("selfservice:ip:"+c.ClientIP(), ipRequestsPerHour)
}

// allowMailTo spends one unit of a recipient's hourly email budget.
func allowMailTo(target string) bool {
	return middleware.GlobalRateLimiter().Allow("selfservice:mail:"+strings.ToLower(target), mailsPerTargetPerHour)
}

// ---------------------------------------------------------------------------
// Forgotten password
// ---------------------------------------------------------------------------

func forgotPasswordPage(p portal) gin.HandlerFunc {
	return func(c *gin.Context) {
		render(c, http.StatusOK, "pages/forgot_password.pongo2", pongo2.Context{"Portal": p})
	}
}

// forgotPasswordSubmit answers identically whether or not the identifier
// matches an account; issuing tokens and queueing mail happens after the
// response so its timing does not reveal a match either.
func forgotPasswordSubmit(p portal) gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := middleware.GetLanguage(c)
		identifier := strings.TrimSpace(c.PostForm("identifier"))
		ctx := pongo2.Context{"Portal": p, "Identifier": identifier}

		if !allowIP(c) {
			ctx["Error"] = translate(lang, "self_service.rate_limited")
			c.Header("Retry-After", "3600")
			render(c, http.StatusTooManyRequests, "pages/forgot_password.pongo2", ctx)
			return
		}
		if identifier == "" {
			ctx["Error"] = translate(lang, "self_service.forgot_password.identifier_required")
			render(c, http.StatusBadRequest, "pages/forgot_password.pongo2", ctx)
			return
		}
		s, ok := storeFor(c)
		if !ok {
			return
		}
		if len(identifier) <= maxIdentifierLen {
			accounts, err := s.findAccounts(c.Request.Context(), p.Type, identifier)
			if err != nil {
				slog.Error("selfservice: account lookup failed", "portal", p.Type, "error", err)
			}
			if len(accounts) > maxAccountsPerRequest {
				accounts = accounts[:maxAccountsPerRequest]
			}
			if len(accounts) > 0 {
				go sendResetLinks(s, p, lang, accounts)
			}
		}
		ctx["Sent"] = true
		ctx["Identifier"] = ""
		render(c, http.StatusOK, "pages/forgot_password.pongo2", ctx)
	}
}

func sendResetLinks(s store, p portal, lang string, accounts []Account) {
	ctx := context.Background()
	for _, acct := range accounts {
		if !allowMailTo(acct.Type + ":" + strconv.Itoa(acct.ID)) {
			slog.Warn("selfservice: reset email budget exhausted", "type", acct.Type, "login", acct.Login)
			continue
		}
		raw, err := s.issueResetToken(ctx, acct)
		if err != nil {
			slog.Error("selfservice: issue reset token", "type", acct.Type, "login", acct.Login, "error", err)
			continue
		}
		link, err := tokenLink(p.ResetURL, raw)
		if err != nil {
			slog.Error("selfservice: cannot build reset link", "error", err)
			return
		}
		subject := translate(lang, "self_service.email.reset_subject")
		body := translate(lang, "self_service.email.reset_body",
			displayName(acct.FirstName, acct.LastName, acct.Login), acct.Login, int(ResetTokenTTL.Minutes()), link)
		if err := queueMail(ctx, s.db, acct.Email, subject, body); err != nil {
			slog.Error("selfservice: queue reset email", "login", acct.Login, "error", err)
		}
	}
}

// ---------------------------------------------------------------------------
// Choosing a new password (reset and registration completion share the page)
// ---------------------------------------------------------------------------

// passwordPage is the template context for pages/reset_password.pongo2.
func passwordPage(lang string, p portal, mode string, policy sysconfig.PasswordPolicy) pongo2.Context {
	ctx := pongo2.Context{
		"Portal":       p,
		"Mode":         mode,
		"Requirements": policyRequirements(lang, policy),
	}
	if mode == "register" {
		ctx["Action"] = registerCompleteURL
		ctx["RetryURL"] = p.RegisterURL
	} else {
		ctx["Action"] = p.ResetURL
		ctx["RetryURL"] = p.ForgotURL
	}
	return ctx
}

func loadPolicy(db *sql.DB, accountType string) sysconfig.PasswordPolicy {
	var (
		policy sysconfig.PasswordPolicy
		err    error
	)
	if accountType == UserAgent {
		policy, err = sysconfig.LoadAgentPasswordPolicy(db)
	} else {
		policy, err = sysconfig.LoadCustomerPasswordPolicy(db)
	}
	if err != nil {
		slog.Error("selfservice: load password policy", "type", accountType, "error", err)
	}
	return policy
}

func policyRequirements(lang string, p sysconfig.PasswordPolicy) []string {
	var out []string
	if p.PasswordMinSize > 0 {
		out = append(out, policyMessage(lang, "min_size", p))
	}
	if p.PasswordMin2Lower2UpperCharacters {
		out = append(out, policyMessage(lang, "min_2_lower_2_upper", p))
	}
	if p.PasswordNeedDigit {
		out = append(out, policyMessage(lang, "need_digit", p))
	}
	if p.PasswordMin2Characters {
		out = append(out, policyMessage(lang, "min_2_characters", p))
	}
	if p.PasswordRegExp != "" {
		out = append(out, policyMessage(lang, "regexp_mismatch", p))
	}
	return out
}

func policyMessage(lang, code string, p sysconfig.PasswordPolicy) string {
	return strings.ReplaceAll(translate(lang, sysconfig.PasswordRequirementKey(code)), "{n}", strconv.Itoa(p.PasswordMinSize))
}

// checkNewPassword validates the submitted password pair against the policy
// and hashes it. A non-empty message is shown to the user.
func checkNewPassword(c *gin.Context, lang string, policy sysconfig.PasswordPolicy) (hash, message string) {
	password := c.PostForm("password")
	if password == "" {
		return "", translate(lang, "self_service.reset_password.required")
	}
	if password != c.PostForm("confirm_password") {
		return "", translate(lang, "password.passwords_no_match")
	}
	if verr := policy.ValidatePassword(password); verr != nil {
		return "", policyMessage(lang, verr.Code, policy)
	}
	hash, err := auth.NewPasswordHasher().HashPassword(password)
	if errors.Is(err, auth.ErrPasswordTooLong) {
		return "", translate(lang, "self_service.reset_password.too_long")
	}
	if err != nil {
		slog.Error("selfservice: hash password", "error", err)
		return "", translate(lang, "self_service.error")
	}
	return hash, ""
}

func invalidLink(c *gin.Context, ctx pongo2.Context, lang string) {
	ctx["Invalid"] = true
	ctx["Error"] = translate(lang, "self_service.reset_password.invalid_link")
	render(c, http.StatusBadRequest, "pages/reset_password.pongo2", ctx)
}

func resetPasswordPage(p portal) gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := middleware.GetLanguage(c)
		s, ok := storeFor(c)
		if !ok {
			return
		}
		ctx := passwordPage(lang, p, "reset", loadPolicy(s.db, p.Type))
		raw := c.Query("token")
		tok, err := s.lookupToken(c.Request.Context(), raw, TokenPasswordReset, p.Type)
		if err == nil {
			_, err = s.accountForToken(c.Request.Context(), p.Type, tok)
		}
		if err != nil {
			if !errors.Is(err, errTokenInvalid) {
				slog.Error("selfservice: reset link check", "error", err)
			}
			invalidLink(c, ctx, lang)
			return
		}
		ctx["Token"] = raw
		render(c, http.StatusOK, "pages/reset_password.pongo2", ctx)
	}
}

func resetPasswordSubmit(p portal) gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := middleware.GetLanguage(c)
		s, ok := storeFor(c)
		if !ok {
			return
		}
		reqCtx := c.Request.Context()
		policy := loadPolicy(s.db, p.Type)
		ctx := passwordPage(lang, p, "reset", policy)
		raw := c.PostForm("token")

		tok, err := s.lookupToken(reqCtx, raw, TokenPasswordReset, p.Type)
		var acct *Account
		if err == nil {
			acct, err = s.accountForToken(reqCtx, p.Type, tok)
		}
		if err != nil {
			if !errors.Is(err, errTokenInvalid) {
				slog.Error("selfservice: reset token check", "error", err)
			}
			invalidLink(c, ctx, lang)
			return
		}
		ctx["Token"] = raw

		hash, msg := checkNewPassword(c, lang, policy)
		if msg != "" {
			ctx["Error"] = msg
			render(c, http.StatusBadRequest, "pages/reset_password.pongo2", ctx)
			return
		}
		if err := s.resetPassword(reqCtx, tok, acct, hash); err != nil {
			if !errors.Is(err, errTokenInvalid) {
				slog.Error("selfservice: reset password", "login", acct.Login, "error", err)
			}
			invalidLink(c, ctx, lang)
			return
		}
		if n, err := s.killSessions(reqCtx, acct); err != nil {
			slog.Error("selfservice: end sessions after reset", "login", acct.Login, "error", err)
		} else {
			slog.Info("selfservice: password reset", "type", acct.Type, "login", acct.Login, "sessions_ended", n)
		}
		auth.DefaultLoginRateLimiter.RecordSuccess(c.ClientIP(), acct.Login)

		delete(ctx, "Token")
		ctx["Done"] = true
		ctx["Message"] = translate(lang, "self_service.reset_password.success")
		render(c, http.StatusOK, "pages/reset_password.pongo2", ctx)
	}
}

// ---------------------------------------------------------------------------
// Customer self-registration
// ---------------------------------------------------------------------------

func registerPage(c *gin.Context) {
	render(c, http.StatusOK, "pages/customer/register.pongo2", pongo2.Context{"Portal": customerPortal})
}

func registerSubmit(c *gin.Context) {
	lang := middleware.GetLanguage(c)
	firstName := strings.TrimSpace(c.PostForm("first_name"))
	lastName := strings.TrimSpace(c.PostForm("last_name"))
	email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))
	ctx := pongo2.Context{"Portal": customerPortal, "FirstName": firstName, "LastName": lastName, "Email": email}

	if !allowIP(c) {
		ctx["Error"] = translate(lang, "self_service.rate_limited")
		c.Header("Retry-After", "3600")
		render(c, http.StatusTooManyRequests, "pages/customer/register.pongo2", ctx)
		return
	}
	if firstName == "" || lastName == "" || email == "" {
		ctx["Error"] = translate(lang, "self_service.register.fields_required")
		render(c, http.StatusBadRequest, "pages/customer/register.pongo2", ctx)
		return
	}
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email || len(email) > maxEmailLen ||
		len(firstName) > maxNameLen || len(lastName) > maxNameLen {
		ctx["Error"] = translate(lang, "self_service.register.invalid_input")
		render(c, http.StatusBadRequest, "pages/customer/register.pongo2", ctx)
		return
	}
	s, ok := storeFor(c)
	if !ok {
		return
	}
	exists, err := customerExists(c.Request.Context(), s.db, email)
	if err != nil {
		slog.Error("selfservice: registration lookup", "error", err)
		c.String(http.StatusServiceUnavailable, "service unavailable")
		return
	}
	go sendRegistrationMail(s, lang, email, firstName, lastName, exists)

	render(c, http.StatusOK, "pages/customer/register.pongo2", pongo2.Context{"Portal": customerPortal, "Sent": true})
}

// sendRegistrationMail emails a confirmation link, or, when the address
// already has an account, a pointer to password reset instead. Both cases
// look the same to whoever submitted the form.
func sendRegistrationMail(s store, lang, email, firstName, lastName string, exists bool) {
	ctx := context.Background()
	if !allowMailTo("register:" + email) {
		slog.Warn("selfservice: registration email budget exhausted", "email", email)
		return
	}
	if exists {
		if !lostPasswordEnabled() {
			return
		}
		base, err := publicBaseURL()
		if err != nil {
			slog.Error("selfservice: cannot build forgot-password link", "error", err)
			return
		}
		body := translate(lang, "self_service.email.register_exists_body", base+customerPortal.ForgotURL)
		if err := queueMail(ctx, s.db, email, translate(lang, "self_service.email.register_exists_subject"), body); err != nil {
			slog.Error("selfservice: queue account-exists email", "error", err)
		}
		return
	}
	raw, err := s.createRegistration(ctx, email, firstName, lastName)
	if err != nil {
		slog.Error("selfservice: create registration", "error", err)
		return
	}
	link, err := tokenLink(registerCompleteURL, raw)
	if err != nil {
		slog.Error("selfservice: cannot build confirmation link", "error", err)
		return
	}
	body := translate(lang, "self_service.email.register_body",
		displayName(firstName, lastName, email), int(VerifyTokenTTL.Hours()), link)
	if err := queueMail(ctx, s.db, email, translate(lang, "self_service.email.register_subject"), body); err != nil {
		slog.Error("selfservice: queue confirmation email", "error", err)
	}
}

func registerCompletePage(c *gin.Context) {
	lang := middleware.GetLanguage(c)
	s, ok := storeFor(c)
	if !ok {
		return
	}
	ctx := passwordPage(lang, customerPortal, "register", loadPolicy(s.db, UserCustomer))
	raw := c.Query("token")
	_, reg, err := s.registrationForToken(c.Request.Context(), raw)
	if err != nil {
		if !errors.Is(err, errTokenInvalid) {
			slog.Error("selfservice: confirmation link check", "error", err)
		}
		invalidLink(c, ctx, lang)
		return
	}
	ctx["Token"] = raw
	ctx["Login"] = reg.Email
	render(c, http.StatusOK, "pages/reset_password.pongo2", ctx)
}

func registerCompleteSubmit(c *gin.Context) {
	lang := middleware.GetLanguage(c)
	s, ok := storeFor(c)
	if !ok {
		return
	}
	reqCtx := c.Request.Context()
	policy := loadPolicy(s.db, UserCustomer)
	ctx := passwordPage(lang, customerPortal, "register", policy)
	raw := c.PostForm("token")
	tok, reg, err := s.registrationForToken(reqCtx, raw)
	if err != nil {
		if !errors.Is(err, errTokenInvalid) {
			slog.Error("selfservice: confirmation token check", "error", err)
		}
		invalidLink(c, ctx, lang)
		return
	}
	ctx["Token"] = raw
	ctx["Login"] = reg.Email

	hash, msg := checkNewPassword(c, lang, policy)
	if msg != "" {
		ctx["Error"] = msg
		render(c, http.StatusBadRequest, "pages/reset_password.pongo2", ctx)
		return
	}
	err = s.completeRegistration(reqCtx, tok, reg, hash)
	switch {
	case errors.Is(err, errAccountExists):
		delete(ctx, "Token")
		ctx["Invalid"] = true
		ctx["Error"] = translate(lang, "self_service.register.account_exists")
		ctx["RetryURL"] = customerPortal.ForgotURL
		ctx["AccountExists"] = true
		render(c, http.StatusConflict, "pages/reset_password.pongo2", ctx)
		return
	case err != nil:
		if !errors.Is(err, errTokenInvalid) {
			slog.Error("selfservice: complete registration", "error", err)
		}
		invalidLink(c, ctx, lang)
		return
	}
	slog.Info("selfservice: customer registered", "login", reg.Email)
	delete(ctx, "Token")
	ctx["Done"] = true
	ctx["Message"] = translate(lang, "self_service.register.complete_success")
	render(c, http.StatusOK, "pages/reset_password.pongo2", ctx)
}
