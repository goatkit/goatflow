//go:build integration

package api

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	goldap "github.com/go-ldap/ldap/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/ldap"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	platformservice "github.com/goatkit/goatflow/internal/platform/service"
	"github.com/goatkit/goatflow/internal/platform/yamlmgmt"
)

const (
	ldapTestBase       = "dc=goatflow,dc=test"
	ldapTestAdminDN    = "cn=admin," + ldapTestBase
	ldapTestAdminPW    = "ldap-admin-pw"
	ldapTestReadonlyDN = "cn=readonly," + ldapTestBase
	ldapTestReadonlyPW = "ldap-readonly-pw"
	ldapTestPeople     = "ou=people," + ldapTestBase
	ldapTestGroups     = "ou=groups," + ldapTestBase
	ldapLoginPrefix    = "ldapt-"
	ldapRunHint        = "run it with `make test-ldap-integration` (needs Docker and the test databases)"
)

// ldapTestServer is an OpenLDAP container seeded with the users below.
type ldapTestServer struct {
	host   string
	port   string
	caFile string
}

type ldapTestUser struct {
	uid, given, sn, mail, password string
	ou                             string
}

var ldapTestUsers = []ldapTestUser{
	{uid: "ldapt-agent", given: "Lisa", sn: "Agent", mail: "lisa.agent@goatflow.test", password: "agent-pass", ou: "people"},
	{uid: "ldapt-admin", given: "Ada", sn: "Admin", mail: "ada.admin@goatflow.test", password: "admin-pass", ou: "people"},
	{uid: "ldapt-qa", given: "Quinn", sn: "Tester", mail: "quinn@goatflow.test", password: "qa-pass", ou: "people"},
	{uid: "ldapt-new", given: "Nora", sn: "New", mail: "nora@goatflow.test", password: "new-pass", ou: "people"},
	{uid: "ldapt-dup", given: "Dup", sn: "One", mail: "dup1@goatflow.test", password: "dup-pass", ou: "people"},
	{uid: "ldapt-dup", given: "Dup", sn: "Two", mail: "dup2@goatflow.test", password: "dup-pass", ou: "contractors"},
}

func ldapUserDN(u ldapTestUser) string { return "uid=" + u.uid + ",ou=" + u.ou + "," + ldapTestBase }

func startLDAPServer(t *testing.T) *ldapTestServer {
	t.Helper()
	ctx := context.Background()
	caPEM, certPEM, keyPEM := ldapTestCertificates(t)
	certFile := func(name string, data []byte) testcontainers.ContainerFile {
		return testcontainers.ContainerFile{Reader: bytes.NewReader(data),
			ContainerFilePath: "/container/service/slapd/assets/certs/" + name, FileMode: 0o644}
	}
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "osixia/openldap:1.5.0",
			ExposedPorts: []string{"389/tcp"},
			Env: map[string]string{
				"LDAP_DOMAIN":                 "goatflow.test",
				"LDAP_ORGANISATION":           "GoatFlow Test",
				"LDAP_ADMIN_PASSWORD":         ldapTestAdminPW,
				"LDAP_READONLY_USER":          "true",
				"LDAP_READONLY_USER_USERNAME": "readonly",
				"LDAP_READONLY_USER_PASSWORD": ldapTestReadonlyPW,
				"LDAP_TLS_CRT_FILENAME":       "server.crt",
				"LDAP_TLS_KEY_FILENAME":       "server.key",
				"LDAP_TLS_CA_CRT_FILENAME":    "test-ca.crt",
				"LDAP_TLS_VERIFY_CLIENT":      "never",
			},
			// Server certificate for localhost signed by a CA made for this run
			// (the image's bundled CA has expired); the test connects to
			// localhost (the make target uses --network host).
			Files: []testcontainers.ContainerFile{
				certFile("test-ca.crt", caPEM), certFile("server.crt", certPEM), certFile("server.key", keyPEM),
			},
			// The image runs a temporary slapd while bootstrapping; "slapd
			// starting" is only logged by the final one.
			WaitingFor: wait.ForAll(
				wait.ForLog("slapd starting"),
				wait.ForListeningPort("389/tcp"),
			).WithDeadline(120 * time.Second),
		},
		Started: true,
	})
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatalf("FATAL: LDAP integration test could not start OpenLDAP: %v\n%s", err, ldapRunHint)
	}
	port, err := ctr.MappedPort(ctx, "389/tcp")
	require.NoError(t, err)
	srv := &ldapTestServer{host: "localhost", port: port.Port()}

	// slapd restarts once while bootstrapping; wait until the admin can bind.
	var conn *goldap.Conn
	deadline := time.Now().Add(90 * time.Second)
	for {
		conn, err = goldap.DialURL("ldap://" + srv.host + ":" + srv.port)
		if err == nil {
			if err = conn.Bind(ldapTestAdminDN, ldapTestAdminPW); err == nil {
				break
			}
			conn.Close()
		}
		if time.Now().After(deadline) {
			t.Fatalf("OpenLDAP never accepted the admin bind: %v", err)
		}
		time.Sleep(time.Second)
	}
	defer conn.Close()
	seedLDAP(t, conn)

	srv.caFile = filepath.Join(t.TempDir(), "ldap-ca.crt")
	require.NoError(t, os.WriteFile(srv.caFile, caPEM, 0o600))
	return srv
}

