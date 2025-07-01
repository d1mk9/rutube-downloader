package handler

import (
	"html/template"
	"log"
	"net/http"
	"strings"

	"rutube-downloader/internal/parser"
)

type ResultPageData struct {
	Error       string
	OriginalURL string
	VideoLink   string
}

func DownloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	log.Printf("📩 METHOD: %s", r.Method)
	url := strings.TrimSpace(r.FormValue("url"))
	log.Printf("🌐 Полученная ссылка: '%s'", url)

	if !strings.Contains(url, "rutube.ru") {
		renderError(w, "Введите корректную ссылку на RuTube")
		return
	}

	videoLink, err := parser.ExtractMP4(url)
	if err != nil || videoLink == "" {
		log.Printf("❌ Ошибка при парсинге RuTube: %v", err)
		renderError(w, "Не удалось извлечь видео. Попробуйте позже.")
		return
	}
	log.Println("✅ Ссылка на видео:", videoLink)

	tmpl, err := template.ParseFiles("internal/templates/result.html")
	if err != nil {
		http.Error(w, "Ошибка шаблона", http.StatusInternalServerError)
		return
	}

	data := ResultPageData{
		Error:       "",
		OriginalURL: url,
		VideoLink:   videoLink,
	}

	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("❌ Ошибка при отрисовке шаблона: %v", err)
	}
}

func renderError(w http.ResponseWriter, message string) {
	tmpl, err := template.ParseFiles("internal/templates/result.html")
	if err != nil {
		http.Error(w, "Ошибка шаблона", http.StatusInternalServerError)
		return
	}

	data := ResultPageData{
		Error:       message,
		OriginalURL: "",
		VideoLink:   "",
	}

	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("❌ Ошибка при отрисовке ошибки: %v", err)
	}
}
