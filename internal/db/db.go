package db

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// Node represents an org-roam node
type Node struct {
	ID         string
	File       string
	Level      int
	Pos        int
	Title      string
	Tags       []string
	Properties map[string]string
}

// Link represents a link between nodes
type Link struct {
	Source string
	Target string
	Type   string
}

// DB wraps the org-roam SQLite database
type DB struct {
	db *sql.DB
}

// Open opens the org-roam database
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite3", path+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &DB{db: db}, nil
}

// Close closes the database connection
func (d *DB) Close() error {
	return d.db.Close()
}

// LoadNodes loads all nodes from the database
func (d *DB) LoadNodes() ([]Node, error) {
	rows, err := d.db.Query(`
		SELECT n.id, n.file, n.level, n.pos, n.title, n.properties
		FROM nodes n
		WHERE n.level = 0
		ORDER BY n.file DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		var propsStr sql.NullString
		var titleStr sql.NullString
		var fileStr string

		if err := rows.Scan(&n.ID, &fileStr, &n.Level, &n.Pos, &titleStr, &propsStr); err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}

		// Clean ID and file path (remove quotes)
		n.ID = trimQuotes(n.ID)
		n.File = trimQuotes(fileStr)

		if titleStr.Valid {
			n.Title = cleanTitle(titleStr.String)
		}

		// Parse properties (elisp format)
		if propsStr.Valid {
			n.Properties = parseElispProps(propsStr.String)
		}

		nodes = append(nodes, n)
	}

	return nodes, rows.Err()
}

// LoadTags loads all tags for nodes
func (d *DB) LoadTags() (map[string][]string, error) {
	rows, err := d.db.Query(`SELECT node_id, tag FROM tags`)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags: %w", err)
	}
	defer rows.Close()

	tags := make(map[string][]string)
	for rows.Next() {
		var nodeID, tag string
		if err := rows.Scan(&nodeID, &tag); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		// Clean nodeID and tag strings (remove quotes)
		nodeID = trimQuotes(nodeID)
		tag = trimQuotes(tag)
		tags[nodeID] = append(tags[nodeID], tag)
	}

	return tags, rows.Err()
}

// LoadLinks loads all links between nodes
func (d *DB) LoadLinks() ([]Link, error) {
	rows, err := d.db.Query(`
		SELECT source, dest, type 
		FROM links 
		WHERE type = '"id"'
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query links: %w", err)
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var l Link
		var linkType string
		if err := rows.Scan(&l.Source, &l.Target, &linkType); err != nil {
			return nil, fmt.Errorf("failed to scan link: %w", err)
		}
		l.Type = trimQuotes(linkType)
		// Clean IDs (remove quotes)
		l.Source = trimQuotes(l.Source)
		l.Target = trimQuotes(l.Target)
		links = append(links, l)
	}

	return links, rows.Err()
}

// GetAllTags returns all unique tags
func (d *DB) GetAllTags() ([]string, error) {
	rows, err := d.db.Query(`SELECT DISTINCT tag FROM tags ORDER BY tag`)
	if err != nil {
		return nil, fmt.Errorf("failed to query distinct tags: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tag = trimQuotes(tag)
		tags = append(tags, tag)
	}

	return tags, rows.Err()
}

// trimQuotes removes surrounding double quotes from a string
func trimQuotes(s string) string {
	return strings.Trim(s, "\"")
}

// cleanTitle removes quotes and unescapes Lisp-style escapes from title
func cleanTitle(s string) string {
	s = trimQuotes(s)
	// Unescape Lisp-style backslash escapes (e.g., \\pi -> \pi)
	s = strings.ReplaceAll(s, "\\\\", "\\")
	return s
}

// parseElispProps parses elisp property list format
// Example: (("CATEGORY" . "foo") ("ID" . "bar"))
func parseElispProps(s string) map[string]string {
	props := make(map[string]string)
	
	// Simple regex to extract key-value pairs
	// Matches ("KEY" . "VALUE") or ("KEY" . VALUE)
	re := regexp.MustCompile(`\("([^"]+)"\s*\.\s*"?([^")]*)"?\)`)
	matches := re.FindAllStringSubmatch(s, -1)
	
	for _, m := range matches {
		if len(m) >= 3 {
			props[m[1]] = m[2]
		}
	}
	
	return props
}
