package graph

import (
	"net/netip"
	"testing"

	"github.com/LeeSeokBln/servicemap/internal/collect"
)

func addr(s string) netip.Addr { return netip.MustParseAddr(s) }

func nodeByName(g *Graph, name string) *Node {
	for _, n := range g.Nodes {
		if n.Name == name {
			return n
		}
	}
	return nil
}

func TestBuildNodesAndListens(t *testing.T) {
	snap := &collect.Snapshot{
		Sockets: []collect.Socket{
			{Proto: "tcp", LocalIP: addr("0.0.0.0"), LocalPort: 80, State: collect.StateListen, Inode: 1},
			{Proto: "tcp", LocalIP: addr("::"), LocalPort: 80, State: collect.StateListen, Inode: 2},
			{Proto: "tcp", LocalIP: addr("127.0.0.1"), LocalPort: 8125, State: collect.StateListen, Inode: 3},
			{Proto: "udp", LocalIP: addr("0.0.0.0"), LocalPort: 53, State: collect.StateListen, Inode: 4},
		},
		Processes: []collect.Process{
			{PID: 10, Comm: "nginx", Unit: "nginx.service", SocketInodes: []uint64{1, 2}},
			{PID: 11, Comm: "nginx", Unit: "nginx.service"}, // worker, no sockets
			{PID: 20, Comm: "statsd", SocketInodes: []uint64{3}},
			{PID: 30, Comm: "dnsmasq", Unit: "dnsmasq.service", SocketInodes: []uint64{4}},
		},
	}
	g := Build(snap, Options{})

	nginx := nodeByName(g, "nginx.service")
	if nginx == nil {
		t.Fatal("nginx.service node missing")
	}
	if nginx.Kind != KindSystemd || len(nginx.PIDs) != 2 {
		t.Errorf("bad nginx node: %+v", nginx)
	}
	// two wildcard binds on :80 collapse to one entry
	if len(nginx.Listens) != 1 || nginx.Listens[0].String() != ":80" {
		t.Errorf("nginx listens = %v", nginx.Listens)
	}

	statsd := nodeByName(g, "statsd (pid 20)")
	if statsd == nil || statsd.Kind != KindProcess {
		t.Fatalf("statsd node missing or wrong kind: %+v", statsd)
	}
	if len(statsd.Listens) != 1 || statsd.Listens[0].String() != "127.0.0.1:8125" {
		t.Errorf("statsd listens = %v", statsd.Listens)
	}

	dnsmasq := nodeByName(g, "dnsmasq.service")
	if dnsmasq == nil || len(dnsmasq.Listens) != 1 || dnsmasq.Listens[0].String() != ":53/udp" {
		t.Errorf("dnsmasq = %+v", dnsmasq)
	}
}

func TestDockerNodeNaming(t *testing.T) {
	longID := "abcdef123456abcdef123456abcdef123456abcdef123456abcdef123456abcd"
	snap := &collect.Snapshot{
		Sockets: []collect.Socket{
			{Proto: "tcp", LocalIP: addr("0.0.0.0"), LocalPort: 8080, State: collect.StateListen, Inode: 1},
			{Proto: "tcp", LocalIP: addr("0.0.0.0"), LocalPort: 9090, State: collect.StateListen, Inode: 2},
		},
		Processes: []collect.Process{
			{PID: 40, Comm: "webapp", ContainerID: longID, SocketInodes: []uint64{1}},
			{PID: 41, Comm: "other", ContainerID: "1234567890ab", SocketInodes: []uint64{2}},
		},
		DockerNames: map[string]string{longID: "webapp"},
	}
	g := Build(snap, Options{})
	if n := nodeByName(g, "webapp"); n == nil || n.Kind != KindDocker {
		t.Errorf("named container missing: %+v", n)
	}
	// unresolved name falls back to short ID
	if n := nodeByName(g, "1234567890ab"); n == nil || n.Kind != KindDocker {
		t.Errorf("short-ID fallback missing: %+v", n)
	}
}

