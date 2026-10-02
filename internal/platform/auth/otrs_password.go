package auth

import (
	"crypto/md5"  //nolint:gosec // verifying legacy OTRS md5-crypt hashes, never creating them
	"crypto/sha1" //nolint:gosec // verifying legacy OTRS sha1 hashes, never creating them
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Base64 alphabets: bcrypt's, and crypt(3)'s itoa64 used by md5-crypt.
const (
	bcryptAlphabet = "./ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	cryptAlphabet  = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

var (
	bcryptBase64   = base64.NewEncoding(bcryptAlphabet).WithPadding(base64.NoPadding)
	otrsBcryptHash = regexp.MustCompile(`^BCRYPT:(\d+):(.{16}):(.{31})$`)
)

// verifyOTRSHash checks the password formats an OTRS 6 / Znuny 6.x-7.x users.pw
// or customer_user.pw can hold, following Kernel/System/Auth/DB.pm and
// Kernel/System/CustomerAuth/DB.pm:
//
//   - sha2 (default CryptType): lowercase hex SHA-256, 64 chars
//   - sha512: lowercase hex SHA-512, 128 chars
//   - sha1: lowercase hex SHA-1, 40 chars
//   - md5 / apr1: "$1$salt$hash" (unix md5-crypt) and "$apr1$salt$hash" // sql-ok: crypt prefix, not SQL
//   - bcrypt: "BCRYPT:<cost>:<16-char salt>:<31-char hash>" (Crypt::Eksblowfish
//     with key_nul, i.e. standard bcrypt with the 16 salt characters as the
//     raw salt bytes)
//
// Not verified: CryptType "plain" (OTRS only compares plaintext when that type
// is configured, which GoatFlow cannot know) and DES crypt(3).
func verifyOTRSHash(password, stored string) bool {
	switch {
	case stored == "":
		return false
	case strings.HasPrefix(stored, "$1$"): // sql-ok: md5-crypt prefix, not SQL
		return constantTimeEqual(md5Crypt(password, stored, "$1$"), stored) // sql-ok: md5-crypt prefix
	case strings.HasPrefix(stored, "$apr1$"):
		return constantTimeEqual(md5Crypt(password, stored, "$apr1$"), stored)
	case strings.HasPrefix(stored, "BCRYPT:"):
		std, ok := otrsBcryptToStandard(stored)
		return ok && bcrypt.CompareHashAndPassword([]byte(std), []byte(password)) == nil
	case !isLowerHex(stored):
		return false
	case len(stored) == 64:
		return constantTimeEqual(hashSHA256(password), stored)
	case len(stored) == 128:
		sum := sha512.Sum512([]byte(password))
		return constantTimeEqual(hex.EncodeToString(sum[:]), stored)
	case len(stored) == 40:
		sum := sha1.Sum([]byte(password)) //nolint:gosec // legacy OTRS verification
		return constantTimeEqual(hex.EncodeToString(sum[:]), stored)
	default:
		return false
	}
}

// otrsBcryptToStandard rewrites OTRS's "BCRYPT:cost:salt:hash" as the
// equivalent "$2a$cost$<salt><hash>" string the bcrypt package verifies.
func otrsBcryptToStandard(stored string) (string, bool) {
	m := otrsBcryptHash.FindStringSubmatch(stored)
	if m == nil {
		return "", false
	}
	cost, err := strconv.Atoi(m[1])
	if err != nil || cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return "", false
	}
	return fmt.Sprintf("$2a$%02d$%s%s", cost, bcryptBase64.EncodeToString([]byte(m[2])), m[3]), true
}

// md5Crypt computes the FreeBSD md5-crypt string for password using the salt
// embedded in stored ("<magic><salt>$<hash>"), as Crypt::PasswdMD5's
// unix_md5_crypt ($1$) and apache_md5_crypt ($apr1$) do. // sql-ok: crypt prefix, not SQL
func md5Crypt(password, stored, magic string) string {
	salt := strings.TrimPrefix(stored, magic)
	if i := strings.IndexByte(salt, '$'); i >= 0 {
		salt = salt[:i]
	}
	if len(salt) > 8 {
		salt = salt[:8]
	}
	pw := []byte(password)

	alt := md5.New() //nolint:gosec // legacy OTRS verification
	alt.Write(pw)
	alt.Write([]byte(salt))
	alt.Write(pw)
	altSum := alt.Sum(nil)

	d := md5.New() //nolint:gosec // legacy OTRS verification
	d.Write(pw)
	d.Write([]byte(magic))
	d.Write([]byte(salt))
	for n := len(pw); n > 0; n -= 16 {
		d.Write(altSum[:min(n, 16)])
	}
	for n := len(pw); n > 0; n >>= 1 {
		if n&1 != 0 {
			d.Write([]byte{0})
		} else {
			d.Write(pw[:1])
		}
	}
	final := d.Sum(nil)

	for i := range 1000 {
		r := md5.New() //nolint:gosec // legacy OTRS verification
		if i&1 != 0 {
			r.Write(pw)
		} else {
			r.Write(final)
		}
		if i%3 != 0 {
			r.Write([]byte(salt))
		}
		if i%7 != 0 {
			r.Write(pw)
		}
		if i&1 != 0 {
			r.Write(final)
		} else {
			r.Write(pw)
		}
		final = r.Sum(nil)
	}

	var out strings.Builder
	out.WriteString(magic + salt + "$")
	to64 := func(v uint32, n int) {
		for ; n > 0; n-- {
			out.WriteByte(cryptAlphabet[v&0x3f])
			v >>= 6
		}
	}
	for _, g := range [][3]int{{0, 6, 12}, {1, 7, 13}, {2, 8, 14}, {3, 9, 15}, {4, 10, 5}} {
		to64(uint32(final[g[0]])<<16|uint32(final[g[1]])<<8|uint32(final[g[2]]), 4)
	}
	to64(uint32(final[11]), 2)
	return out.String()
}
