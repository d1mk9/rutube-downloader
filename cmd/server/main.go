package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"rutube-downloader/internal/handler"
)

func main() {
	// Проверка: можно ли найти шаблон из текущей директории
	if !templateExists("internal/templates/index.html") {
		projectRoot := getProjectRoot()
		if err := os.Chdir(projectRoot); err != nil {
			log.Fatalf("❌ Не удалось выполнить os.Chdir: %v", err)
		}
		log.Println("📁 Авто-переход в директорию:", projectRoot)
	}

	// Роутинг
	http.HandleFunc("/", handler.IndexHandler)
	http.HandleFunc("/download", handler.DownloadHandler)

	// Статические файлы
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/downloads/", http.StripPrefix("/downloads/", http.FileServer(http.Dir("downloads"))))

	// Сервер
	log.Println("🚀 Server running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("❌ ListenAndServe error: %v", err)
	}
}

// Проверка наличия шаблона
func templateExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Корень проекта для Chdir (из /cmd/server → ../../)
func getProjectRoot() string {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("❌ os.Executable failed: %v", err)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(exePath), "..", ".."))
}
