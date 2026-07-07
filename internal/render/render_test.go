package render

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMarkdown(t *testing.T) {
	got := Markdown(testGraph())
	for _, want := range []string{
		"# Service Map",
		"| Service | Kind | Listens | Connections |",
		"| nginx.service | systemd | :80, :443 | proxies to myapp.service (:3000) |",
		"| myapp.service | systemd | :3000 | connects to mariadb.service (:3306); connects to 10.0.0.5:5432 |",
		"| webapp | docker | 127.0.0.1:8080 |  |",
		"```mermaid",
		"flowchart LR",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("markdown missing %q:\n%s", want, got)
		}
	}
}

func TestJSON(t *testing.T) {
	got, err := JSON(testGraph())
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Nodes []struct {
			ID      string   `json:"id"`
			Name    string   `json:"name"`
			Kind    string   `json:"kind"`
			Listens []string `json:"listens"`
			PIDs    []int    `json:"pids"`
		} `json:"nodes"`
		Edges []struct {
			From  string   `json:"from"`
			To    string   `json:"to"`
			Kind  string   `json:"kind"`
			Ports []uint16 `json:"ports"`
		} `json:"edges"`
	}
	if err := json.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Nodes) != 5 || len(doc.Edges) != 3 {
		t.Fatalf("nodes=%d edges=%d", len(doc.Nodes), len(doc.Edges))
	}
	if doc.Nodes[2].Name != "nginx.service" || doc.Nodes[2].Listens[0] != ":80" {
		t.Errorf("bad node: %+v", doc.Nodes[2])
	}
	if doc.Edges[2].Kind != "proxies" {
		t.Errorf("bad edge: %+v", doc.Edges[2])
	}
}

func TestRenderDispatch(t *testing.T) {
	g := testGraph()
	for _, f := range []string{"tree", "mermaid", "md", "json"} {
		out, err := Render(g, f)
		if err != nil || out == "" {
			t.Errorf("format %s: err=%v empty=%v", f, err, out == "")
		}
	}
	if _, err := Render(g, "yaml"); err == nil {
		t.Error("unknown format must error")
	}
}

func TestMarkdownEscapesPipes(t *testing.T) {
	g := testGraph()
	g.Nodes[0].Name = "weird|name.service"
	got := Markdown(g)
	if !strings.Contains(got, `weird\|name.service`) {
		t.Errorf("pipe in node name must be escaped:\n%s", got)
	}
}
