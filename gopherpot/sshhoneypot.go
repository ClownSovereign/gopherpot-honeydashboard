package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHHoneypot, gerçek bir SSH sunucusu gibi banner ve key-exchange
// protokolünü tamamlar (bu yüzden gerçek ssh istemcileri bağlanabilir),
// ancak hiçbir zaman kimlik doğrulamayı KABUL ETMEZ. Amaç tek bir şey:
// saldırganın denediği kullanıcı_adı/şifre çiftini güvenli biçimde yakalamak.
type SSHHoneypot struct {
	cfg         Config
	reporter    *Reporter
	rateLimiter *RateLimiter
	sshConfig   *ssh.ServerConfig
}

func NewSSHHoneypot(cfg Config, reporter *Reporter, rl *RateLimiter) (*SSHHoneypot, error) {
	signer, err := loadOrCreateHostKey(cfg.HostKeyPath)
	if err != nil {
		return nil, fmt.Errorf("host key yüklenemedi: %w", err)
	}

	hp := &SSHHoneypot{cfg: cfg, reporter: reporter, rateLimiter: rl}

	sshConfig := &ssh.ServerConfig{
		// Gerçekçi görünmesi için yaygın bir OpenSSH banner'ı taklit ediyoruz.
		ServerVersion: "SSH-2.0-OpenSSH_8.2p1 Ubuntu-4ubuntu0.5",

		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			hp.logAttempt(conn, conn.User(), string(password))
			// Honeypot olduğumuz için HİÇBİR ZAMAN gerçek erişim vermiyoruz.
			return nil, fmt.Errorf("kimlik doğrulama başarısız")
		},

		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			fp := ssh.FingerprintSHA256(key)
			hp.logAttempt(conn, conn.User(), "pubkey:"+fp)
			return nil, fmt.Errorf("kimlik doğrulama başarısız")
		},

		AuthLogCallback: func(conn ssh.ConnMetadata, method string, err error) {
			// İsteğe bağlı: her denemeyi (yöntem fark etmeksizin) ham olarak da görmek istersen
			// burada ek bir debug log basabilirsin. Şimdilik sessiz geçiyoruz; ayrıntılı
			// kayıt yukarıdaki callback'lerde zaten tutuluyor.
		},
	}
	sshConfig.AddHostKey(signer)
	hp.sshConfig = sshConfig

	return hp, nil
}

func (hp *SSHHoneypot) logAttempt(conn ssh.ConnMetadata, username, payload string) {
	ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
	log.Printf("[ssh] %s kullanıcı=%q payload=%q", ip, username, payload)

	hp.reporter.Submit(LogEntry{
		AttackerIP:  ip,
		ServiceType: "SSH",
		Username:    username,
		Payload:     payload,
		NodeName:    hp.cfg.NodeName,
		Timestamp:   time.Now().UTC(),
	})
}

// ListenAndServe, SSH portunu dinlemeye başlar ve her bağlantıyı
// kendi goroutine'inde ele alır.
func (hp *SSHHoneypot) ListenAndServe() error {
	listener, err := net.Listen("tcp", ":"+hp.cfg.SSHPort)
	if err != nil {
		return err
	}
	log.Printf("[ssh] honeypot dinlemede: :%s", hp.cfg.SSHPort)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[ssh] accept hatası: %v", err)
			continue
		}

		ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
		if !hp.rateLimiter.Allow(ip) {
			_ = conn.Close()
			continue
		}

		go hp.handleConn(conn)
	}
}

func (hp *SSHHoneypot) handleConn(conn net.Conn) {
	defer conn.Close()

	// SSH handshake'i tamamlanana kadar bekleme süresini sınırlıyoruz,
	// aksi halde yarım açık bağlantılar kaynak tüketebilir.
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	sshConn, channels, requests, err := ssh.NewServerConn(conn, hp.sshConfig)
	if err != nil {
		// Çoğu zaman bu satıra normal şekilde düşülür: kimlik doğrulama
		// reddedildiği için handshake burada "hata" ile sonlanır.
		return
	}
	defer sshConn.Close()

	go ssh.DiscardRequests(requests)
	for newChannel := range channels {
		_ = newChannel.Reject(ssh.Prohibited, "erişim reddedildi")
	}
}

func loadOrCreateHostKey(path string) (ssh.Signer, error) {
	if data, err := os.ReadFile(path); err == nil {
		return ssh.ParsePrivateKey(data)
	}

	log.Printf("[ssh] host key bulunamadı, yeni RSA anahtarı üretiliyor: %s", path)
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	}
	pemBytes := pem.EncodeToMemory(pemBlock)

	if err := os.WriteFile(path, pemBytes, 0600); err != nil {
		log.Printf("[ssh] uyarı: host key diske yazılamadı: %v", err)
	}

	return ssh.ParsePrivateKey(pemBytes)
}
