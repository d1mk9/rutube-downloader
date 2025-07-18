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
	http.HandleFunc("/terms.html", handler.TermsHandler)
	http.HandleFunc("/privacy.html", handler.PrivacyHandler)
	http.HandleFunc("/about.html", handler.AboutHandler)
	http.HandleFunc("/how-to-download-rutube", handler.HowToDownloadHandler)
	http.HandleFunc("/rutube-to-mp4", handler.RutubeToMP4Handler)
	http.HandleFunc("/rutube-android-download", handler.RutubeAndroidHandler)
	http.HandleFunc("/download-rutube-2025", handler.Download2025Handler)
	http.HandleFunc("/rutube-playlist-download", handler.RutubePlaylistHandler)
	http.HandleFunc("/rutube-without-watermark", handler.RutubeNoWatermarkHandler)
	http.HandleFunc("/save-rutube-on-iphone", handler.RutubeIphoneHandler)
	http.HandleFunc("/rutube-windows-download", handler.RutubeWindowsHandler)
	http.HandleFunc("/rutube-smarttv-save", handler.RutubeSmartTVHandler)
	http.HandleFunc("/rutube-shorts-download", handler.RutubeShortsHandler)
	http.HandleFunc("/rutube-private-download", handler.RutubePrivateHandler)
	http.HandleFunc("/rutube-embed-download", handler.RutubeEmbedHandler)
	http.HandleFunc("/top-rutube-videos", handler.TopRutubeHandler)
	http.HandleFunc("/rutube-ads-remove", handler.RutubeAdsRemoveHandler)

	// — Статика —
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/downloads/", http.StripPrefix("/downloads/", http.FileServer(http.Dir("downloads"))))
	http.Handle("/sitemap.xml", http.FileServer(http.Dir("static")))
	http.Handle("/robots.txt", http.FileServer(http.Dir("static")))

	// — Слушаем только localhost:8080 —
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

// Определение корня проекта
func getProjectRoot() string {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("❌ os.Executable failed: %v", err)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(exePath), "..", ".."))
}
