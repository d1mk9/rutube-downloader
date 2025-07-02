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

	// Запуск HTTP-сервера для редиректа с 80 на 443
	go func() {
		log.Fatal(http.ListenAndServe(":80", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			target := "https://" + r.Host + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		})))
	}()

	// Запуск HTTPS-сервера
	log.Println("🚀 Server running on https://vidpull.ru")
	err := http.ListenAndServeTLS(":443", "/etc/ssl/vidpull.crt", "/etc/ssl/vidpull.key", nil)
	if err != nil {
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
