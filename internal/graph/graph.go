package graph

import (
	"encoding/json"

	"github.com/nicehiro/org-roam-web/internal/db"
)

// Graph represents the note graph
type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Links []GraphLink `json:"links"`
}

// GraphNode represents a node in the graph
type GraphNode struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Tags     []string `json:"tags"`
	LinkCount int     `json:"linkCount"`
}

// GraphLink represents a link in the graph
type GraphLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// BuildGraph creates a graph from nodes and links
func BuildGraph(nodes []db.Node, links []db.Link, nodeTags map[string][]string) *Graph {
	g := &Graph{
		Nodes: make([]GraphNode, 0, len(nodes)),
		Links: make([]GraphLink, 0, len(links)),
	}

	// Build node set for quick lookup
	nodeSet := make(map[string]bool)
	for _, n := range nodes {
		nodeSet[n.ID] = true
	}

	// Count links per node
	linkCount := make(map[string]int)
	for _, l := range links {
		// Only count links where both nodes exist
		if nodeSet[l.Source] && nodeSet[l.Target] {
			linkCount[l.Source]++
			linkCount[l.Target]++
		}
	}

	// Add nodes
	for _, n := range nodes {
		tags := nodeTags[n.ID]
		if tags == nil {
			tags = []string{}
		}
		g.Nodes = append(g.Nodes, GraphNode{
			ID:        n.ID,
			Title:     n.Title,
			Tags:      tags,
			LinkCount: linkCount[n.ID],
		})
	}

	// Add links (only between existing nodes)
	for _, l := range links {
		if nodeSet[l.Source] && nodeSet[l.Target] {
			g.Links = append(g.Links, GraphLink{
				Source: l.Source,
				Target: l.Target,
			})
		}
	}

	return g
}

// ToJSON converts the graph to JSON
func (g *Graph) ToJSON() ([]byte, error) {
	return json.MarshalIndent(g, "", "  ")
}

// LocalGraph creates a subgraph around a specific node
func LocalGraph(nodeID string, depth int, nodes []db.Node, links []db.Link, nodeTags map[string][]string) *Graph {
	// Build adjacency list
	adjacency := make(map[string][]string)
	for _, l := range links {
		adjacency[l.Source] = append(adjacency[l.Source], l.Target)
		adjacency[l.Target] = append(adjacency[l.Target], l.Source)
	}

	// BFS to find nodes within depth
	visited := make(map[string]bool)
	queue := []struct {
		id    string
		depth int
	}{{nodeID, 0}}
	visited[nodeID] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if curr.depth >= depth {
			continue
		}

		for _, neighbor := range adjacency[curr.id] {
			if !visited[neighbor] {
				visited[neighbor] = true
				queue = append(queue, struct {
					id    string
					depth int
				}{neighbor, curr.depth + 1})
			}
		}
	}

	// Build node map for quick lookup
	nodeMap := make(map[string]db.Node)
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}

	// Create subgraph
	g := &Graph{
		Nodes: make([]GraphNode, 0),
		Links: make([]GraphLink, 0),
	}

	// Count links for local nodes
	linkCount := make(map[string]int)
	for _, l := range links {
		if visited[l.Source] && visited[l.Target] {
			linkCount[l.Source]++
			linkCount[l.Target]++
		}
	}

	// Add visited nodes
	for id := range visited {
		if n, ok := nodeMap[id]; ok {
			tags := nodeTags[id]
			if tags == nil {
				tags = []string{}
			}
			g.Nodes = append(g.Nodes, GraphNode{
				ID:        n.ID,
				Title:     n.Title,
				Tags:      tags,
				LinkCount: linkCount[id],
			})
		}
	}

	// Add links between visited nodes
	for _, l := range links {
		if visited[l.Source] && visited[l.Target] {
			g.Links = append(g.Links, GraphLink{
				Source: l.Source,
				Target: l.Target,
			})
		}
	}

	return g
}
