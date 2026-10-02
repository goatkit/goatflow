package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The generated test data used to be written to migrations/postgres as version
// 000004, which collides with the real 000004 migration and stops golang-migrate
// ("duplicate migration file"). It must land outside migrations/, readable only
// by the owner because it holds plaintext passwords.
func TestSynthesizeTestDataStaysOutOfMigrations(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	require.NoError(t, NewSynthesizer(".env").SynthesizeTestData())

	_, err := os.Stat(filepath.Join(dir, "migrations"))
	assert.True(t, os.IsNotExist(err), "synthesize must not create anything under migrations/")

	info, err := os.Stat(filepath.Join(dir, GeneratedTestDataPath))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}
