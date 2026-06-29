package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// LogEntry, HoneyDashboard backend'inin /api/v1/log-submit endpoint'ine
// gönderilen JSON yapısını temsil eder.
type LogEntry struct {
	AttackerIP  string    `json:"attacker_ip"`
	ServiceType string    `json:"service_type"` // "SSH" veya "HTTP"
	Payload     string    `json:"payload"`      // denenen şifre, istenen URL, vb.
	Username    string    `json:"username,omitempty"`
	NodeName    string    `json:"node_name"`
	Timestamp   time.Time `json:"timestamp"`
}

// Reporter, gelen logları arkada birden fazla worker üzerinden
// backend'e gönderir. Backend o an ayakta değilse veya kuyruk doluysa,
// log fallback_logs.jsonl dosyasına güvenli (mutex korumalı) şekilde yazılır
// ki yoğun saldırı/flood anında veri sessizce kaybolmasın.
type Reporter struct {
	backendURL string
	client     *http.Client
	queue      chan LogEntry
	fallback   *os.File
	mu         sync.Mutex // fallback dosyasına eşzamanlı yazımı korur
}

const reporterWorkerCount = 3

func NewReporter(backendURL string) *Reporter {
	f, err := os.OpenFile("fallback_logs.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[reporter] fallback log dosyası açılamadı: %v", err)
	}
	r := &Reporter{
		backendURL: backendURL,
		// Timeout'u 5s'den 1.5s'e çektik: backend yanıt vermiyorsa worker'ı
		// uzun süre bloklamak, kuyruğun hızla dolup log kaybına yol açmasını engeller.
		client:   &http.Client{Timeout: 1500 * time.Millisecond},
		queue:    make(chan LogEntry, 1000),
		fallback: f,
	}

	// Tek worker yerine birden fazla worker: biri network'te beklerken
	// diğerleri kuyruğu tüketmeye devam eder.
	for i := 0; i < reporterWorkerCount; i++ {
		go r.worker()
	}
	return r
}

func (r *Reporter) Submit(entry LogEntry) {
	select {
	case r.queue <- entry:
	default:
		// Kuyruk tamamen doluysa (flood anı), logu sessizce düşürmek yerine
		// doğrudan diske yazıyoruz. Bu işlem çağıran goroutine'i (her bağlantı
		// kendi goroutine'inde işlendiği için) kısa süreliğine bloklar ama
		// diğer bağlantıları etkilemez.
		log.Printf("[reporter] kuyruk dolu, log doğrudan fallback dosyasına yazılıyor: %s", entry.AttackerIP)
		r.writeFallback(entry)
	}
}

func (r *Reporter) worker() {
	for entry := range r.queue {
		if err := r.send(entry); err != nil {
			log.Printf("[reporter] backend'e gönderilemedi (%v), fallback dosyasına yazılıyor", err)
			r.writeFallback(entry)
		}
	}
}

func (r *Reporter) send(entry LogEntry) error {
	body, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, r.backendURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return &httpStatusError{resp.StatusCode}
	}
	return nil
}

func (r *Reporter) writeFallback(entry LogEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.fallback == nil {
		return
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return
	}
	b = append(b, '\n')
	_, _ = r.fallback.Write(b)
}

type httpStatusError struct{ code int }

func (e *httpStatusError) Error() string {
	return "beklenmeyen HTTP durumu: " + http.StatusText(e.code)
}