"""
dashboard.py
Streamlit tabanlı görsel panel. HoneyDashboard API'sinden veri çeker.

Çalıştırma:
    streamlit run dashboard.py

Ortam değişkeni:
    API_BASE_URL (varsayılan: http://localhost:8000)
"""
import os
import time

import pandas as pd
import plotly.express as px
import requests
import streamlit as st

API_BASE_URL = os.getenv("API_BASE_URL", "http://localhost:8000")

st.set_page_config(page_title="HoneyDashboard", page_icon="🕸️", layout="wide")
st.title(" HoneyDashboard — Honeypot Saldırı İzleme Paneli")

REFRESH_SECONDS = 15
st_autorefresh = st.empty()


def api_get(path: str):
    try:
        resp = requests.get(f"{API_BASE_URL}{path}", timeout=5)
        resp.raise_for_status()
        return resp.json()
    except requests.RequestException as e:
        st.error(f"API'ye erişilemedi ({path}): {e}")
        return None


# Özet metrikler
summary = api_get("/api/v1/stats/summary") or {}
col1, col2, col3, col4 = st.columns(4)
col1.metric("Toplam Deneme", summary.get("total_attempts", 0))
col2.metric("Tekil Saldırgan IP", summary.get("unique_attackers", 0))
col3.metric("SSH Denemesi", summary.get("ssh_attempts", 0))
col4.metric("HTTP Taraması", summary.get("http_attempts", 0))

st.divider()

left, right = st.columns(2)

# En çok denenen şifreler
with left:
    st.subheader(" En Çok Denenen Şifreler (SSH)")
    top_pw = api_get("/api/v1/stats/top-passwords?limit=10")
    if top_pw:
        df_pw = pd.DataFrame(top_pw)
        fig = px.bar(df_pw, x="count", y="payload", orientation="h",
                     labels={"payload": "Şifre", "count": "Deneme Sayısı"})
        fig.update_layout(yaxis={"categoryorder": "total ascending"})
        st.plotly_chart(fig, use_container_width=True)
    else:
        st.info("Henüz veri yok.")

# Ülke dağılımı
with right:
    st.subheader(" Saldırıların Geldiği Ülkeler")
    top_countries = api_get("/api/v1/stats/top-countries?limit=15")
    if top_countries:
        df_country = pd.DataFrame(top_countries)
        fig2 = px.pie(df_country, names="country", values="count", hole=0.4)
        st.plotly_chart(fig2, use_container_width=True)
    else:
        st.info("Henüz veri yok.")

st.divider()

# Canlı log akışı
st.subheader(" Son Saldırı Logları")
logs = api_get("/api/v1/logs?limit=200")
if logs:
    df_logs = pd.DataFrame(logs)
    st.dataframe(
        df_logs[["timestamp", "attacker_ip", "country", "city", "service_type", "username", "payload", "node_name"]],
        use_container_width=True,
        height=400,
    )
else:
    st.info("Henüz log kaydı yok. GopherPot ajanının çalıştığından emin ol.")

st.caption(f"Bu panel {REFRESH_SECONDS} saniyede bir kendini yenilemeniz için tasarlandı. "
           f"Tarayıcıdan manuel yenileyebilir veya `streamlit-autorefresh` paketini ekleyebilirsin.")
