"""
main.py
-------
HoneyDashboard backend. Tek görevi var: GopherPot ajanlarından gelen
JSON loglarını kabul etmek, GeoIP ile zenginleştirmek ve veritabanına yazmak.
Ayrıca Streamlit dashboard'unun (ya da herhangi bir istemcinin) veriyi
okuyabilmesi için basit, salt-okunur özet endpoint'leri sunar.
"""
from datetime import datetime
from typing import Optional

from fastapi import FastAPI, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy import func
from sqlalchemy.orm import Session

from database import init_db, get_db, LogEntry as LogEntryModel
from geoip import resolve_location

app = FastAPI(
    title="HoneyDashboard API",
    description="GopherPot honeypot ajanlarından gelen saldırı loglarını toplar.",
    version="1.0.0",
)


@app.on_event("startup")
def on_startup():
    init_db()


class LogSubmitRequest(BaseModel):
    attacker_ip: str
    service_type: str  # "SSH" | "HTTP"
    payload: str
    username: Optional[str] = None
    node_name: Optional[str] = None
    timestamp: Optional[datetime] = None


class LogSubmitResponse(BaseModel):
    status: str
    id: int


@app.post("/api/v1/log-submit", response_model=LogSubmitResponse)
def submit_log(entry: LogSubmitRequest, db: Session = Depends(get_db)):
    country, city = resolve_location(entry.attacker_ip)

    db_entry = LogEntryModel(
        attacker_ip=entry.attacker_ip,
        service_type=entry.service_type.upper(),
        username=entry.username,
        payload=entry.payload,
        node_name=entry.node_name,
        country=country,
        city=city,
        timestamp=entry.timestamp or datetime.utcnow(),
    )
    db.add(db_entry)
    db.commit()
    db.refresh(db_entry)

    return LogSubmitResponse(status="ok", id=db_entry.id)


# ---- Dashboard / analiz için salt-okunur yardımcı endpoint'ler ----

@app.get("/api/v1/logs")
def list_logs(limit: int = 100, db: Session = Depends(get_db)):
    rows = (
        db.query(LogEntryModel)
        .order_by(LogEntryModel.timestamp.desc())
        .limit(min(limit, 1000))
        .all()
    )
    return [
        {
            "id": r.id,
            "attacker_ip": r.attacker_ip,
            "service_type": r.service_type,
            "username": r.username,
            "payload": r.payload,
            "node_name": r.node_name,
            "country": r.country,
            "city": r.city,
            "timestamp": r.timestamp,
        }
        for r in rows
    ]


@app.get("/api/v1/stats/top-passwords")
def top_passwords(limit: int = 10, db: Session = Depends(get_db)):
    rows = (
        db.query(LogEntryModel.payload, func.count(LogEntryModel.id).label("count"))
        .filter(LogEntryModel.service_type == "SSH")
        .group_by(LogEntryModel.payload)
        .order_by(func.count(LogEntryModel.id).desc())
        .limit(limit)
        .all()
    )
    return [{"payload": p, "count": c} for p, c in rows]


@app.get("/api/v1/stats/top-countries")
def top_countries(limit: int = 10, db: Session = Depends(get_db)):
    rows = (
        db.query(LogEntryModel.country, func.count(LogEntryModel.id).label("count"))
        .group_by(LogEntryModel.country)
        .order_by(func.count(LogEntryModel.id).desc())
        .limit(limit)
        .all()
    )
    return [{"country": c, "count": n} for c, n in rows]


@app.get("/api/v1/stats/summary")
def summary(db: Session = Depends(get_db)):
    total = db.query(func.count(LogEntryModel.id)).scalar()
    unique_ips = db.query(func.count(func.distinct(LogEntryModel.attacker_ip))).scalar()
    ssh_count = db.query(func.count(LogEntryModel.id)).filter(LogEntryModel.service_type == "SSH").scalar()
    http_count = db.query(func.count(LogEntryModel.id)).filter(LogEntryModel.service_type == "HTTP").scalar()
    return {
        "total_attempts": total,
        "unique_attackers": unique_ips,
        "ssh_attempts": ssh_count,
        "http_attempts": http_count,
    }


@app.get("/health")
def health():
    return {"status": "ok"}
