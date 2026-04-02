FROM python:3.11-slim

WORKDIR /app

RUN pip install poetry && \
    poetry config virtualenvs.create false

# Clone the Apex middleware source
RUN apt-get update && apt-get install -y git && rm -rf /var/lib/apt/lists/*
RUN git clone --depth 1 https://github.com/dstl/Apex-SAPIENT-Middleware.git /tmp/apex && \
    cp -r /tmp/apex/* /app/ && \
    rm -rf /tmp/apex

RUN poetry install --no-interaction --no-root --without dev || \
    poetry install --no-interaction --no-root
RUN poetry install --no-interaction --without dev || \
    poetry install --no-interaction

COPY apex_config.ci.json /app/apex_config.json

EXPOSE 5020 5001 5002 5004 8080

CMD ["python", "-m", "sapient_apex_server.apex"]
