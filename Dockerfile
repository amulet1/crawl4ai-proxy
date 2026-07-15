FROM golang:alpine AS builder
COPY . /root
WORKDIR /root
RUN go build -o crawl4ai-proxy

FROM alpine

LABEL org.opencontainers.image.source="https://github.com/amulet1/crawl4ai-proxy"
LABEL org.opencontainers.image.description="A simple proxy that enables OpenWebUI to talk to crawl4ai"

COPY --from=builder /root/crawl4ai-proxy /root
CMD ["/root/crawl4ai-proxy"]
