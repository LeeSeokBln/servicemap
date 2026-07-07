package render

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/LeeSeokBln/servicemap/internal/graph"
)

// testGraph builds the canonical nginx -> myapp -> mariadb + docker + external graph.
func testGraph() *graph.Graph {
	lo := netip.MustParseAddr("127.0.0.1")
	return &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "unit:mariadb.service", Name: "mariadb.service", Kind: graph.KindSystemd,
				Listens: []graph.ListenAddr{{Port: 3306, Proto: "tcp"}}, PIDs: []int{300}},
			{ID: "unit:myapp.service", Name: "myapp.service", Kind: graph.KindSystemd,
				Listens: []graph.ListenAddr{{Port: 3000, Proto: "tcp"}}, PIDs: []int{200}},
			{ID: "unit:nginx.service", Name: "nginx.service", Kind: graph.KindSystemd,
				Listens: []graph.ListenAddr{{Port: 80, Proto: "tcp"}, {Port: 443, Proto: "tcp"}}, PIDs: []int{100, 101}},
			{ID: "docker:abc", Name: "webapp", Kind: graph.KindDocker,
				Listens: []graph.ListenAddr{{IP: lo, Port: 8080, Proto: "tcp"}}, PIDs: []int{400}},
			{ID: "ext:10.0.0.5:5432", Name: "10.0.0.5:5432", Kind: graph.KindExternal},
		},
		Edges: []*graph.Edge{
			{From: "unit:myapp.service", To: "unit:mariadb.service", Kind: graph.EdgeConnects, Ports: []uint16{3306}},
			{From: "unit:myapp.service", To: "ext:10.0.0.5:5432", Kind: graph.EdgeConnects, Ports: []uint16{5432}},
			{From: "unit:nginx.service", To: "unit:myapp.service", Kind: graph.EdgeProxies, Ports: []uint16{3000}},
		},
	}
}

func TestTree(t *testing.T) {
	got := Tree(testGraph())
	want := `mariadb.service
  └─ listens :3306

myapp.service
  └─ listens :3000
  └─ connects to mariadb.service (:3306)
  └─ connects to 10.0.0.5:5432

nginx.service
  └─ listens :80, :443
  └─ proxies to myapp.service (:3000)

webapp [docker]
  └─ listens 127.0.0.1:8080
`
	if got != want {
		t.Errorf("Tree output mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestTreeNodeNoListensNoEdges(t *testing.T) {
	// Plain process with no listens and no edges
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "proc:orphan", Name: "orphan (pid 999)", Kind: graph.KindProcess},
		},
		Edges: []*graph.Edge{},
	}
	got := Tree(g)
	want := "orphan (pid 999)\n"
	if got != want {
		t.Errorf("orphan node output mismatch:\ngot: %q\nwant: %q", got, want)
	}
}

func TestTreeNodeOnlyListensNoEdges(t *testing.T) {
	// Node with listens but no outbound edges
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "unit:standalone.service", Name: "standalone.service", Kind: graph.KindSystemd,
				Listens: []graph.ListenAddr{{Port: 9999, Proto: "tcp"}}, PIDs: []int{50}},
		},
		Edges: []*graph.Edge{},
	}
	got := Tree(g)
	want := "standalone.service\n  └─ listens :9999\n"
	if got != want {
		t.Errorf("listen-only node output mismatch:\ngot: %q\nwant: %q", got, want)
	}
}

func TestTreeExternalNodeFiltered(t *testing.T) {
	// External nodes should not appear as standalone nodes
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "ext:external.com:443", Name: "external.com:443", Kind: graph.KindExternal},
		},
		Edges: []*graph.Edge{},
	}
	got := Tree(g)
	want := "" // External nodes with no incoming edges should not render
	if got != want {
		t.Errorf("external node should not render:\ngot: %q\nwant: %q", got, want)
	}
}

func TestTreeNilSafetyAssumption(t *testing.T) {
	// Verify the assumption: edges only reference nodes that exist
	// This test documents the contract that graph.finish() maintains
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "unit:a.service", Name: "a.service", Kind: graph.KindSystemd,
				Listens: []graph.ListenAddr{{Port: 80, Proto: "tcp"}}, PIDs: []int{100}},
			{ID: "ext:10.0.0.5:5432", Name: "10.0.0.5:5432", Kind: graph.KindExternal},
		},
		Edges: []*graph.Edge{
			{From: "unit:a.service", To: "ext:10.0.0.5:5432", Kind: graph.EdgeConnects, Ports: []uint16{5432}},
		},
	}
	// This should not panic - the edge references a node that exists
	got := Tree(g)
	if !strings.Contains(got, "connects to 10.0.0.5:5432") {
		t.Errorf("external edge not rendered: %q", got)
	}
}
