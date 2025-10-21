package handler

import (
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"rutube-downloader/internal/parser"
)

type ResultPageData struct {
	Error       string
	OriginalURL string
	VideoLink   string
	JobID       string
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// на крайний случай — timestamp
		return time.Now().Format("20060102T150405.000000000")
	}
	return hex.EncodeToString(b)
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

	// Создаём задачу и сразу возвращаем страницу с прогресс-баром.
	jobID := newID()
	job := &Job{
		ID:        jobID,
		CreatedAt: time.Now(),
		Status:    JobQueued,
		Percent:   0,
	}
	jobsMu.Lock()
	jobs[jobID] = job
	jobsMu.Unlock()

	// Фоновая горутина: парсинг + ffmpeg
	go func(jobID, videoURL string) {
		setJob(jobID, func(j *Job) {
			j.Status = JobRunning
			j.Percent = 0
		})

		// ExtractMP4WithProgress отдаёт имя файла и обновляет проценты через callback
		fileName, err := parser.ExtractMP4WithProgress(videoURL, func(done, total float64) {
			// total может быть 0 в начале — защищаемся
			if total > 0 {
				p := (done / total) * 100
				if p > 100 {
					p = 100
				}
				setJob(jobID, func(j *Job) { j.Percent = p })
			}
		})

		if err != nil || fileName == "" {
			log.Printf("❌ Ошибка при парсинге RuTube: %v", err)
			setJob(jobID, func(j *Job) {
				j.Status = JobError
				j.ErrorText = "Не удалось извлечь видео. Попробуйте позже."
			})
			return
		}

		setJob(jobID, func(j *Job) {
			j.Status = JobDone
			j.Percent = 100
			j.FileName = fileName
		})
	}(jobID, url)

	// Рендерим страницу с прогресс-баром и авто-подстановкой ссылки по готовности
	tmpl, err := template.ParseFiles("internal/templates/result.html")
	if err != nil {
		http.Error(w, "Ошибка шаблона", http.StatusInternalServerError)
		return
	}
	data := ResultPageData{
		Error:       "",
		OriginalURL: url,
		VideoLink:   "", // появится, когда задача завершится
		JobID:       jobID,
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
		JobID:       "",
	}
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("❌ Ошибка при отрисовке ошибки: %v", err)
	}
}
