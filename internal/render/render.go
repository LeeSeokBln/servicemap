package render

import (
	"fmt"

	"github.com/LeeSeokBln/servicemap/internal/graph"
)

// Render formats g as tree, mermaid, md, or json.
func Render(g *graph.Graph, format string) (string, error) {
	switch format {
	case "tree":
		return Tree(g), nil
	case "mermaid":
		return Mermaid(g), nil
	case "md":
		return Markdown(g), nil
	case "json":
		return JSON(g)
	}
	return "", fmt.Errorf("unknown format %q (valid: tree, mermaid, md, json)", format)
}
