package handler

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type JobStatus string

const (
	JobQueued  JobStatus = "queued"
	JobRunning JobStatus = "running"
	JobDone    JobStatus = "done"
	JobError   JobStatus = "error"
)

type Job struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Status    JobStatus `json:"status"`
	Percent   float64   `json:"percent"`   // 0..100
	FileName  string    `json:"file_name"` // когда готов
	ErrorText string    `json:"error,omitempty"`
}

var (
	jobsMu sync.RWMutex
	jobs   = map[string]*Job{}
)

func setJob(id string, upd func(*Job)) {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	if j, ok := jobs[id]; ok {
		upd(j)
	}
}

func getJob(id string) *Job {
	jobsMu.RLock()
	defer jobsMu.RUnlock()
	return jobs[id]
}

func ProgressHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	j := getJob(id)
	if j == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(j)
}
