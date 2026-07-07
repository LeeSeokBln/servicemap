package render

import (
	"encoding/json"

	"github.com/LeeSeokBln/servicemap/internal/graph"
)

type jsonNode struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Kind    string   `json:"kind"`
	Listens []string `json:"listens"`
	PIDs    []int    `json:"pids,omitempty"`
}

type jsonEdge struct {
	From  string   `json:"from"`
	To    string   `json:"to"`
	Kind  string   `json:"kind"`
	Ports []uint16 `json:"ports,omitempty"`
}

type jsonDoc struct {
	Nodes    []jsonNode `json:"nodes"`
	Edges    []jsonEdge `json:"edges"`
	Warnings []string   `json:"warnings,omitempty"`
}

func JSON(g *graph.Graph) (string, error) {
	doc := jsonDoc{Nodes: []jsonNode{}, Edges: []jsonEdge{}, Warnings: g.Warnings}
	for _, n := range g.Nodes {
		listens := []string{}
		for _, l := range n.Listens {
			listens = append(listens, l.String())
		}
		doc.Nodes = append(doc.Nodes, jsonNode{
			ID: n.ID, Name: n.Name, Kind: string(n.Kind), Listens: listens, PIDs: n.PIDs,
		})
	}
	for _, e := range g.Edges {
		doc.Edges = append(doc.Edges, jsonEdge{From: e.From, To: e.To, Kind: string(e.Kind), Ports: e.Ports})
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}
