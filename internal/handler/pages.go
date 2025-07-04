package handler

import (
	"html/template"
	"net/http"
)

func TermsHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "terms.html")
}

func PrivacyHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "privacy.html")
}

func AboutHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "about.html")
}

func renderTemplate(w http.ResponseWriter, name string) {
	tmpl := template.Must(template.ParseFiles("internal/templates/" + name))
	tmpl.Execute(w, nil)
}
