FROM debian:bookworm

# set environment
ENV PREFER_HOST_MODE="ip"\
    BASE_DIR="/app/navapi" \
    TIME_ZONE="Asia/Shanghai"

WORKDIR $BASE_DIR

COPY ../build/navapi .

RUN chmod +x navapi

EXPOSE 8888

ENTRYPOINT ["/app/navapi/navapi"]