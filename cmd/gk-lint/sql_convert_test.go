package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// The fixture marks every line that must be reported with "// want <rule>";
// the rule must report exactly those lines, no more and no fewer.
func TestSQLConversionRulesMatchFixture(t *testing.T) {
	root, _, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	const fixture = "cmd/gk-lint/testdata/sqlconv"

	schema, err := loadSQLSchema(filepath.Join(root, "cmd/gk-lint/testdata/sqlschema"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := scanSQLConversion(root, schema, "./"+fixture)
	if err != nil {
		t.Fatal(err)
	}
	gotSet := make(map[string]bool)
	for _, v := range got {
		gotSet[v.Kind+"@"+strconv.Itoa(v.Line)] = true
	}

	want := make(map[string]bool)
	f, err := os.Open(filepath.Join(root, fixture, "sqlconv.go"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	marker := regexp.MustCompile(`// want (sql-[a-z-]+)`)
	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		if m := marker.FindStringSubmatch(sc.Text()); m != nil {
			want[m[1]+"@"+strconv.Itoa(line)] = true
		}
	}
	if len(want) == 0 {
		t.Fatal("fixture has no want markers")
	}

	for k := range want {
		if !gotSet[k] {
			t.Errorf("missing report %s", k)
		}
	}
	for k := range gotSet {
		if !want[k] {
			t.Errorf("unexpected report %s", k)
		}
	}
}
