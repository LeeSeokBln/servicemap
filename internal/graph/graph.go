// Package graph turns a collect.Snapshot into a service graph:
// nodes (services) with listen addresses, and edges (proxies/connects).
package graph

import (
	"fmt"
	"net/netip"
	"sort"

	"github.com/LeeSeokBln/servicemap/internal/collect"
)

type NodeKind string

const (
	KindSystemd  NodeKind = "systemd"
	KindDocker   NodeKind = "docker"
	KindProcess  NodeKind = "process"
	KindExternal NodeKind = "external"
)

type ListenAddr struct {
	IP    netip.Addr // zero value = wildcard bind
	Port  uint16
	Proto string // "tcp" or "udp"
}

func (l ListenAddr) String() string {
	s := ""
	if l.IP.IsValid() {
		s = l.IP.String()
	}
	s += fmt.Sprintf(":%d", l.Port)
	if l.Proto == "udp" {
		s += "/udp"
	}
	return s
}

type Node struct {
	ID      string
	Name    string
	Kind    NodeKind
	Listens []ListenAddr
	PIDs    []int
}

type EdgeKind string

const (
	EdgeProxies  EdgeKind = "proxies"
	EdgeConnects EdgeKind = "connects"
)

type Edge struct {
	From  string // node ID
	To    string // node ID
	Kind  EdgeKind
	Ports []uint16
}

type Graph struct {
	Nodes    []*Node
	Edges    []*Edge
	Warnings []string
}

type Options struct {
	All bool // disable the noise filter
}

type portKey struct {
	addr netip.Addr // zero value = wildcard
	port uint16
}

type builder struct {
	snap      *collect.Snapshot
	nodes     map[string]*Node
	edges     map[string]*Edge
	listeners map[portKey]string // tcp listeners -> node ID
	localIPs  map[netip.Addr]bool
	warnings  []string
}

func Build(snap *collect.Snapshot, opts Options) *Graph {
	b := &builder{
		snap:      snap,
		nodes:     map[string]*Node{},
		edges:     map[string]*Edge{},
		listeners: map[portKey]string{},
		localIPs:  map[netip.Addr]bool{},
	}
	inodes := map[uint64]collect.Socket{}
	for _, s := range snap.Sockets {
		inodes[s.Inode] = s
		if s.LocalIP.IsValid() && !s.LocalIP.IsUnspecified() {
			b.localIPs[s.LocalIP] = true
		}
	}
	b.buildNodes(inodes)
	b.buildConnectEdges(inodes)
	g := b.finish(opts)
	g.Warnings = append(append([]string{}, snap.Warnings...), b.warnings...)
	return g
}

// nodeID computes the grouping key for a process.
func nodeID(p collect.Process) string {
	switch {
	case p.Unit != "":
		return "unit:" + p.Unit
	case p.ContainerID != "":
		return "docker:" + p.ContainerID
	default:
		return "proc:" + p.Comm
	}
}

func (b *builder) buildNodes(inodes map[uint64]collect.Socket) {
	for _, p := range b.snap.Processes {
		id := nodeID(p)
		node := b.nodes[id]
		if node == nil {
			node = &Node{ID: id}
			switch {
			case p.Unit != "":
				node.Kind, node.Name = KindSystemd, p.Unit
			case p.ContainerID != "":
				node.Kind = KindDocker
				name := b.snap.DockerNames[p.ContainerID]
				if name == "" {
					name = p.ContainerID
					if len(name) > 12 {
						name = name[:12]
					}
				}
				node.Name = name
			default:
				node.Kind, node.Name = KindProcess, p.Comm
			}
			b.nodes[id] = node
		}
		node.PIDs = append(node.PIDs, p.PID)
		for _, ino := range p.SocketInodes {
			s, ok := inodes[ino]
			if !ok || s.State != collect.StateListen {
				continue
			}
			node.Listens = append(node.Listens, ListenAddr{IP: s.LocalIP, Port: s.LocalPort, Proto: s.Proto})
			if s.Proto == "tcp" {
				k := portKey{port: s.LocalPort}
				if !s.LocalIP.IsUnspecified() {
					k.addr = s.LocalIP.Unmap()
				}
				b.listeners[k] = id
			}
		}
	}
	for _, n := range b.nodes {
		sort.Ints(n.PIDs)
		n.Listens = normalizeListens(n.Listens)
		if n.Kind == KindProcess {
			if len(n.PIDs) == 1 {
				n.Name = fmt.Sprintf("%s (pid %d)", n.Name, n.PIDs[0])
			} else {
				n.Name = fmt.Sprintf("%s (%d procs)", n.Name, len(n.PIDs))
			}
		}
	}
}

