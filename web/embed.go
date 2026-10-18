package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"github.com/seintech/panel/internal/core/i18n"
)

//go:embed templates/* static/*
var EmbeddedFiles embed.FS

func getTemplateFuncs(data any) template.FuncMap {
	currentLang := "en"
	if lg, ok := data.(interface{ GetLang() string }); ok {
		currentLang = lg.GetLang()
	}

	return template.FuncMap{
		"percent": func(used, max int64) int {
			if max <= 0 {
				return 0
			}
			p := (used * 100) / max
			if p > 100 {
				p = 100
			}
			return int(p)
		},
		"formatMB": func(mb int64) string {
			if mb <= 0 {
				return "Unlimited"
			}
			if mb >= 1024 && mb%1024 == 0 {
				return fmt.Sprintf("%d GB", mb/1024)
			}
			return fmt.Sprintf("%d MB", mb)
		},
		"T": func(phrase string, args ...any) string {
			return i18n.T(currentLang, phrase, args...)
		},
	}
}

// RenderTemplate renders the layout and a specific content template from embedded FS.
func RenderTemplate(w http.ResponseWriter, tmplName string, data any) error {
	funcs := getTemplateFuncs(data)
	tmpl, err := template.New("layout.html").Funcs(funcs).ParseFS(EmbeddedFiles, "templates/layout.html", "templates/"+tmplName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Template Parse Error (%s): %v", tmplName, err), http.StatusInternalServerError)
		return fmt.Errorf("error parsing template %s: %w", tmplName, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, fmt.Sprintf("Template Execute Error (%s): %v", tmplName, err), http.StatusInternalServerError)
		return err
	}
	return nil
}

// RenderSingleTemplate renders a standalone template without the standard layout (e.g. login.html).
func RenderSingleTemplate(w http.ResponseWriter, tmplName string, data any) error {
	funcs := getTemplateFuncs(data)
	tmpl, err := template.New(tmplName).Funcs(funcs).ParseFS(EmbeddedFiles, "templates/"+tmplName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Template Parse Error (%s): %v", tmplName, err), http.StatusInternalServerError)
		return fmt.Errorf("error parsing standalone template %s: %w", tmplName, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, fmt.Sprintf("Template Execute Error (%s): %v", tmplName, err), http.StatusInternalServerError)
		return err
	}
	return nil
}

// GetStaticFS returns an http.Handler that serves embedded static assets.
func GetStaticFS() (http.Handler, error) {
	sub, err := fs.Sub(EmbeddedFiles, "static")
	if err != nil {
		return nil, err
	}
	return http.FileServer(http.FS(sub)), nil
}
