package main

import "testing"

func FuzzParseProductID(f *testing.F) {
	f.Add("1")
	f.Add("42")
	f.Add("0")
	f.Add("-1")
	f.Add("not-a-number")
	f.Add("")
	f.Fuzz(func(t *testing.T, raw string) {
		id, err := parseProductID(raw)
		if err == nil && id <= 0 {
			t.Fatalf("parseProductID(%q) returned non-positive ID %d", raw, id)
		}
	})
}
