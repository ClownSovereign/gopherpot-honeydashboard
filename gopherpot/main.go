package main

import (
	"log"
	"time"
)

func main() {
	cfg := loadConfig()
	log.Printf("[main] GopherPot başlatılıyor (node=%s, ssh=:%s, http=:%s)",
		cfg.NodeName, cfg.SSHPort, cfg.HTTPPort)

	reporter := NewReporter(cfg.BackendURL)

	// SSH ve HTTP ajanları için ayrı rate limiter'lar:
	// böylece HTTP tarafındaki yoğun bot trafiği SSH tarafını etkilemez.
	sshLimiter := NewRateLimiter(cfg.MaxAttemptsIP, time.Minute)
	httpLimiter := NewRateLimiter(cfg.MaxAttemptsIP*4, time.Minute) // HTTP tarama trafiği doğal olarak daha yoğun olur
	go sshLimiter.Cleanup(5 * time.Minute)
	go httpLimiter.Cleanup(5 * time.Minute)

	sshHoneypot, err := NewSSHHoneypot(cfg, reporter, sshLimiter)
	if err != nil {
		log.Fatalf("[main] SSH honeypot başlatılamadı: %v", err)
	}

	httpHoneypot := NewHTTPHoneypot(cfg, reporter, httpLimiter)

	errCh := make(chan error, 2)

	go func() {
		errCh <- sshHoneypot.ListenAndServe()
	}()
	go func() {
		errCh <- httpHoneypot.ListenAndServe()
	}()

	// İki servisten biri ölümcül şekilde çökerse logla ve çık;
	// bir orkestratör (systemd/docker restart policy) yeniden başlatsın.
	err = <-errCh
	log.Fatalf("[main] bir honeypot servisi durdu: %v", err)
}
