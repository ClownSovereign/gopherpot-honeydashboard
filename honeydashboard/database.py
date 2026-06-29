"""
database.py
SQLite tabanlı hafif veri katmanı. Başlangıç için yeterli;
ileride PostgreSQL'e geçmek istersen sadece DATABASE_URL'i değiştirmek yeterli olur
(SQLAlchemy zaten dialect bağımsız çalışıyor).
"""
import os
from datetime import datetime

from sqlalchemy import create_engine, Column, Integer, String, DateTime, event
from sqlalchemy.orm import declarative_base, sessionmaker

DATABASE_URL = os.getenv("DATABASE_URL", "sqlite:///./honeypot.db")

# SQLite çoklu thread erişimi için check_same_thread=False gerekir
# (FastAPI istekleri farklı thread'lerde işleyebilir).
connect_args = {"check_same_thread": False} if DATABASE_URL.startswith("sqlite") else {}
engine = create_engine(DATABASE_URL, connect_args=connect_args)

# SQLite için WAL (Write-Ahead Logging) modu: okuma ve yazma işlemlerinin
# birbirini kilitlemesini büyük ölçüde azaltır. busy_timeout ile de, kısa süreli
# bir yazma çakışması olduğunda hemen "database is locked" hatası vermek yerine
# birkaç saniye bekleyip tekrar denemesini sağlıyoruz. Orta ölçekli trafik için
# (tek/birkaç GopherPot ajanı) bu yeterlidir; çok daha yüksek hacimde merkezi
# bir PostgreSQL'e geçmek gerekir (bkz. README "Mimari Tercihler" bölümü).
if DATABASE_URL.startswith("sqlite"):
    @event.listens_for(engine, "connect")
    def _set_sqlite_pragma(dbapi_connection, connection_record):
        cursor = dbapi_connection.cursor()
        cursor.execute("PRAGMA journal_mode=WAL;")
        cursor.execute("PRAGMA busy_timeout=5000;")  # ms
        cursor.close()

SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
Base = declarative_base()


class LogEntry(Base):
    """Bir saldırı/deneme kaydı."""
    __tablename__ = "logs"

    id = Column(Integer, primary_key=True, index=True)
    attacker_ip = Column(String, index=True, nullable=False)
    service_type = Column(String, index=True, nullable=False)  # "SSH" / "HTTP"
    username = Column(String, nullable=True)
    payload = Column(String, nullable=False)
    node_name = Column(String, nullable=True)
    country = Column(String, nullable=True)
    city = Column(String, nullable=True)
    timestamp = Column(DateTime, default=datetime.utcnow, index=True)


def init_db():
    Base.metadata.create_all(bind=engine)


def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()