// ldapTestCertificates returns a throwaway CA and a localhost server
// certificate signed by it, PEM encoded.
func ldapTestCertificates(t *testing.T) (caPEM, certPEM, keyPEM []byte) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "GoatFlow LDAP test CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	ca, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "localhost"},
		DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

func seedLDAP(t *testing.T, conn *goldap.Conn) {
	t.Helper()
	add := func(dn string, attrs map[string][]string) {
		req := goldap.NewAddRequest(dn, nil)
		for k, v := range attrs {
			req.Attribute(k, v)
		}
		require.NoError(t, conn.Add(req), "add %s", dn)
	}
	for _, ou := range []string{"people", "contractors", "groups"} {
		add("ou="+ou+","+ldapTestBase, map[string][]string{"objectClass": {"organizationalUnit"}, "ou": {ou}})
	}
	for _, u := range ldapTestUsers {
		add(ldapUserDN(u), map[string][]string{
			"objectClass":  {"inetOrgPerson"},
			"uid":          {u.uid},
			"cn":           {u.given + " " + u.sn},
			"givenName":    {u.given},
			"sn":           {u.sn},
			"mail":         {u.mail},
			"userPassword": {u.password},
		})
	}
	dn := func(uid string) string { return "uid=" + uid + "," + ldapTestPeople }
	add("cn=helpdesk,"+ldapTestGroups, map[string][]string{"objectClass": {"groupOfNames"}, "cn": {"helpdesk"},
		"member": {dn("ldapt-agent"), dn("ldapt-new"), dn("ldapt-dup")}})
	add("cn=goatflow-admins,"+ldapTestGroups, map[string][]string{"objectClass": {"groupOfNames"}, "cn": {"goatflow-admins"},
		"member": {dn("ldapt-admin")}})
	add("cn=qa,"+ldapTestGroups, map[string][]string{"objectClass": {"groupOfNames"}, "cn": {"qa"},
		"member": {dn("ldapt-qa")}})
}

func (s *ldapTestServer) modify(t *testing.T, dn string, apply func(*goldap.ModifyRequest)) {
	t.Helper()
	conn, err := goldap.DialURL("ldap://" + s.host + ":" + s.port)
	require.NoError(t, err)
	defer conn.Close()
	require.NoError(t, conn.Bind(ldapTestAdminDN, ldapTestAdminPW))
	req := goldap.NewModifyRequest(dn, nil)
	apply(req)
	require.NoError(t, conn.Modify(req))
}

// setLDAPEnv configures LDAP the way an operator would (StartTLS, verified
// against the server's CA, read-only service account, group mapping).
func (s *ldapTestServer) setLDAPEnv(t *testing.T) {
	for k, v := range map[string]string{
		"LDAP_ENABLED":           "true",
		"LDAP_TYPE":              "openldap",
		"LDAP_HOST":              s.host,
		"LDAP_PORT":              s.port,
		"LDAP_USE_TLS":           "true",
		"LDAP_TLS_CA_FILE":       s.caFile,
		"LDAP_TIMEOUT":           "5",
		"LDAP_BIND_DN":           ldapTestReadonlyDN,
		"LDAP_BIND_PASSWORD":     ldapTestReadonlyPW,
		"LDAP_BASE_DN":           ldapTestBase,
		"LDAP_GROUP_BASE_DN":     ldapTestGroups,
		"LDAP_AGENT_GROUPS":      "helpdesk",
		"LDAP_ADMIN_GROUPS":      "goatflow-admins",
		"LDAP_AUTO_CREATE_USERS": "true",
		"LDAP_AUTO_UPDATE_USERS": "true",
		"LDAP_INITIAL_GROUPS":    "users",
	} {
		t.Setenv(k, v)
	}
}

