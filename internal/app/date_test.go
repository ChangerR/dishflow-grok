package app

import "testing"

func TestBizDate(t *testing.T) {
	cases := map[string]string{
		"2026-08-14":                "2026-08-14",
		"2026-08-14T00:00:00Z":      "2026-08-14",
		"2026-08-14T08:00:00+08:00": "2026-08-14",
		" 2026-08-14T00:00:00Z ":    "2026-08-14",
	}
	for in, want := range cases {
		if got := bizDate(in); got != want {
			t.Fatalf("bizDate(%q)=%q want %q", in, got, want)
		}
	}
}
