package render

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nicehiro/org-roam-web/internal/config"
	"github.com/nicehiro/org-roam-web/internal/db"
	"github.com/nicehiro/org-roam-web/internal/graph"
	"github.com/nicehiro/org-roam-web/internal/parser"
	"github.com/nicehiro/org-roam-web/internal/search"
)

//go:embed templates/*
var templatesFS embed.FS

// NoteData holds data for rendering a note page
type NoteData struct {
	Site       SiteData
	ID         string
	Title      string
	Tags       []string
	Content    template.HTML
	Links      []LinkData
	Backlinks  []LinkData
	LocalGraph template.JS
	HasGraph   bool
	ToC        []parser.ToCEntry
	ModTime    time.Time
}

// LinkData represents a link to another note
type LinkData struct {
	ID    string
	Title string
}

// HomeData holds data for rendering the home page
type HomeData struct {
	Site        SiteData
	RecentNotes []NotePreview
}

// GraphPageData holds data for the graph page
type GraphPageData struct {
	Site      SiteData
	GraphJSON template.JS
	AllTags   []string
	TopTags   []string
}

// TagPageData holds data for a tag page
type TagPageData struct {
	Site  SiteData
	Tag   string
	Notes []NotePreview
}

// NotePreview is a short preview of a note
type NotePreview struct {
	ID      string
	Title   string
	Tags    []string
	ModTime time.Time
}

// SiteData holds global site information
type SiteData struct {
	Title   string
	BaseURL string
}

// Renderer handles site generation
type Renderer struct {
	cfg       *config.Config
	nodes     []db.Node
	links     []db.Link
	nodeTags  map[string][]string
	nodeMap   map[string]string   // ID -> Title
	backlinks map[string][]string // ID -> []SourceID
}

// NewRenderer creates a new site renderer
func NewRenderer(cfg *config.Config) (*Renderer, error) {
	return &Renderer{
		cfg:       cfg,
		nodeMap:   make(map[string]string),
		backlinks: make(map[string][]string),
	}, nil
}

// templateFuncs returns the template function map
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"join": strings.Join,
		"formatDate": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("Jan 2, 2006")
		},
		// safeHTML marks a string as safe HTML (won't be escaped)
		// Used for titles containing LaTeX like $\pi_0$
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
	}
}

// parseTemplate parses a specific template with the base template
func parseTemplate(name string) (*template.Template, error) {
	return template.New("").Funcs(templateFuncs()).ParseFS(templatesFS, "templates/base.html", "templates/"+name)
}

