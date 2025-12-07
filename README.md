# org-roam-web

A static site generator for [org-roam](https://www.orgroam.com/) notes. Generates a clean, elegant website with:

- **Home page** with search and recent notes
- **Note pages** with content, local graph, and backlinks
- **Graph explorer** with tag filtering
- **Tag pages** for browsing by topic

## Features

- OpenCode-inspired dark theme (with light mode support)
- Full-text search (Fuse.js)
- Interactive graph visualization (D3.js)
- LaTeX math rendering (KaTeX)
- Responsive layout
- Dev server with live reload
- GitHub Action for automated deployment

## Installation

### From source

```bash
git clone https://github.com/nicehiro/org-roam-web.git
cd org-roam-web
go build -o org-roam-web .
```

### Using Go

```bash
go install github.com/nicehiro/org-roam-web@latest
```

## Usage

### Build

```bash
# With config file
org-roam-web build --config config.yaml

# With command line flags
org-roam-web build --roam-dir ~/Documents/roam --output ./dist
```

### Development Server

```bash
org-roam-web serve --config config.yaml --port 8080
```

The server watches for changes to `.org` files and automatically rebuilds.

## Configuration

Create a `config.yaml` file:

```yaml
site:
  title: "My Notes"
  base_url: ""  # Set to "/repo-name" for GitHub Pages project sites

paths:
  roam_dir: "~/Documents/roam"
  db_path: "roam.db"
  output_dir: "./dist"

exclude:
  tags:
    - private
    - draft
  files: []
  ids: []

display:
  recent_count: 20
  local_graph_depth: 2
```

## GitHub Action

Use in your org-roam notes repository:

```yaml
# .github/workflows/deploy.yml
name: Deploy Notes

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Build site
        uses: nicehiro/org-roam-web@v1
        with:
          roam_dir: '.'
          db_path: 'roam.db'
          output_dir: 'dist'
          site_title: "My Notes"
          exclude_tags: 'private,draft'
          
      - name: Deploy to GitHub Pages
        uses: peaceiris/actions-gh-pages@v3
        with:
          github_token: ${{ secrets.GITHUB_TOKEN }}
          publish_dir: ./dist
```

### Action Inputs

| Input | Description | Default |
|-------|-------------|---------|
| `roam_dir` | Path to org-roam directory | `.` |
| `db_path` | Path to org-roam database | `roam.db` |
| `output_dir` | Output directory | `dist` |
| `site_title` | Site title | `My Notes` |
| `base_url` | Base URL for links | `` |
| `exclude_tags` | Comma-separated tags to exclude | `private,draft` |

## Requirements

- Go 1.21+ (for building from source)
- org-roam SQLite database (`roam.db`)

## Design

The site uses an OpenCode-inspired design:

- Dark theme by default (respects `prefers-color-scheme`)
- Muted color palette with soft blue-purple accent
- System font stack
- Generous whitespace
- Internal links styled as `# Note Title`

## License

MIT
