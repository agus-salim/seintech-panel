package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
)

//go:embed templates/* static/*
var EmbeddedFiles embed.FS

// RenderTemplate renders the layout and a specific content template from embedded FS.
func RenderTemplate(w http.ResponseWriter, tmplName string, data any) error {
	tmpl, err := template.ParseFS(EmbeddedFiles, "templates/layout.html", "templates/"+tmplName)
	if err != nil {
		return fmt.Errorf("error parsing template %s: %w", tmplName, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.Execute(w, data)
}

// RenderSingleTemplate renders a standalone template without the standard layout (e.g. login.html).
func RenderSingleTemplate(w http.ResponseWriter, tmplName string, data any) error {
	tmpl, err := template.ParseFS(EmbeddedFiles, "templates/"+tmplName)
	if err != nil {
		return fmt.Errorf("error parsing standalone template %s: %w", tmplName, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.Execute(w, data)
}

// GetStaticFS returns an http.Handler that serves embedded static assets.
func GetStaticFS() (http.Handler, error) {
	sub, err := fs.Sub(EmbeddedFiles, "static")
	if err != nil {
		return nil, err
	}
	return http.FileServer(http.FS(sub)), nil
}
