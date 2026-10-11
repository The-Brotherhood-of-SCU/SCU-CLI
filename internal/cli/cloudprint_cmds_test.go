package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/api"
)

func captureCloudPrintCommand(t *testing.T, args ...string) (map[string]interface{}, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old; r.Close() }()
	cmd := newCloudPrintCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	runErr := cmd.Execute()
	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	var envelope map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("bad CLI JSON: %s", buf.String())
	}
	return envelope, runErr
}

func TestCloudPrintDryRunWorksWithoutLoginAndKeepsRange(t *testing.T) {
	t.Setenv("SCU_CLI_CONFIG_DIR", t.TempDir())
	path := filepath.Join(t.TempDir(), "document with spaces.pdf")
	os.WriteFile(path, []byte("%PDF fixture"), 0600)
	envelope, err := captureCloudPrintCommand(t, "upload", "--file", path, "--duplex", "short-edge", "--copies", "2", "--pages", "2-3", "--dry-run")
	if err != nil || envelope["ok"] != true {
		t.Fatalf("err=%v envelope=%v", err, envelope)
	}
	data := envelope["data"].(map[string]interface{})
	fields := data["fields"].(map[string]interface{})
	if data["dry_run"] != true || fields["dwDuplex"] != "3" || fields["dwFrom"] != "2" || fields["dwTo"] != "3" || fields["dwCopies"] != "2" {
		t.Fatalf("data=%v", data)
	}
}

func TestCloudPrintInvalidInputIsNotAuthenticationError(t *testing.T) {
	t.Setenv("SCU_CLI_CONFIG_DIR", t.TempDir())
	for _, args := range [][]string{{"upload"}, {"upload", "missing.pdf"}, {"upload", "missing.pdf", "--pages", "3-2"}, {"upload", "a.pdf", "--file", "b.pdf"}, {"upload", "missing.pdf", "--copies", "0"}} {
		envelope, err := captureCloudPrintCommand(t, args...)
		if err == nil || envelope["error"].(map[string]interface{})["kind"] != "input" {
			t.Fatalf("args=%v err=%v envelope=%v", args, err, envelope)
		}
	}
	if got := errorKind(&api.CloudPrintUploadUncertainError{Reason: "timeout"}); got != "upload_uncertain" {
		t.Fatal(got)
	}
}