// normalizeListens dedupes binds: per (proto, port), any wildcard bind
// collapses the group to a single wildcard entry.
func normalizeListens(in []ListenAddr) []ListenAddr {
	type pk struct {
		proto string
		port  uint16
	}
	byPort := map[pk][]ListenAddr{}
	for _, l := range in {
		k := pk{l.Proto, l.Port}
		byPort[k] = append(byPort[k], l)
	}
	var out []ListenAddr
	for k, addrs := range byPort {
		wildcard := false
		seen := map[netip.Addr]bool{}
		var specific []ListenAddr
		for _, l := range addrs {
			if !l.IP.IsValid() || l.IP.IsUnspecified() {
				wildcard = true
			} else if !seen[l.IP] {
				seen[l.IP] = true
				specific = append(specific, ListenAddr{IP: l.IP, Port: l.Port, Proto: l.Proto})
			}
		}
		if wildcard {
			out = append(out, ListenAddr{Port: k.port, Proto: k.proto})
		} else {
			out = append(out, specific...)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Port != out[j].Port {
			return out[i].Port < out[j].Port
		}
		if out[i].Proto != out[j].Proto {
			return out[i].Proto < out[j].Proto
		}
		return out[i].String() < out[j].String()
	})
	return out
}

var kindRank = map[NodeKind]int{KindSystemd: 0, KindDocker: 1, KindProcess: 2, KindExternal: 3}

func (b *builder) finish(opts Options) *Graph {
	keep := map[string]bool{}
	if opts.All {
		for id := range b.nodes {
			keep[id] = true
		}
	} else {
		for id, n := range b.nodes {
			if len(n.Listens) > 0 && n.Kind != KindExternal {
				keep[id] = true
			}
		}
		// systemd/docker services with only outbound connections still count
		for _, e := range b.edges {
			if n := b.nodes[e.From]; n.Kind == KindSystemd || n.Kind == KindDocker {
				keep[e.From] = true
			}
		}
		// externals survive only when a kept node references them
		for _, e := range b.edges {
			if keep[e.From] && b.nodes[e.To].Kind == KindExternal {
				keep[e.To] = true
			}
		}
	}
	g := &Graph{}
	for id, n := range b.nodes {
		if keep[id] {
			g.Nodes = append(g.Nodes, n)
		}
	}
	sort.Slice(g.Nodes, func(i, j int) bool {
		a, c := g.Nodes[i], g.Nodes[j]
		if kindRank[a.Kind] != kindRank[c.Kind] {
			return kindRank[a.Kind] < kindRank[c.Kind]
		}
		if a.Name != c.Name {
			return a.Name < c.Name
		}
		return a.ID < c.ID
	})
	idx := map[string]int{}
	for i, n := range g.Nodes {
		idx[n.ID] = i
	}
	for _, e := range b.edges {
		if keep[e.From] && keep[e.To] {
			g.Edges = append(g.Edges, e)
		}
	}
	sort.Slice(g.Edges, func(i, j int) bool {
		a, c := g.Edges[i], g.Edges[j]
		if idx[a.From] != idx[c.From] {
			return idx[a.From] < idx[c.From]
		}
		return idx[a.To] < idx[c.To]
	})
	return g
}

func (b *builder) buildConnectEdges(inodes map[uint64]collect.Socket) {
	for _, p := range b.snap.Processes {
		from := nodeID(p)
		node := b.nodes[from]
		listenPorts := map[uint16]bool{}
		for _, l := range node.Listens {
			if l.Proto == "tcp" {
				listenPorts[l.Port] = true
			}
		}
		for _, ino := range p.SocketInodes {
			s, ok := inodes[ino]
			if !ok || s.State != collect.StateEstablished || s.Proto != "tcp" {
				continue
			}
			if listenPorts[s.LocalPort] {
				continue // inbound: server side of someone else's connection
			}
			remoteIP := s.RemoteIP.Unmap()
			to := b.lookupListener(remoteIP, s.RemotePort)
			if to == "" {
				if b.isLocalAddr(remoteIP) {
					continue // local endpoint without listener: mid-scan race
				}
				to = b.externalNode(fmt.Sprintf("%s:%d", remoteIP, s.RemotePort))
			}
			if to == from {
				continue
			}
			b.addEdge(from, to, EdgeConnects, s.RemotePort)
		}
	}
}

func (b *builder) lookupListener(ip netip.Addr, port uint16) string {
	if id, ok := b.listeners[portKey{addr: ip, port: port}]; ok {
		return id
	}
	if ip.IsLoopback() || b.isLocalAddr(ip) {
		if id, ok := b.listeners[portKey{port: port}]; ok {
			return id
		}
	}
	return ""
}

func (b *builder) isLocalAddr(ip netip.Addr) bool {
	return ip.IsLoopback() || b.localIPs[ip]
}

func (b *builder) addEdge(from, to string, kind EdgeKind, port uint16) {
	key := from + "|" + to
	e := b.edges[key]
	if e == nil {
		e = &Edge{From: from, To: to, Kind: kind}
		b.edges[key] = e
	}
	if kind == EdgeProxies {
		e.Kind = EdgeProxies // config knowledge wins over runtime
	}
	if port != 0 {
		for _, p := range e.Ports {
			if p == port {
				return
			}
		}
		e.Ports = append(e.Ports, port)
		sort.Slice(e.Ports, func(i, j int) bool { return e.Ports[i] < e.Ports[j] })
	}
}

// externalNode returns the aggregated external node ID for a remote
// "host:port", creating the node on first reference.
func (b *builder) externalNode(name string) string {
	id := "ext:" + name
	if b.nodes[id] == nil {
		b.nodes[id] = &Node{ID: id, Name: name, Kind: KindExternal}
	}
	return id
}
