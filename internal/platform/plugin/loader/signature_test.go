package loader_test

import (
	"context"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/plugin/loader"
	"github.com/goatkit/goatflow/internal/platform/plugin/signing"
)

// A loader that requires signatures loads a WASM module only with a .sig
// from a trusted key.
func TestLoaderRequiresSignedWASM(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	wasmBytes, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "plugins", "hello-wasm", "hello.wasm"))
	require.NoError(t, err)
	pub, priv, err := signing.GenerateKeyPair()
	require.NoError(t, err)
	_, otherPriv, err := signing.GenerateKeyPair()
	require.NoError(t, err)
	verifier := signing.Verifier{Required: true, Keys: []ed25519.PublicKey{pub}}

	for _, tc := range []struct {
		name   string
		signer ed25519.PrivateKey
		loaded bool
	}{
		{name: "unsigned", signer: nil},
		{name: "signed by an untrusted key", signer: otherPriv},
		{name: "signed by a trusted key", signer: priv, loaded: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pluginDir := t.TempDir()
			module := filepath.Join(pluginDir, "hello-wasm", "hello.wasm")
			require.NoError(t, os.MkdirAll(filepath.Dir(module), 0o755))
			require.NoError(t, os.WriteFile(module, wasmBytes, 0o644))
			if tc.signer != nil {
				require.NoError(t, signing.SignBinary(module, module+".sig", tc.signer))
			}

			ctx := context.Background()
			mgr := plugin.NewManager(&mockHostAPI{})
			l := loader.NewLoader(pluginDir, mgr, nil, loader.WithVerifier(verifier))
			t.Cleanup(func() { _ = mgr.Unregister(ctx, "hello-wasm") })

			n, errs := l.LoadAll(ctx)
			_, registered := mgr.Get("hello-wasm")
			require.Equal(t, tc.loaded, registered)
			if tc.loaded {
				require.Equal(t, 1, n)
				require.Empty(t, errs)
			} else {
				require.Zero(t, n)
				require.NotEmpty(t, errs, "refusal is reported")
			}
		})
	}
}