func TestNodeSortOrder(t *testing.T) {
	snap := &collect.Snapshot{
		Sockets: []collect.Socket{
			{Proto: "tcp", LocalIP: addr("0.0.0.0"), LocalPort: 1, State: collect.StateListen, Inode: 1},
			{Proto: "tcp", LocalIP: addr("0.0.0.0"), LocalPort: 2, State: collect.StateListen, Inode: 2},
			{Proto: "tcp", LocalIP: addr("0.0.0.0"), LocalPort: 3, State: collect.StateListen, Inode: 3},
		},
		Processes: []collect.Process{
			{PID: 1, Comm: "zzz", SocketInodes: []uint64{1}},
			{PID: 2, Comm: "app", ContainerID: "1234567890ab", SocketInodes: []uint64{2}},
			{PID: 3, Comm: "mariadbd", Unit: "mariadb.service", SocketInodes: []uint64{3}},
		},
	}
	g := Build(snap, Options{})
	if len(g.Nodes) != 3 {
		t.Fatalf("got %d nodes", len(g.Nodes))
	}
	// systemd < docker < process
	if g.Nodes[0].Kind != KindSystemd || g.Nodes[1].Kind != KindDocker || g.Nodes[2].Kind != KindProcess {
		t.Errorf("wrong order: %v %v %v", g.Nodes[0].Kind, g.Nodes[1].Kind, g.Nodes[2].Kind)
	}
}

func edgeBetween(g *Graph, from, to string) *Edge {
	for _, e := range g.Edges {
		if e.From == from && e.To == to {
			return e
		}
	}
	return nil
}

func connectSnap() *collect.Snapshot {
	return &collect.Snapshot{
		Sockets: []collect.Socket{
			// listeners
			{Proto: "tcp", LocalIP: addr("0.0.0.0"), LocalPort: 3000, State: collect.StateListen, Inode: 201},
			{Proto: "tcp", LocalIP: addr("0.0.0.0"), LocalPort: 3306, State: collect.StateListen, Inode: 301},
			// myapp -> mariadb (client side)
			{Proto: "tcp", LocalIP: addr("127.0.0.1"), LocalPort: 52000, RemoteIP: addr("127.0.0.1"), RemotePort: 3306, State: collect.StateEstablished, Inode: 202},
			// mariadb's server side of the same connection (must be filtered)
			{Proto: "tcp", LocalIP: addr("127.0.0.1"), LocalPort: 3306, RemoteIP: addr("127.0.0.1"), RemotePort: 52000, State: collect.StateEstablished, Inode: 302},
			// myapp -> external db
			{Proto: "tcp", LocalIP: addr("192.168.0.10"), LocalPort: 52001, RemoteIP: addr("10.0.0.5"), RemotePort: 5432, State: collect.StateEstablished, Inode: 203},
		},
		Processes: []collect.Process{
			{PID: 200, Comm: "myapp", Unit: "myapp.service", SocketInodes: []uint64{201, 202, 203}},
			{PID: 300, Comm: "mariadbd", Unit: "mariadb.service", SocketInodes: []uint64{301, 302}},
		},
	}
}

func TestConnectEdges(t *testing.T) {
	g := Build(connectSnap(), Options{})

	e := edgeBetween(g, "unit:myapp.service", "unit:mariadb.service")
	if e == nil {
		t.Fatal("myapp -> mariadb edge missing")
	}
	if e.Kind != EdgeConnects || len(e.Ports) != 1 || e.Ports[0] != 3306 {
		t.Errorf("bad edge: %+v", e)
	}

	// inbound side must NOT create mariadb -> external(52000) edge
	for _, e := range g.Edges {
		if e.From == "unit:mariadb.service" {
			t.Errorf("unexpected edge from mariadb: %+v", e)
		}
	}

	ext := edgeBetween(g, "unit:myapp.service", "ext:10.0.0.5:5432")
	if ext == nil {
		t.Fatal("external edge missing")
	}
	extNode := nodeByName(g, "10.0.0.5:5432")
	if extNode == nil || extNode.Kind != KindExternal {
		t.Errorf("external node missing: %+v", extNode)
	}
}
