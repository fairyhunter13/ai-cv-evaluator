FROM python:3.11-alpine

WORKDIR /app

COPY exporter.py /app/exporter.py

RUN pip install docker prometheus-client

CMD ["python", "exporter.py"]
