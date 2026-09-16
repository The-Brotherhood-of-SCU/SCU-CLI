package cli

import "testing"

func TestNormalizePasspointMac(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"B8782EBDCE85", "B8782EBDCE85"},
		{"b8782ebdce85", "B8782EBDCE85"},
		{"b8:78:2e:bd:ce:85", "B8782EBDCE85"},
		{"b8-78-2e-bd-ce-85", "B8782EBDCE85"},
		{" B8782EBDCE85 ", "B8782EBDCE85"},
	} {
		got, err := normalizePasspointMac(c.in)
		if err != nil || got != c.want {
			t.Fatalf("normalize(%q) = %q, %v；期望 %q", c.in, got, err, c.want)
		}
	}
	for _, bad := range []string{"", "AA:BB:CC:DD:EE", "AABBCCDDEEFF0", "GGHHIIJJKKLL", "AABBCCDDEEF:"} {
		if _, err := normalizePasspointMac(bad); err == nil {
			t.Fatalf("normalize(%q) 应报格式错误", bad)
		}
	}
}
