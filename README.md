# crawl4ai OpenWebUI proxy
This simple proxy server can be run in a docker container to let an [OpenWebUI](https://github.com/open-webui/open-webui) instance interact with a [crawl4ai](https://github.com/unclecode/crawl4ai) instance.
This makes the OpenWebUI's web search feature a lot faster and way more usable without paying for an API service. 🎉

## Features
- URL validation (http/https only, non-empty)
- API token support for crawl4ai authentication
- Browser and crawler configuration via environment variables
- Graceful error handling with HTTP timeouts

## Fork Notes
This is a fork of [lennyerik/crawl4ai-proxy](https://github.com/lennyerik/crawl4ai-proxy) with additional enhancements.

## Usage
Given a `compose.yml` file that looks something like this:

```
services:
    crawl4ai-proxy:
        image: ghcr.io/amulet1/crawl4ai-proxy:latest
        environment:
            - LISTEN_PORT=8000
            - CRAWL4AI_ENDPOINT=http://crawl4ai:11235/crawl
            - CRAWL4AI_API_TOKEN=your_token_here
            - CRAWL4AI_BROWSER_CONFIG='{"viewport": {"width": 1920, "height": 1080}}'
            - CRAWL4AI_CRAWLER_CONFIG='{"timeout": 30}'
        networks:
            - openwebui

    openwebui:
        image: ghcr.io/open-webui/open-webui:ollama
        ports:
            - "8080:8080"
        deploy:
            resources:
                reservations:
                    devices:
                        - driver: nvidia
                          count: all
                          capabilities: [gpu]
        networks:
            - openwebui

    crawl4ai:
        image: unclecode/crawl4ai:0.6.0-r2
        shm_size: 1g
        networks:
            - openwebui

networks:
    - openwebui
```

Run `docker compose up -d`, visit `localhost:8080` in a browser, navigate to `Admin Panel->Web Search` and under the "Loader" section, set

    Web Loader Engine: external
    External Web Loader URL: http://crawl4ai-proxy:8000/crawl
    External Web Loader API Key: * (doesn't matter, but is a required field)

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LISTEN_PORT` | Port the proxy listens on | `8000` |
| `LISTEN_IP` | IP address to bind to | `""` (all interfaces) |
| `CRAWL4AI_ENDPOINT` | URL of the crawl4ai instance | `http://crawl4ai:11235/crawl` |
| `CRAWL4AI_API_TOKEN` | API token for crawl4ai authentication | `""` |
| `CRAWL4AI_BROWSER_CONFIG` | Browser configuration as JSON | `{}` |
| `CRAWL4AI_CRAWLER_CONFIG` | Crawler configuration as JSON | `{}` |