// useAuthProviders makes the next GetAuthService() build the real auth
// service with the given Auth::Providers order and the current environment.
func useAuthProviders(t *testing.T, providers ...string) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "Config.yaml")
	yaml := "version: \"1.0\"\nsettings:\n  - name: \"Auth::Providers\"\n    default: [\"" +
		strings.Join(providers, "\", \"") + "\"]\n"
	require.NoError(t, os.WriteFile(cfgPath, []byte(yaml), 0o600))
	ca := yamlmgmt.NewConfigAdapter(yamlmgmt.NewVersionManager(dir))
	require.NoError(t, ca.ImportConfigYAML(cfgPath))
	platformservice.SetConfigAdapter(ca)
	resetServices()
	t.Cleanup(func() {
		platformservice.SetConfigAdapter(nil)
		resetServices()
	})
}

func resetServices() {
	servicesMu.Lock()
	clearServicesLocked()
	servicesMu.Unlock()
}

func deleteLDAPTestAgents(t *testing.T, db *sql.DB) {
	t.Helper()
	rows, err := db.Query(database.ConvertPlaceholders("SELECT id FROM users WHERE login LIKE ?"), ldapLoginPrefix+"%")
	require.NoError(t, err)
	var ids []int64
	for rows.Next() {
		var id int64
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())
	rows.Close()
	for _, id := range ids {
		for _, q := range []string{
			// LDAP sync records the agent as its own change_by (FK to users).
			"UPDATE users SET change_by = 1 WHERE id = ?",
			"DELETE FROM group_user WHERE user_id = ?",
			"DELETE FROM user_preferences WHERE user_id = ?",
			"DELETE FROM users WHERE id = ?",
		} {
			_, err := db.Exec(database.ConvertPlaceholders(q), id)
			require.NoError(t, err)
		}
	}
}

func postLogin(t *testing.T, login, password string) (int, map[string]any) {
	t.Helper()
	r := gin.New()
	r.POST("/api/auth/login", HandleAuthLogin)
	body, _ := json.Marshal(map[string]string{"login": login, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), "body: %s", w.Body.String())
	return w.Code, out
}

type agentRow struct {
	id              int64
	first, last, pw string
	validID         int
	email           string
	groups          []string
	found           bool
}

func loadAgent(t *testing.T, db *sql.DB, login string) agentRow {
	t.Helper()
	var a agentRow
	err := db.QueryRow(database.ConvertPlaceholders(
		"SELECT id, first_name, last_name, pw, valid_id FROM users WHERE login = ?"), login).
		Scan(&a.id, &a.first, &a.last, &a.pw, &a.validID)
	if errors.Is(err, sql.ErrNoRows) {
		return a
	}
	require.NoError(t, err)
	a.found = true
	var email []byte
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT preferences_value FROM user_preferences WHERE user_id = ? AND preferences_key = 'UserEmail'"), a.id).Scan(&email)
	if !errors.Is(err, sql.ErrNoRows) {
		require.NoError(t, err)
	}
	a.email = string(email)
	rows, err := db.Query(database.ConvertPlaceholders(
		"SELECT g.name FROM group_user gu JOIN `groups` g ON g.id = gu.group_id WHERE gu.user_id = ? AND gu.permission_key = 'rw' ORDER BY g.name"), a.id)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var g string
		require.NoError(t, rows.Scan(&g))
		a.groups = append(a.groups, g)
	}
	require.NoError(t, rows.Err())
	return a
}

func TestLDAPLoginIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.GetDB()
	require.NoError(t, err, ldapRunHint)
	require.NotNil(t, db, ldapRunHint)

	srv := startLDAPServer(t)
	srv.setLDAPEnv(t)
	deleteLDAPTestAgents(t, db)
	t.Cleanup(func() { deleteLDAPTestAgents(t, db) })

	// A database-only agent: not in the directory, so LDAP passes it on.
	hash, err := auth.NewPasswordHasher().HashPassword("db-only-pass")
	require.NoError(t, err)
	validID, err := lookups.ID(context.Background(), db, lookups.ValidLookup, "valid")
	require.NoError(t, err)
	now := time.Now()
	_, err = database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, title, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, '', 'Dee', 'Bee', ?, ?, 1, ?, 1) RETURNING id`), "ldapt-dbonly", hash, validID, now, now)
	require.NoError(t, err)

	useAuthProviders(t, "ldap", "database")

	t.Run("directory agent logs in and gets an account", func(t *testing.T) {
		code, body := postLogin(t, "ldapt-agent", "agent-pass")
		require.Equal(t, http.StatusOK, code, "body: %v", body)
		assert.Equal(t, true, body["success"])
		assert.NotEmpty(t, body["access_token"])

		a := loadAgent(t, db, "ldapt-agent")
		require.True(t, a.found, "agent account created on first login")
		assert.Equal(t, "Lisa", a.first)
		assert.Equal(t, "Agent", a.last)
		assert.Equal(t, "lisa.agent@goatflow.test", a.email)
		assert.Equal(t, []string{"users"}, a.groups, "LDAP_INITIAL_GROUPS, and no admin rights")
		assert.Empty(t, a.pw, "LDAP accounts get no local password")
		assert.Equal(t, validID, a.validID)
	})

	t.Run("wrong password is rejected", func(t *testing.T) {
		code, body := postLogin(t, "ldapt-agent", "not-the-password")
		assert.Equal(t, http.StatusUnauthorized, code)
		assert.Equal(t, false, body["success"])
	})

	t.Run("login is not a filter", func(t *testing.T) {
		for _, login := range []string{"*", "ldapt-a*", "ldapt-agent)(uid=*", "*)(|(uid=*"} {
			code, _ := postLogin(t, login, "agent-pass")
			assert.Equal(t, http.StatusUnauthorized, code, "login %q", login)
		}
	})

	t.Run("user not in LDAP falls through to the database", func(t *testing.T) {
		code, body := postLogin(t, "ldapt-dbonly", "db-only-pass")
		assert.Equal(t, http.StatusOK, code, "body: %v", body)
		code, _ = postLogin(t, "ldapt-dbonly", "wrong")
		assert.Equal(t, http.StatusUnauthorized, code)
	})

	t.Run("user outside LDAP_AGENT_GROUPS is refused", func(t *testing.T) {
		code, _ := postLogin(t, "ldapt-qa", "qa-pass")
		assert.Equal(t, http.StatusUnauthorized, code)
		assert.False(t, loadAgent(t, db, "ldapt-qa").found, "no account for a refused user")
	})

	t.Run("filter matching two entries is refused", func(t *testing.T) {
		code, _ := postLogin(t, "ldapt-dup", "dup-pass")
		assert.Equal(t, http.StatusUnauthorized, code)
		assert.False(t, loadAgent(t, db, "ldapt-dup").found)
	})

	t.Run("admin group membership follows LDAP_ADMIN_GROUPS", func(t *testing.T) {
		code, body := postLogin(t, "ldapt-admin", "admin-pass")
		require.Equal(t, http.StatusOK, code, "body: %v", body)
		assert.Equal(t, []string{"admin", "users"}, loadAgent(t, db, "ldapt-admin").groups)

		srv.modify(t, "cn=goatflow-admins,"+ldapTestGroups, func(m *goldap.ModifyRequest) {
			m.Replace("member", []string{"uid=ldapt-agent," + ldapTestPeople})
		})
		code, _ = postLogin(t, "ldapt-admin", "admin-pass")
		require.Equal(t, http.StatusUnauthorized, code, "no longer an admin and not in helpdesk")

		code, _ = postLogin(t, "ldapt-agent", "agent-pass")
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, []string{"admin", "users"}, loadAgent(t, db, "ldapt-agent").groups)
		srv.modify(t, "cn=goatflow-admins,"+ldapTestGroups, func(m *goldap.ModifyRequest) {
			m.Replace("member", []string{"uid=ldapt-admin," + ldapTestPeople})
		})
		code, _ = postLogin(t, "ldapt-agent", "agent-pass")
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, []string{"users"}, loadAgent(t, db, "ldapt-agent").groups, "admin removed when LDAP membership ends")
	})

	t.Run("name and email are synced on login", func(t *testing.T) {
		srv.modify(t, "uid=ldapt-agent,"+ldapTestPeople, func(m *goldap.ModifyRequest) {
			m.Replace("givenName", []string{"Elisabeth"})
			m.Replace("mail", []string{"elisabeth@goatflow.test"})
		})
		code, _ := postLogin(t, "ldapt-agent", "agent-pass")
		require.Equal(t, http.StatusOK, code)
		a := loadAgent(t, db, "ldapt-agent")
		assert.Equal(t, "Elisabeth", a.first)
		assert.Equal(t, "elisabeth@goatflow.test", a.email)
	})

	t.Run("invalid GoatFlow account is refused", func(t *testing.T) {
		invalidID, err := lookups.ID(context.Background(), db, lookups.ValidLookup, "invalid")
		require.NoError(t, err)
		_, err = db.Exec(database.ConvertPlaceholders("UPDATE users SET valid_id = ? WHERE login = ?"), invalidID, "ldapt-agent")
		require.NoError(t, err)
		code, _ := postLogin(t, "ldapt-agent", "agent-pass")
		assert.Equal(t, http.StatusUnauthorized, code)
		_, err = db.Exec(database.ConvertPlaceholders("UPDATE users SET valid_id = ? WHERE login = ?"), validID, "ldapt-agent")
		require.NoError(t, err)
	})

	t.Run("without auto-create an unknown agent cannot log in", func(t *testing.T) {
		t.Setenv("LDAP_AUTO_CREATE_USERS", "false")
		useAuthProviders(t, "ldap", "database")
		code, _ := postLogin(t, "ldapt-new", "new-pass")
		assert.Equal(t, http.StatusUnauthorized, code)
		assert.False(t, loadAgent(t, db, "ldapt-new").found)
	})
}

// TestLDAPClientIntegration covers the directory client directly: empty
// passwords and certificate verification.
func TestLDAPClientIntegration(t *testing.T) {
	srv := startLDAPServer(t)
	srv.setLDAPEnv(t)
	cfg, err := ldap.LoadFromEnvironment()
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("verified StartTLS and correct password", func(t *testing.T) {
		c, err := ldap.NewClient(cfg)
		require.NoError(t, err)
		u, err := c.Authenticate(ctx, "ldapt-agent", "agent-pass")
		require.NoError(t, err)
		assert.Equal(t, "uid=ldapt-agent,"+ldapTestPeople, u.DN)
		assert.Equal(t, "ldapt-agent", u.Username)
		assert.Equal(t, []string{"helpdesk"}, u.Groups)
	})

	t.Run("empty password never authenticates", func(t *testing.T) {
		c, err := ldap.NewClient(cfg)
		require.NoError(t, err)
		_, err = c.Authenticate(ctx, "ldapt-agent", "")
		assert.ErrorIs(t, err, ldap.ErrInvalidCredentials)
	})

	t.Run("unknown user", func(t *testing.T) {
		c, err := ldap.NewClient(cfg)
		require.NoError(t, err)
		_, err = c.Authenticate(ctx, "ldapt-nobody", "x")
		assert.ErrorIs(t, err, ldap.ErrUserNotFound)
	})

	t.Run("certificate is verified by default", func(t *testing.T) {
		noCA := *cfg
		noCA.CACertFile = ""
		c, err := ldap.NewClient(&noCA)
		require.NoError(t, err)
		_, err = c.Authenticate(ctx, "ldapt-agent", "agent-pass")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "certificate", "self-signed server certificate must be rejected without LDAP_TLS_CA_FILE")
	})

	t.Run("wrong service account password fails closed", func(t *testing.T) {
		bad := *cfg
		bad.BindPassword = "wrong"
		c, err := ldap.NewClient(&bad)
		require.NoError(t, err)
		_, err = c.Authenticate(ctx, "ldapt-agent", "agent-pass")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "service account bind")
	})
}
