"""
geoip.py
IP -> ülke/şehir çözümlemesi.

Varsayılan olarak ip-api.com'un ücretsiz endpoint'ini kullanıyoruz (key gerektirmez,
~45 istek/dakika sınırı var). Daha ciddi/üretim kullanımı için MaxMind GeoLite2
veritabanını indirip yerel (offline) sorgu yapmak daha sağlıklı olur -- bunu
GEOIP_BACKEND=maxmind ortam değişkeniyle aktif edebilirsin (geolite2 .mmdb dosyası
ve `geoip2` paketi gerektirir, README'de anlatılıyor).
"""
import os
import time
import requests

_CACHE: dict[str, tuple[str, str]] = {}
_CACHE_TTL_SECONDS = 60 * 60 * 6  # 6 saat
_cache_timestamps: dict[str, float] = {}

GEOIP_BACKEND = os.getenv("GEOIP_BACKEND", "ip-api")

# Özel/yerel IP aralıkları için boşuna dış servise sormayalım.
_PRIVATE_PREFIXES = ("10.", "172.16.", "192.168.", "127.", "::1")


def resolve_location(ip: str) -> tuple[str, str]:
    """IP için (ülke, şehir) döner. Bulunamazsa ("Unknown", "Unknown")."""
    if ip.startswith(_PRIVATE_PREFIXES):
        return ("Local/Private", "-")

    now = time.time()
    cached = _CACHE.get(ip)
    if cached and now - _cache_timestamps.get(ip, 0) < _CACHE_TTL_SECONDS:
        return cached

    if GEOIP_BACKEND == "maxmind":
        result = _resolve_with_maxmind(ip)
    else:
        result = _resolve_with_ip_api(ip)

    _CACHE[ip] = result
    _cache_timestamps[ip] = now
    return result


def _resolve_with_ip_api(ip: str) -> tuple[str, str]:
    try:
        resp = requests.get(
            f"http://ip-api.com/json/{ip}",
            params={"fields": "status,country,city"},
            timeout=3,
        )
        data = resp.json()
        if data.get("status") == "success":
            return (data.get("country", "Unknown"), data.get("city", "Unknown"))
    except requests.RequestException:
        pass
    return ("Unknown", "Unknown")


def _resolve_with_maxmind(ip: str) -> tuple[str, str]:
    """
    MaxMind GeoLite2-City.mmdb dosyasını kullanır (offline, hız sınırı yok).
    Kurulum: `pip install geoip2` ve GEOLITE2_DB_PATH ortam değişkenini
    indirdiğin .mmdb dosyasının yoluna ayarla. MaxMind hesabı + ücretsiz
    lisans key'i gerekir (2024'ten beri anonim indirme kapalı).
    """
    db_path = os.getenv("GEOLITE2_DB_PATH", "./GeoLite2-City.mmdb")
    try:
        import geoip2.database  # type: ignore

        with geoip2.database.Reader(db_path) as reader:
            resp = reader.city(ip)
            country = resp.country.name or "Unknown"
            city = resp.city.name or "Unknown"
            return (country, city)
    except Exception:
        return ("Unknown", "Unknown")
