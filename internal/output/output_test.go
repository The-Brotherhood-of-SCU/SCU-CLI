package output

import (
	"bytes"
	"os"
	"testing"
)

// TestWriteToIndentedByDefault 默认输出缩进 JSON（人类排障友好）。
func TestWriteToIndentedByDefault(t *testing.T) {
	t.Setenv("SCU_CLI_COMPACT", "")
	os.Unsetenv("SCU_CLI_COMPACT")

	var buf bytes.Buffer
	if err := writeTo(&buf, Envelope{OK: true, Data: map[string]int{"a": 1}}); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"ok\": true,\n  \"data\": {\n    \"a\": 1\n  }\n}\n"
	if buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}
}

// TestWriteToCompact SCU_CLI_COMPACT 开启时输出单行紧凑 JSON。
func TestWriteToCompact(t *testing.T) {
	t.Setenv("SCU_CLI_COMPACT", "1")

	var buf bytes.Buffer
	if err := writeTo(&buf, Envelope{OK: true, Data: map[string]int{"a": 1}}); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "{\"ok\":true,\"data\":{\"a\":1}}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCompactOutputValues(t *testing.T) {
	for _, v := range []string{"", "0", "false"} {
		t.Setenv("SCU_CLI_COMPACT", v)
		if compactOutput() {
			t.Errorf("SCU_CLI_COMPACT=%q: 应为缩进模式", v)
		}
	}
	for _, v := range []string{"1", "true", "yes"} {
		t.Setenv("SCU_CLI_COMPACT", v)
		if !compactOutput() {
			t.Errorf("SCU_CLI_COMPACT=%q: 应为紧凑模式", v)
		}
	}
}
