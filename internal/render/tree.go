// Package render formats a graph.Graph as tree, mermaid, markdown, or JSON.
package render

import (
	"fmt"
	"strings"

	"github.com/LeeSeokBln/servicemap/internal/graph"
)

// Tree formats g as the default terminal tree view.
func Tree(g *graph.Graph) string {
	byID := map[string]*graph.Node{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	out := map[string][]*graph.Edge{}
	for _, e := range g.Edges {
		out[e.From] = append(out[e.From], e)
	}
	var b strings.Builder
	first := true
	for _, n := range g.Nodes {
		if n.Kind == graph.KindExternal {
			continue
		}
		if !first {
			b.WriteString("\n")
		}
		first = false
		b.WriteString(nodeTitle(n) + "\n")
		if len(n.Listens) > 0 {
			b.WriteString("  └─ listens " + joinListens(n.Listens) + "\n")
		}
		for _, e := range out[n.ID] {
			b.WriteString("  └─ " + edgeText(e, byID) + "\n")
		}
	}
	return b.String()
}

func nodeTitle(n *graph.Node) string {
	if n.Kind == graph.KindDocker {
		return n.Name + " [docker]"
	}
	return n.Name
}

func joinListens(ls []graph.ListenAddr) string {
	parts := make([]string, len(ls))
	for i, l := range ls {
		parts[i] = l.String()
	}
	return strings.Join(parts, ", ")
}

func edgeText(e *graph.Edge, byID map[string]*graph.Node) string {
	verb := "connects to"
	if e.Kind == graph.EdgeProxies {
		verb = "proxies to"
	}
	to := byID[e.To]
	if to.Kind == graph.KindExternal {
		return verb + " " + to.Name // name already carries host:port
	}
	s := verb + " " + nodeTitle(to)
	if len(e.Ports) > 0 {
		ports := make([]string, len(e.Ports))
		for i, p := range e.Ports {
			ports[i] = fmt.Sprintf(":%d", p)
		}
		s += " (" + strings.Join(ports, ", ") + ")"
	}
	return s
}
