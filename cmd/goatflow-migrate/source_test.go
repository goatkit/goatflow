package main

import "testing"

func TestDBSourceIdentEscapesQuoteCharacters(t *testing.T) {
	cases := []struct {
		driver, name, want string
	}{
		{"mysql", "ticket", "`ticket`"},
		{"mysql", "odd`name", "`odd``name`"},
		{"postgres", "ticket", `"ticket"`},
		{"postgres", `odd"name`, `"odd""name"`},
		// Only the active dialect's quote character is special.
		{"postgres", "odd`name", "\"odd`name\""},
		{"mysql", `odd"name`, "`odd\"name`"},
	}
	for _, tc := range cases {
		s := &dbSource{driver: tc.driver}
		if got := s.ident(tc.name); got != tc.want {
			t.Errorf("ident(%q) on %s = %s, want %s", tc.name, tc.driver, got, tc.want)
		}
	}
}
