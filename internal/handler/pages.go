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

func HowToDownloadHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "how-to-download-rutube.html")
}

func RutubeToMP4Handler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "rutube-to-mp4.html")
}

func RutubeAndroidHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "rutube-android-download.html")
}

func Download2025Handler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "download-rutube-2025.html")
}

func RutubePlaylistHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "rutube-playlist-download.html")
}

func RutubeNoWatermarkHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "rutube-without-watermark.html")
}

func RutubeIphoneHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "save-rutube-on-iphone.html")
}

func RutubeWindowsHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "rutube-windows-download.html")
}

func RutubeSmartTVHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "rutube-smarttv-save.html")
}

func RutubeShortsHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "rutube-shorts-download.html")
}

func RutubePrivateHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "rutube-private-download.html")
}

func RutubeEmbedHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "rutube-embed-download.html")
}

func TopRutubeHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "top-rutube-videos.html")
}

func RutubeAdsRemoveHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "rutube-ads-remove.html")
}

func renderTemplate(w http.ResponseWriter, name string) {
	tmpl := template.Must(template.ParseFiles("internal/templates/" + name))
	tmpl.Execute(w, nil)
}
