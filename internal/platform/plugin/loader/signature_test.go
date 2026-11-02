package loader_test

import (
	"context"
	"crypto/ed25519"
	"errors"
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

// Reload, which hot reload and health-check restarts use, checks the
// signature again: a module replaced on disk after it was loaded is refused
// and the running plugin stays.
func TestLoaderReloadRequiresSignedWASM(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	wasmBytes, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "plugins", "hello-wasm", "hello.wasm"))
	require.NoError(t, err)
	pub, priv, err := signing.GenerateKeyPair()
	require.NoError(t, err)
	verifier := signing.Verifier{Required: true, Keys: []ed25519.PublicKey{pub}}

	pluginDir := t.TempDir()
	module := filepath.Join(pluginDir, "hello-wasm", "hello.wasm")
	require.NoError(t, os.MkdirAll(filepath.Dir(module), 0o755))
	require.NoError(t, os.WriteFile(module, wasmBytes, 0o644))
	require.NoError(t, signing.SignBinary(module, module+".sig", priv))

	ctx := context.Background()
	mgr := plugin.NewManager(&mockHostAPI{})
	l := loader.NewLoader(pluginDir, mgr, nil, loader.WithVerifier(verifier))
	t.Cleanup(func() { _ = mgr.Unregister(ctx, "hello-wasm") })
	n, errs := l.LoadAll(ctx)
	require.Empty(t, errs)
	require.Equal(t, 1, n)
	loaded, ok := mgr.Get("hello-wasm")
	require.True(t, ok)

	// A valid module with different bytes (an extra custom section), left
	// with the old signature.
	tampered := append(append([]byte{}, wasmBytes...), 0x00, 0x06, 0x04, 't', 'e', 's', 't', 0x01)
	require.NoError(t, os.WriteFile(module, tampered, 0o644))
	err = l.Reload(ctx, "hello-wasm")
	require.ErrorIs(t, err, signing.ErrUntrusted)
	current, ok := mgr.Get("hello-wasm")
	require.True(t, ok)
	require.Same(t, loaded, current, "the running plugin is kept")

	require.NoError(t, os.Remove(module+".sig"))
	require.ErrorIs(t, l.Reload(ctx, "hello-wasm"), signing.ErrUntrusted)

	require.NoError(t, signing.SignBinary(module, module+".sig", priv))
	require.NoError(t, l.Reload(ctx, "hello-wasm"))
	current, ok = mgr.Get("hello-wasm")
	require.True(t, ok)
	require.NotSame(t, loaded, current, "a re-signed module is reloaded")
}

// A gRPC plugin binary is refused on reload before it is executed.
func TestLoaderReloadRequiresSignedGRPC(t *testing.T) {
	pub, _, err := signing.GenerateKeyPair()
	require.NoError(t, err)
	pluginDir := t.TempDir()
	dir := filepath.Join(pluginDir, "unsigned-grpc")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.yaml"),
		[]byte("name: unsigned-grpc\nversion: \"1.0.0\"\nruntime: grpc\nbinary: unsigned-grpc\n"), 0o644))
	marker := filepath.Join(dir, "ran")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "unsigned-grpc"),
		[]byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755)) // #nosec G306 -- test plugin must be executable

	mgr := plugin.NewManager(&mockHostAPI{})
	l := loader.NewLoader(pluginDir, mgr, nil,
		loader.WithVerifier(signing.Verifier{Required: true, Keys: []ed25519.PublicKey{pub}}))
	_, err = l.DiscoverAll()
	require.NoError(t, err)

	err = l.Reload(context.Background(), "unsigned-grpc")
	require.ErrorIs(t, err, signing.ErrUntrusted)
	_, statErr := os.Stat(marker)
	require.True(t, errors.Is(statErr, os.ErrNotExist), "the unsigned binary was not run")
}
