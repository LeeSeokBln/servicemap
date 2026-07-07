package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LeeSeokBln/servicemap/internal/testfix"
)

func runCmd(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := newRootCmd()
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), errBuf.String(), err
}

func TestE2ETree(t *testing.T) {
	root := testfix.WriteRoot(t)
	got, _, err := runCmd(t, "--proc-root", root)
	if err != nil {
		t.Fatal(err)
	}
	want := `mariadb.service
  └─ listens :3306

myapp.service
  └─ listens :3000
  └─ connects to mariadb.service (:3306)

nginx.service
  └─ listens :80
  └─ proxies to myapp.service (:3000)

abcdef123456 [docker]
  └─ listens :8080
`
	if got != want {
		t.Errorf("tree mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if strings.Contains(got, "cachetool") {
		t.Error("noise filter failed: cachetool visible without --all")
	}
}

func TestE2EAll(t *testing.T) {
	root := testfix.WriteRoot(t)
	got, _, err := runCmd(t, "--proc-root", root, "--all")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "cachetool (pid 500)") {
		t.Errorf("--all must show cachetool:\n%s", got)
	}
	if !strings.Contains(got, "connects to nginx.service (:80)") {
		t.Errorf("--all must show cachetool's edge:\n%s", got)
	}
}

func TestE2EJSON(t *testing.T) {
	root := testfix.WriteRoot(t)
	got, _, err := runCmd(t, "--proc-root", root, "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, got)
	}
	if len(doc.Nodes) != 4 {
		t.Errorf("got %d nodes, want 4", len(doc.Nodes))
	}
}

func TestE2EOutputFileInfersFormat(t *testing.T) {
	root := testfix.WriteRoot(t)
	out := filepath.Join(t.TempDir(), "diagram.mmd")
	_, _, err := runCmd(t, "--proc-root", root, "--output", out)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "flowchart LR") {
		t.Errorf(".mmd output must be mermaid:\n%s", data)
	}
}

func TestE2EBadFormat(t *testing.T) {
	root := testfix.WriteRoot(t)
	_, _, err := runCmd(t, "--proc-root", root, "--format", "yaml")
	if err == nil {
		t.Error("unknown format must error")
	}
}

func TestResolveFormat(t *testing.T) {
	cases := []struct{ flag, output, want string }{
		{"", "", "tree"},
		{"json", "x.mmd", "json"}, // explicit flag wins
		{"", "diagram.mmd", "mermaid"},
		{"", "report.md", "md"},
		{"", "map.json", "json"},
		{"", "notes.txt", "tree"},
	}
	for _, c := range cases {
		got, err := resolveFormat(c.flag, c.output)
		if err != nil || got != c.want {
			t.Errorf("resolveFormat(%q,%q) = %q,%v want %q", c.flag, c.output, got, err, c.want)
		}
	}
	if _, err := resolveFormat("yaml", ""); err == nil {
		t.Error("bad flag must error")
	}
}
