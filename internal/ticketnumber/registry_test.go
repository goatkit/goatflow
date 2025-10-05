package ticketnumber

import (
    "strings"
    "testing"
)

func TestResolve_KnownGenerators(t *testing.T) {
    cases := []string{"Increment", "Date", "DateChecksum", "Random"}
    for _, cfgName := range cases {
        g, err := Resolve(cfgName, "10", fixedClock{})
        if err != nil { t.Fatalf("unexpected error for %s: %v", cfgName, err) }
        if g == nil { t.Fatalf("nil generator for %s", cfgName) }
        wantInternal := cfgName
        if strings.EqualFold(cfgName, "Increment") { wantInternal = "AutoIncrement" }
        if g.Name() != wantInternal { t.Fatalf("expected internal name %s got %s (config %s)", wantInternal, g.Name(), cfgName) }
    }
}

func TestResolve_InvalidName(t *testing.T) {
    if _, err := Resolve("DoesNotExist", "10", nil); err == nil { t.Fatalf("expected error for invalid name") }
}
