package render

import (
	"net/netip"
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
