package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"rutube-downloader/internal/handler"
)

func main() {
	// Определяем корень проекта (чтобы запускать из любой директории)
	projectRoot := getProjectRoot()
	if err := os.Chdir(projectRoot); err != nil {
		log.Fatalf("❌ Не удалось установить рабочую директорию: %v", err)
	}

	// Роутинг
	http.HandleFunc("/", handler.IndexHandler)
	http.HandleFunc("/download", handler.DownloadHandler)

	// Статические файлы (CSS, favicon и прочее)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Скачанные видео
	http.Handle("/downloads/", http.StripPrefix("/downloads/", http.FileServer(http.Dir("downloads"))))

	// Запуск сервера
	log.Println("🚀 Server running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("❌ Ошибка запуска сервера: %v", err)
	}
}

// getProjectRoot возвращает путь к корню проекта (из /cmd/server → ../../)
func getProjectRoot() string {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("❌ Не удалось определить путь к бинарнику: %v", err)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(exePath), "..", ".."))
}
