FROM debian:bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && update-ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# set environment
ENV PREFER_HOST_MODE="ip"\
    BASE_DIR="/app/navapi" \
    TIME_ZONE="Asia/Shanghai"

WORKDIR $BASE_DIR

COPY navapi .

RUN chmod +x navapi

EXPOSE 8888

ENTRYPOINT ["/app/navapi/navapi"]
