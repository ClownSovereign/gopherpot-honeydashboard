package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// HTTPHoneypot, botların/zafiyet tarayıcılarının sık ziyaret ettiği
// yolları (wp-login.php, /admin, .env, vb.) taklit eder ve gelen
// her isteği (yol, metod, body, header) loglar.
type HTTPHoneypot struct {
	cfg         Config
	reporter    *Reporter
	rateLimiter *RateLimiter
}

func NewHTTPHoneypot(cfg Config, reporter *Reporter, rl *RateLimiter) *HTTPHoneypot {
	return &HTTPHoneypot{cfg: cfg, reporter: reporter, rateLimiter: rl}
}

// fakeResponses, bilinen yollar için gerçekçi sahte yanıtlar tanımlar.
// Anahtar: yolun bir parçası (substring eşleşmesi), Değer: (content-type, body)
var fakeResponses = map[string]struct {
	contentType string
	body        string
	status      int
}{
	"wp-login.php": {
		"text/html",
		`<!DOCTYPE html><html><head><title>Log In &lsaquo; Site &mdash; WordPress</title></head>
<body class="login"><div id="login"><h1>Site</h1>
<form name="loginform" id="loginform" action="wp-login.php" method="post">
<p><label>Username or Email Address</label><input type="text" name="log"></p>
<p><label>Password</label><input type="password" name="pwd"></p>
<p class="submit"><input type="submit" value="Log In"></p>
</form></div></body></html>`,
		200,
	},
	"/admin": {
		"text/html",
		`<!DOCTYPE html><html><head><title>Admin Login</title></head>
<body><h2>Administration Panel</h2>
<form method="post"><input name="username" placeholder="Username"><br>
<input name="password" type="password" placeholder="Password"><br>
<button type="submit">Sign in</button></form></body></html>`,
		200,
	},
	".env": {
		"text/plain",
		"DB_CONNECTION=mysql\nDB_HOST=127.0.0.1\nDB_DATABASE=app\nDB_USERNAME=root\nDB_PASSWORD=\nAPP_KEY=base64:fake\n",
		200,
	},
	"phpmyadmin": {
		"text/html",
		`<!DOCTYPE html><html><head><title>phpMyAdmin</title></head><body><h1>phpMyAdmin</h1></body></html>`,
		200,
	},
}

func (h *HTTPHoneypot) handler(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !h.rateLimiter.Allow(ip) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	body, _ := io.ReadAll(io.LimitReader(r.Body, 4096)) // body'yi sınırlı oku, flood'a karşı
	payload := r.Method + " " + r.URL.String()
	if len(body) > 0 {
		payload += " body=" + string(body)
	}

	log.Printf("[http] %s %s", ip, payload)

	h.reporter.Submit(LogEntry{
		AttackerIP:  ip,
		ServiceType: "HTTP",
		Payload:     payload,
		NodeName:    h.cfg.NodeName,
		Timestamp:   time.Now().UTC(),
	})

	for pathFragment, resp := range fakeResponses {
		if strings.Contains(strings.ToLower(r.URL.Path), pathFragment) {
			w.Header().Set("Content-Type", resp.contentType)
			w.Header().Set("Server", "Apache/2.4.41 (Ubuntu)") // gerçekçi görünmek için
			w.WriteHeader(resp.status)
			_, _ = w.Write([]byte(resp.body))
			return
		}
	}

	// Bilinmeyen yollar için genel, sıradan görünen bir 404 dön.
	w.Header().Set("Server", "Apache/2.4.41 (Ubuntu)")
	http.NotFound(w, r)
}

func (h *HTTPHoneypot) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.handler)

	server := &http.Server{
		Addr:         ":" + h.cfg.HTTPPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	log.Printf("[http] honeypot dinlemede: :%s", h.cfg.HTTPPort)
	return server.ListenAndServe()
}

func clientIP(r *http.Request) string {
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}
	return r.RemoteAddr
}
