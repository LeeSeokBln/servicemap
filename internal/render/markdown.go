package render

import (
	"fmt"
	"strings"

	"github.com/LeeSeokBln/servicemap/internal/graph"
)

// Markdown formats g as a markdown report: a service table followed by an
// embedded mermaid diagram.
func Markdown(g *graph.Graph) string {
	byID := map[string]*graph.Node{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	out := map[string][]*graph.Edge{}
	for _, e := range g.Edges {
		out[e.From] = append(out[e.From], e)
	}
	var b strings.Builder
	b.WriteString("# Service Map\n\n")
	b.WriteString("| Service | Kind | Listens | Connections |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, n := range g.Nodes {
		if n.Kind == graph.KindExternal {
			continue
		}
		var conns []string
		for _, e := range out[n.ID] {
			conns = append(conns, edgeText(e, byID))
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			markdownEscape(n.Name), n.Kind,
			markdownEscape(joinListens(n.Listens)),
			markdownEscape(strings.Join(conns, "; "))))
	}
	b.WriteString("\n```mermaid\n")
	b.WriteString(Mermaid(g))
	b.WriteString("```\n")
	return b.String()
}

// markdownEscape keeps table cells intact when a /proc-derived name contains
// a pipe character.
func markdownEscape(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}
