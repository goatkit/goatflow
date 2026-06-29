package main

import "testing"

func TestIsProductPackage(t *testing.T) {
	modulePath := "github.com/goatkit/goatflow"

	cases := []struct {
		name       string
		importPath string
		want       bool
	}{
		{
			name:       "product package",
			importPath: "github.com/goatkit/goatflow/internal/models",
			want:       true,
		},
		{
			name:       "product subpackage",
			importPath: "github.com/goatkit/goatflow/internal/services/scheduler",
			want:       true,
		},
		{
			name:       "platform package",
			importPath: "github.com/goatkit/goatflow/internal/platform/models",
			want:       false,
		},
		{
			name:       "platform subpackage",
			importPath: "github.com/goatkit/goatflow/internal/platform/plugin/loader",
			want:       false,
		},
		{
			name:       "public sdk",
			importPath: "github.com/goatkit/goatflow/pkg/plugin",
			want:       false,
		},
		{
			name:       "stdlib internal package",
			importPath: "internal/abi",
			want:       false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isProductPackage(tc.importPath, modulePath)
			if got != tc.want {
				t.Fatalf("isProductPackage(%q) = %v, want %v", tc.importPath, got, tc.want)
			}
		})
	}
}

func TestAllowedImport(t *testing.T) {
	old := allowedProductImports
	allowedProductImports = []allowRule{{
		Import: "github.com/goatkit/goatflow/internal/models",
		Reason: "test exception",
	}}
	t.Cleanup(func() { allowedProductImports = old })

	if _, ok := allowedImport("github.com/goatkit/goatflow/internal/models"); !ok {
		t.Fatal("exact allowlist import was not honored")
	}
	if _, ok := allowedImport("github.com/goatkit/goatflow/internal/models/subpkg"); !ok {
		t.Fatal("subpackage allowlist import was not honored")
	}
	if _, ok := allowedImport("github.com/goatkit/goatflow/internal/repository"); ok {
		t.Fatal("unlisted product import was allowed")
	}
}
