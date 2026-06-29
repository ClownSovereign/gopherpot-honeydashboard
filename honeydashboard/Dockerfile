FROM python:3.12-slim
WORKDIR /app

# Veritabanı dosyasının yazılacağı dizini önceden oluşturuyoruz,
# ki appuser'a geçtikten sonra yazma izni sorunu yaşanmasın.
RUN mkdir -p /app/data

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .

# Container içinde root yetkisiyle çalışmayı önlemek için sınırlı kullanıcı.
# Bir RCE zafiyeti çıksa bile saldırgan root yetkisi elde edemez.
RUN useradd -u 10001 -U appuser && chown -R appuser:appuser /app
USER appuser

EXPOSE 8000
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]