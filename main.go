package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/nicehiro/org-roam-web/internal/config"
	"github.com/nicehiro/org-roam-web/internal/render"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "build":
		buildCmd(os.Args[2:])
	case "serve":
		serveCmd(os.Args[2:])
	case "version":
		fmt.Printf("org-roam-web %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`org-roam-web - Static site generator for org-roam notes

Usage:
  org-roam-web <command> [options]

Commands:
  build     Build the static site
  serve     Start development server with live reload
  version   Print version information
  help      Print this help message

Build Options:
  -config string    Path to config file (default "config.yaml")
  -roam-dir string  Path to org-roam directory
  -db-path string   Path to org-roam database
  -output string    Output directory (default "dist")

Serve Options:
  -config string    Path to config file (default "config.yaml")
  -port int         Server port (default 8080)

Examples:
  org-roam-web build --config config.yaml
  org-roam-web serve --port 3000
  org-roam-web build --roam-dir ~/Documents/roam --output ./dist`)
}

func buildCmd(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	configPath := fs.String("config", "config.yaml", "Path to config file")
	roamDir := fs.String("roam-dir", "", "Path to org-roam directory")
	dbPath := fs.String("db-path", "", "Path to org-roam database")
	outputDir := fs.String("output", "", "Output directory")
	fs.Parse(args)

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Override with command line flags
	if *roamDir != "" {
		cfg.Paths.RoamDir = *roamDir
	}
	if *dbPath != "" {
		cfg.Paths.DBPath = *dbPath
	}
	if *outputDir != "" {
		cfg.Paths.OutputDir = *outputDir
	}

	// Make paths absolute
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}
	if !filepath.IsAbs(cfg.Paths.RoamDir) {
		cfg.Paths.RoamDir = filepath.Join(cwd, cfg.Paths.RoamDir)
	}
	if !filepath.IsAbs(cfg.Paths.DBPath) {
		cfg.Paths.DBPath = filepath.Join(cfg.Paths.RoamDir, filepath.Base(cfg.Paths.DBPath))
	}

	fmt.Printf("Building site...\n")
	fmt.Printf("  Roam dir: %s\n", cfg.Paths.RoamDir)
	fmt.Printf("  Database: %s\n", cfg.Paths.DBPath)
	fmt.Printf("  Output:   %s\n", cfg.Paths.OutputDir)

	r, err := render.NewRenderer(cfg)
	if err != nil {
		log.Fatalf("Failed to create renderer: %v", err)
	}

	start := time.Now()
	if err := r.Build(); err != nil {
		log.Fatalf("Failed to build site: %v", err)
	}

	fmt.Printf("Done in %v\n", time.Since(start).Round(time.Millisecond))
}

func serveCmd(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "config.yaml", "Path to config file")
	port := fs.Int("port", 8080, "Server port")
	roamDir := fs.String("roam-dir", "", "Path to org-roam directory")
	fs.Parse(args)

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if *roamDir != "" {
		cfg.Paths.RoamDir = *roamDir
	}

	// Make paths absolute
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}
	if !filepath.IsAbs(cfg.Paths.RoamDir) {
		cfg.Paths.RoamDir = filepath.Join(cwd, cfg.Paths.RoamDir)
	}
	if !filepath.IsAbs(cfg.Paths.DBPath) {
		cfg.Paths.DBPath = filepath.Join(cfg.Paths.RoamDir, filepath.Base(cfg.Paths.DBPath))
	}

	// Initial build
	rebuild(cfg)

	// Set up file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// Watch org files directory
	if err := watcher.Add(cfg.Paths.RoamDir); err != nil {
		log.Printf("Warning: Failed to watch roam directory: %v", err)
	}

	// Watch for changes
	go func() {
		var debounceTimer *time.Timer
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// Only rebuild on write events for .org files
				if event.Has(fsnotify.Write) && filepath.Ext(event.Name) == ".org" {
					// Debounce rebuilds
					if debounceTimer != nil {
						debounceTimer.Stop()
					}
					debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
						fmt.Printf("\nFile changed: %s\n", filepath.Base(event.Name))
						rebuild(cfg)
					})
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Watcher error: %v", err)
			}
		}
	}()

	// Start HTTP server
	addr := fmt.Sprintf(":%d", *port)
	fmt.Printf("\nServing at http://localhost%s\n", addr)
	fmt.Printf("Press Ctrl+C to stop\n\n")

	http.Handle("/", http.FileServer(http.Dir(cfg.Paths.OutputDir)))
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func rebuild(cfg *config.Config) {
	fmt.Printf("Building...")
	start := time.Now()

	r, err := render.NewRenderer(cfg)
	if err != nil {
		log.Printf("Failed to create renderer: %v", err)
		return
	}

	if err := r.Build(); err != nil {
		log.Printf("Failed to build: %v", err)
		return
	}

	fmt.Printf(" done in %v\n", time.Since(start).Round(time.Millisecond))
}
