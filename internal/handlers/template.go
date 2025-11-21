package handlers

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// TemplateManager is responsible for loading and caching HTML templates
// from the webappTemplates/ directory and providing helper methods for
// rendering them in handlers.
type TemplateManager struct {
	mu        sync.RWMutex
	templates *template.Template
	baseDir   string
}

// NewTemplateManager creates a new TemplateManager. If baseDir is empty,
// it defaults to "./webappTemplates".
func NewTemplateManager(baseDir string) *TemplateManager {
	if baseDir == "" {
		baseDir = "./webappTemplates"
	}
	return &TemplateManager{
		baseDir: baseDir,
	}
}

// Load parses all *.html templates under the baseDir and caches them.
// It is safe to call multiple times; each call replaces the internal
// template cache.
func (m *TemplateManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	root := m.baseDir

	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			log.Printf("TemplateManager: base directory %q does not exist", root)
			// Keep templates nil; rendering will return an error.
			return nil
		}
		return err
	}

	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".html") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if len(files) == 0 {
		log.Printf("TemplateManager: no *.html templates found under %q", root)
		m.templates = nil
		return nil
	}

	tmpl, err := template.ParseFiles(files...)
	if err != nil {
		return err
	}

	m.templates = tmpl
	log.Printf("TemplateManager: loaded %d templates from %s", len(files), root)
	return nil
}

// Render renders the named template with the given data and HTTP status.
// It handles missing templates and internal errors gracefully by logging
// and returning an appropriate HTTP error response.
func (m *TemplateManager) Render(c *gin.Context, name string, status int, data gin.H) {
	m.mu.RLock()
	tmpl := m.templates
	m.mu.RUnlock()

	if tmpl == nil {
		log.Printf("TemplateManager: no templates loaded (attempted to render %q)", name)
		c.String(http.StatusInternalServerError, "templates not loaded")
		return
	}

	if tmpl.Lookup(name) == nil {
		log.Printf("TemplateManager: template %q not found", name)
		c.String(http.StatusNotFound, "template %s not found", name)
		return
	}

	// Gin's HTML rendering expects a template name that is already
	// registered; we execute it directly to the underlying ResponseWriter.
	c.Status(status)
	if err := tmpl.ExecuteTemplate(c.Writer, name, data); err != nil {
		log.Printf("TemplateManager: error executing template %q: %v", name, err)
		c.Status(http.StatusInternalServerError)
		// Avoid writing another body if some output was already sent.
		if !c.Writer.Written() {
			c.String(http.StatusInternalServerError, "error rendering template")
		}
	}
}



