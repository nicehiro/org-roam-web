package search

import (
	"encoding/json"

	"github.com/nicehiro/org-roam-web/internal/db"
)

// SearchEntry represents a searchable note
type SearchEntry struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
}

// SearchIndex holds all searchable entries
type SearchIndex struct {
	Entries []SearchEntry `json:"entries"`
}

// BuildIndex creates a search index from nodes
func BuildIndex(nodes []db.Node, nodeTags map[string][]string) *SearchIndex {
	index := &SearchIndex{
		Entries: make([]SearchEntry, 0, len(nodes)),
	}

	for _, n := range nodes {
		tags := nodeTags[n.ID]
		if tags == nil {
			tags = []string{}
		}
		index.Entries = append(index.Entries, SearchEntry{
			ID:    n.ID,
			Title: n.Title,
			Tags:  tags,
		})
	}

	return index
}

// ToJSON converts the index to JSON
func (idx *SearchIndex) ToJSON() ([]byte, error) {
	return json.MarshalIndent(idx, "", "  ")
}