// Build generates the static site
func (r *Renderer) Build() error {
	// Load data from database
	if err := r.loadData(); err != nil {
		return err
	}

	// Create output directory
	if err := os.MkdirAll(r.cfg.Paths.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate pages
	if err := r.generateHome(); err != nil {
		return err
	}

	if err := r.generateNotes(); err != nil {
		return err
	}

	if err := r.generateGraph(); err != nil {
		return err
	}

	if err := r.generateTags(); err != nil {
		return err
	}

	// Copy images
	if err := r.copyImages(); err != nil {
		return err
	}

	// Generate search index
	if err := r.generateSearchIndex(); err != nil {
		return err
	}

	// Generate graph JSON
	if err := r.generateGraphJSON(); err != nil {
		return err
	}

	return nil
}

// loadData loads all data from the database
func (r *Renderer) loadData() error {
	database, err := db.Open(r.cfg.Paths.DBPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// Load nodes
	nodes, err := database.LoadNodes()
	if err != nil {
		return fmt.Errorf("failed to load nodes: %w", err)
	}

	// Load tags
	nodeTags, err := database.LoadTags()
	if err != nil {
		return fmt.Errorf("failed to load tags: %w", err)
	}

	// Load links
	links, err := database.LoadLinks()
	if err != nil {
		return fmt.Errorf("failed to load links: %w", err)
	}

	// Filter excluded nodes
	r.nodes = r.filterNodes(nodes, nodeTags)
	r.nodeTags = nodeTags
	r.links = links

	// Build node map
	for _, n := range r.nodes {
		r.nodeMap[n.ID] = n.Title
	}

	// Build backlinks map
	for _, l := range r.links {
		r.backlinks[l.Target] = append(r.backlinks[l.Target], l.Source)
	}

	return nil
}

// filterNodes removes excluded nodes
func (r *Renderer) filterNodes(nodes []db.Node, nodeTags map[string][]string) []db.Node {
	excludeTags := make(map[string]bool)
	for _, t := range r.cfg.Exclude.Tags {
		excludeTags[t] = true
	}

	excludeIDs := make(map[string]bool)
	for _, id := range r.cfg.Exclude.IDs {
		excludeIDs[id] = true
	}

	var filtered []db.Node
	for _, n := range nodes {
		// Check excluded IDs
		if excludeIDs[n.ID] {
			continue
		}

		// Check excluded tags
		excluded := false
		for _, tag := range nodeTags[n.ID] {
			if excludeTags[tag] {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		// Check excluded file patterns
		for _, pattern := range r.cfg.Exclude.Files {
			if matched, _ := filepath.Match(pattern, filepath.Base(n.File)); matched {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		filtered = append(filtered, n)
	}

	return filtered
}

// extractDateFromFilename extracts date from org-roam filename
// Formats supported:
// - 20201031101403-title.org (org-roam format)
// - authorTitle2025.org (year at end)
func extractDateFromFilename(filename string) time.Time {
	base := filepath.Base(filename)

	// Try org-roam format: 20201031101403-xxx.org (14 digits)
	if len(base) >= 14 {
		dateStr := base[:14]
		if t, err := time.Parse("20060102150405", dateStr); err == nil {
			return t
		}
	}

	// Try year at end: authorTitle2025.org
	re := regexp.MustCompile(`(\d{4})\.org$`)
	if matches := re.FindStringSubmatch(base); len(matches) > 1 {
		year, _ := strconv.Atoi(matches[1])
		return time.Date(year, 6, 1, 0, 0, 0, 0, time.UTC) // Mid-year as approximation
	}

	// Fallback: file modification time
	if info, err := os.Stat(filename); err == nil {
		return info.ModTime()
	}

	return time.Time{}
}

// generateHome generates the home page
func (r *Renderer) generateHome() error {
	// Sort nodes by date extracted from filename (descending - newest first)
	sorted := make([]db.Node, len(r.nodes))
	copy(sorted, r.nodes)
	sort.Slice(sorted, func(i, j int) bool {
		dateI := extractDateFromFilename(sorted[i].File)
		dateJ := extractDateFromFilename(sorted[j].File)
		return dateI.After(dateJ)
	})

	// Take recent notes
	count := r.cfg.Display.RecentCount
	if count > len(sorted) {
		count = len(sorted)
	}

	recentNotes := make([]NotePreview, count)
	for i := 0; i < count; i++ {
		n := sorted[i]
		recentNotes[i] = NotePreview{
			ID:      n.ID,
			Title:   n.Title,
			Tags:    r.nodeTags[n.ID],
			ModTime: extractDateFromFilename(n.File),
		}
	}

	data := HomeData{
		Site: SiteData{
			Title:   r.cfg.Site.Title,
			BaseURL: r.cfg.Site.BaseURL,
		},
		RecentNotes: recentNotes,
	}

	return r.renderPage("home.html", filepath.Join(r.cfg.Paths.OutputDir, "index.html"), data)
}

// generateNotes generates all note pages
func (r *Renderer) generateNotes() error {
	notesDir := filepath.Join(r.cfg.Paths.OutputDir, "notes")
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		return fmt.Errorf("failed to create notes directory: %w", err)
	}

	p := parser.NewParser(r.cfg.Paths.RoamDir, r.nodeMap)

	for _, n := range r.nodes {
		if err := r.generateNote(p, n, notesDir); err != nil {
			fmt.Printf("Warning: failed to generate note %s: %v\n", n.Title, err)
		}
	}

	return nil
}

// generateNote generates a single note page
func (r *Renderer) generateNote(p *parser.Parser, n db.Node, notesDir string) error {
	// Parse org file
	parsed, err := p.ParseFile(n.File)
	if err != nil {
		return err
	}

	// Build links data
	var links []LinkData
	for _, l := range parsed.Links {
		if title, ok := r.nodeMap[l.ID]; ok {
			links = append(links, LinkData{ID: l.ID, Title: title})
		}
	}

	// Build backlinks data
	var backlinks []LinkData
	for _, sourceID := range r.backlinks[n.ID] {
		if title, ok := r.nodeMap[sourceID]; ok {
			backlinks = append(backlinks, LinkData{ID: sourceID, Title: title})
		}
	}

	// Generate local graph JSON
	localG := graph.LocalGraph(n.ID, r.cfg.Display.LocalGraphDepth, r.nodes, r.links, r.nodeTags)
	localJSON, err := localG.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize local graph: %w", err)
	}

	data := NoteData{
		Site: SiteData{
			Title:   r.cfg.Site.Title,
			BaseURL: r.cfg.Site.BaseURL,
		},
		ID:         n.ID,
		Title:      parsed.Title,
		Tags:       r.nodeTags[n.ID],
		Content:    template.HTML(parsed.Content),
		Links:      links,
		Backlinks:  backlinks,
		LocalGraph: template.JS(localJSON),
		HasGraph:   len(localG.Nodes) > 1,
		ToC:        parsed.ToC,
		ModTime:    extractDateFromFilename(n.File),
	}

	outPath := filepath.Join(notesDir, n.ID+".html")
	return r.renderPage("note.html", outPath, data)
}

// generateGraph generates the graph page
func (r *Renderer) generateGraph() error {
	g := graph.BuildGraph(r.nodes, r.links, r.nodeTags)
	graphJSON, err := g.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize graph: %w", err)
	}

	// Count tags by frequency
	tagCounts := make(map[string]int)
	for _, tags := range r.nodeTags {
		for _, t := range tags {
			tagCounts[t]++
		}
	}

	// Sort tags by count (descending) for top tags
	type tagCount struct {
		Tag   string
		Count int
	}
	var tagList []tagCount
	for t, c := range tagCounts {
		tagList = append(tagList, tagCount{t, c})
	}
	sort.Slice(tagList, func(i, j int) bool {
		return tagList[i].Count > tagList[j].Count
	})

	// Get top 10 tags
	topTags := make([]string, 0, 10)
	for i := 0; i < len(tagList) && i < 10; i++ {
		topTags = append(topTags, tagList[i].Tag)
	}

	// Get all tags (sorted alphabetically)
	var allTags []string
	for t := range tagCounts {
		allTags = append(allTags, t)
	}
	sort.Strings(allTags)

	data := GraphPageData{
		Site: SiteData{
			Title:   r.cfg.Site.Title,
			BaseURL: r.cfg.Site.BaseURL,
		},
		GraphJSON: template.JS(graphJSON),
		AllTags:   allTags,
		TopTags:   topTags,
	}

	return r.renderPage("graph.html", filepath.Join(r.cfg.Paths.OutputDir, "graph.html"), data)
}

// generateTags generates tag listing pages
func (r *Renderer) generateTags() error {
	tagsDir := filepath.Join(r.cfg.Paths.OutputDir, "tags")
	if err := os.MkdirAll(tagsDir, 0755); err != nil {
		return fmt.Errorf("failed to create tags directory: %w", err)
	}

	// Group notes by tag
	tagNotes := make(map[string][]NotePreview)
	for _, n := range r.nodes {
		preview := NotePreview{
			ID:    n.ID,
			Title: n.Title,
			Tags:  r.nodeTags[n.ID],
		}
		for _, tag := range r.nodeTags[n.ID] {
			tagNotes[tag] = append(tagNotes[tag], preview)
		}
	}

	// Generate a page for each tag
	for tag, notes := range tagNotes {
		data := TagPageData{
			Site: SiteData{
				Title:   r.cfg.Site.Title,
				BaseURL: r.cfg.Site.BaseURL,
			},
			Tag:   tag,
			Notes: notes,
		}

		outPath := filepath.Join(tagsDir, tag+".html")
		if err := r.renderPage("tag.html", outPath, data); err != nil {
			return err
		}
	}

	return nil
}

// copyImages copies images from roam directory to output
func (r *Renderer) copyImages() error {
	srcImgDir := filepath.Join(r.cfg.Paths.RoamDir, "img")
	dstImgDir := filepath.Join(r.cfg.Paths.OutputDir, "img")

	// Check if source image directory exists
	if _, err := os.Stat(srcImgDir); os.IsNotExist(err) {
		return nil // No images to copy
	}

	// Walk and copy all images
	return filepath.WalkDir(srcImgDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(srcImgDir, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dstImgDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		// Copy file
		return copyFile(path, dstPath)
	})
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// generateSearchIndex generates the search index JSON
func (r *Renderer) generateSearchIndex() error {
	index := search.BuildIndex(r.nodes, r.nodeTags)
	data, err := index.ToJSON()
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(r.cfg.Paths.OutputDir, "search.json"), data, 0644)
}

// generateGraphJSON generates the full graph JSON
func (r *Renderer) generateGraphJSON() error {
	g := graph.BuildGraph(r.nodes, r.links, r.nodeTags)
	data, err := g.ToJSON()
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(r.cfg.Paths.OutputDir, "graph.json"), data, 0644)
}

// renderPage renders a template to a file
func (r *Renderer) renderPage(tmplName, outPath string, data interface{}) error {
	// Parse template fresh each time to avoid name collisions
	tmpl, err := parseTemplate(tmplName)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", tmplName, err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", outPath, err)
	}
	defer f.Close()

	if err := tmpl.ExecuteTemplate(f, "base", data); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", tmplName, err)
	}

	return nil
}
