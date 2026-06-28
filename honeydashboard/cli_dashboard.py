"""
cli_dashboard.py
"Ben tamamen terminalciyim" diyenler için Rich tabanlı canlı panel.

Çalıştırma:
    python cli_dashboard.py
"""
import os
import time

import requests
from rich.console import Console
from rich.layout import Layout
from rich.live import Live
from rich.panel import Panel
from rich.table import Table

API_BASE_URL = os.getenv("API_BASE_URL", "http://localhost:8000")
REFRESH_SECONDS = float(os.getenv("REFRESH_SECONDS", "5"))

console = Console()


def api_get(path: str):
    try:
        resp = requests.get(f"{API_BASE_URL}{path}", timeout=5)
        resp.raise_for_status()
        return resp.json()
    except requests.RequestException:
        return None


def build_summary_panel() -> Panel:
    s = api_get("/api/v1/stats/summary") or {}
    text = (
        f"[bold]Toplam Deneme:[/bold] {s.get('total_attempts', 0)}\n"
        f"[bold]Tekil Saldırgan:[/bold] {s.get('unique_attackers', 0)}\n"
        f"[bold]SSH:[/bold] {s.get('ssh_attempts', 0)}   "
        f"[bold]HTTP:[/bold] {s.get('http_attempts', 0)}"
    )
    return Panel(text, title=" Özet", border_style="cyan")


def build_logs_table() -> Table:
    table = Table(title=" Son Saldırı Logları", expand=True)
    table.add_column("Zaman", style="dim", width=20)
    table.add_column("IP", style="bold red")
    table.add_column("Ülke")
    table.add_column("Servis")
    table.add_column("Kullanıcı")
    table.add_column("Payload", overflow="fold")

    logs = api_get("/api/v1/logs?limit=25") or []
    for row in logs:
        ts = str(row.get("timestamp", ""))[:19]
        table.add_row(
            ts,
            row.get("attacker_ip", ""),
            row.get("country", "?"),
            row.get("service_type", ""),
            row.get("username") or "-",
            row.get("payload", ""),
        )
    return table


def build_top_passwords_table() -> Table:
    table = Table(title=" En Çok Denenen Şifreler")
    table.add_column("Şifre")
    table.add_column("Sayı", justify="right")
    for row in (api_get("/api/v1/stats/top-passwords?limit=10") or []):
        table.add_row(row["payload"], str(row["count"]))
    return table


def render() -> Layout:
    layout = Layout()
    layout.split_column(
        Layout(build_summary_panel(), size=5),
        Layout(name="lower"),
    )
    layout["lower"].split_row(
        Layout(build_logs_table(), ratio=2),
        Layout(build_top_passwords_table(), ratio=1),
    )
    return layout


def main():
    with Live(render(), console=console, refresh_per_second=1, screen=True) as live:
        while True:
            time.sleep(REFRESH_SECONDS)
            live.update(render())


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        console.print("\n[yellow]Görüşürüz![/yellow]")
