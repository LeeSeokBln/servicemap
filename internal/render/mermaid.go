package render

import (
	"fmt"
	"strings"

	"github.com/LeeSeokBln/servicemap/internal/graph"
)

func Mermaid(g *graph.Graph) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	ids := map[string]string{}
	for i, n := range g.Nodes {
		ids[n.ID] = fmt.Sprintf("n%d", i)
	}
	for _, n := range g.Nodes {
		label := mermaidEscape(nodeTitle(n))
		if len(n.Listens) > 0 {
			label += "<br/>" + mermaidEscape(joinListens(n.Listens))
		}
		if n.Kind == graph.KindExternal {
			b.WriteString(fmt.Sprintf("    %s([\"%s\"])\n", ids[n.ID], label))
		} else {
			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", ids[n.ID], label))
		}
	}
	for _, e := range g.Edges {
		arrow, verb := "-.->", "connects"
		if e.Kind == graph.EdgeProxies {
			arrow, verb = "-->", "proxies"
		}
		label := verb
		for _, p := range e.Ports {
			label += fmt.Sprintf(" :%d", p)
		}
		b.WriteString(fmt.Sprintf("    %s %s|%s| %s\n", ids[e.From], arrow, label, ids[e.To]))
	}
	return b.String()
}

// mermaidEscape neutralizes characters that would break a quoted mermaid
// label. Realistic node names (systemd units, docker names, host:port) never
// contain these, but /proc-derived strings are not fully trusted.
func mermaidEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, "#quot;", "\n", " ", "\r", " ")
	return r.Replace(s)
}
