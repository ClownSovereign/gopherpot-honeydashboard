package main

import (
	"sync"
	"time"
)

// RateLimiter, aynı IP'den dakikada belirli bir sayıdan fazla bağlantı
// denemesini engeller. Honeypot olsa da sınırsız flood'a açık olmamalı.
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

func NewRateLimiter(limitPerWindow int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limitPerWindow,
		window:   window,
	}
}

// Allow, verilen IP için bu istek kabul edilebilir mi diye bakar.
// Eşik aşıldıysa false döner (bağlantı reddedilmeli / loglanmamalı).
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	timestamps := rl.attempts[ip]
	pruned := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			pruned = append(pruned, t)
		}
	}

	if len(pruned) >= rl.limit {
		rl.attempts[ip] = pruned
		return false
	}

	pruned = append(pruned, now)
	rl.attempts[ip] = pruned
	return true
}

// Cleanup, periyodik olarak eski/boş girdileri haritadan temizler
// (uzun süre çalışan ajanlarda bellek şişmesini önlemek için).
func (rl *RateLimiter) Cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-rl.window)
		for ip, timestamps := range rl.attempts {
			pruned := timestamps[:0]
			for _, t := range timestamps {
				if t.After(cutoff) {
					pruned = append(pruned, t)
				}
			}
			if len(pruned) == 0 {
				delete(rl.attempts, ip)
			} else {
				rl.attempts[ip] = pruned
			}
		}
		rl.mu.Unlock()
	}
}
