package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"rutube-downloader/internal/handler"
)

func main() {
	// — Авто-смена директории, если нужно —
	if !templateExists("internal/templates/index.html") {
		projectRoot := getProjectRoot()
		if err := os.Chdir(projectRoot); err != nil {
			log.Fatalf("❌ Не удалось выполнить os.Chdir: %v", err)
		}
		log.Println("📁 Авто-переход в директорию:", projectRoot)
	}

	// — Роутинг —
	http.HandleFunc("/", handler.IndexHandler)
	http.HandleFunc("/download", handler.DownloadHandler)

	// — Статика —
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/downloads/", http.StripPrefix("/downloads/", http.FileServer(http.Dir("downloads"))))

	// — Слушаем ТОЛЬКО localhost:8080 —
	addr := "127.0.0.1:8080"
	log.Println("🚀 Backend running on", addr, "(за nginx-прокси)")
	if err := http.ListenAndServe(addr, nil); err != nil {
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
