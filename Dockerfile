# ==========================================
# STAGE 1: The Builder
# ==========================================
FROM --platform=linux/amd64 jfrog.fkinternal.com/fk-base-images/golang:1.25.5-debian12.10 AS builder

USER root
ENV GOPROXY="https://artifactory.artifactory-prod.fkcloud.in/artifactory/api/go/go_virtual"

# 1. Install Git
RUN apt-get update && apt-get install -y git && rm -rf /var/lib/apt/lists/*

WORKDIR /

# 2. Clone CORE (For Tools & ZPages)
RUN git clone --depth 1 --branch v0.141.0 https://github.com/open-telemetry/opentelemetry-collector.git otel-core

# 3. Clone CONTRIB (For Health Check) - This is the missing piece!
RUN git clone --depth 1 --branch v0.141.0 https://github.com/open-telemetry/opentelemetry-collector-contrib.git otel-contrib

# 4. Build Tools from Core
RUN cd /otel-core/cmd/builder && go install . && \
    cd ../mdatagen && go install .

# 5. Setup Build Dir
WORKDIR /build

# 6. Copy Config & Custom Code
COPY --chown=root:root builder-config.yaml .
COPY --chown=root:root processor/flipkartelbprocessor ./processor/flipkartelbprocessor

# 7. Regenerate Metadata
RUN rm -rf ./processor/flipkartelbprocessor/internal && \
    cd processor/flipkartelbprocessor && \
    mdatagen metadata.yaml && \
    go mod tidy && \
    chmod -R 777 /build

# 8. Run Builder
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    builder --config builder-config.yaml


# ==========================================
# STAGE 2: The Final Image
# ==========================================
FROM --platform=linux/amd64 jfrog.fkinternal.com/fk-base-images/debian:11.8

USER root
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

RUN groupadd -g 10001 otel && \
    useradd -u 10001 -g 10001 -m -s /bin/bash otel

RUN mkdir -p /etc/otelcol && \
    chown -R 10001:10001 /etc/otelcol

COPY --from=builder /build/dist/otelcol-custom /otelcol-contrib
RUN chmod 755 /otelcol-contrib

USER 10001
ENTRYPOINT ["/otelcol-contrib"]
CMD ["--config", "/etc/otelcol/config.yaml"]