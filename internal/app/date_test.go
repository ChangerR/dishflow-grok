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

func TestParseOptionSnapshot(t *testing.T) {
	names, ids := parseOptionSnapshot(`[{"id":"o1","name":"加辣"}]`)
	if names[0] != "加辣" || ids[0] != "o1" {
		t.Fatalf("%v %v", names, ids)
	}
	names, ids = parseOptionSnapshot(`["加蛋"]`)
	if names[0] != "加蛋" || len(ids) != 0 {
		t.Fatalf("%v %v", names, ids)
	}
}
