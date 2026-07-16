package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	LISTEN_IP               string = ""
	LISTEN_PORT             int    = 8000
	CRAWL4AI_ENDPOINT              = "http://crawl4ai:11235/crawl"
	CRAWL4AI_API_TOKEN             = ""
	CRAWL4AI_BROWSER_CONFIG        = map[string]interface{}{}
	CRAWL4AI_CRAWLER_CONFIG        = map[string]interface{}{}
	httpClient                     = &http.Client{
		Timeout: 60 * time.Second,
	}
)

func ReadEnvironment() {
	portStr := os.Getenv("LISTEN_PORT")
	port, err := strconv.Atoi(portStr)
	if err == nil {
		LISTEN_PORT = port
	}

	ip := os.Getenv("LISTEN_IP")
	if ip != "" {
		LISTEN_IP = ip
	}

	endpoint := os.Getenv("CRAWL4AI_ENDPOINT")
	if endpoint != "" {
		CRAWL4AI_ENDPOINT = endpoint
	}

	token := os.Getenv("CRAWL4AI_API_TOKEN")
	if token != "" {
		CRAWL4AI_API_TOKEN = token
	}

	browser_config := os.Getenv("CRAWL4AI_BROWSER_CONFIG")
	if browser_config != "" {
		err = json.Unmarshal([]byte(browser_config), &CRAWL4AI_BROWSER_CONFIG)
		if err != nil {
			log.Printf("Ignoring invalid CRAWL4AI_BROWSER_CONFIG")
		}
	}

	crawler_config := os.Getenv("CRAWL4AI_CRAWLER_CONFIG")
	if crawler_config != "" {
		err = json.Unmarshal([]byte(crawler_config), &CRAWL4AI_CRAWLER_CONFIG)
		if err != nil {
			log.Printf("Ignoring invalid CRAWL4AI_CRAWLER_CONFIG")
		}
	}
}

// For the openwebui-facing endpoint
type Request struct {
	Urls          []string               `json:"urls"`
	BrowserConfig map[string]interface{} `json:"browser_config"`
	CrawlerConfig map[string]interface{} `json:"crawler_config"`
}

type SuccessResponseItem struct {
	PageContent string            `json:"page_content"`
	Metadata    map[string]string `json:"metadata"`
}
type SuccessResponse []SuccessResponseItem

type ErrorResponse struct {
	ErrorName string `json:"error"`
	Detail    string `json:"detail"`
}

// For the crawl4ai-facing endpoint
type CrawlResponse struct {
	Results []struct {
		Url      string `json:"url"`
		Markdown struct {
			RawMarkdown string `json:"raw_markdown"`
		} `json:"markdown"`
		Metadata map[string]string `json:"metadata"`
	} `json:"results"`
}

func errorResponseFromError(name string, err error) ErrorResponse {
	return ErrorResponse{
		ErrorName: name,
		Detail:    err.Error(),
	}
}

func jsonEncode(object any) ([]byte, error) {
	return json.Marshal(object)
}

func CrawlEndpoint(response http.ResponseWriter, request *http.Request) {
	if request.Method != "POST" {
		response.WriteHeader(405)
		resp := ErrorResponse{ErrorName: "method not allowed"}
		body, _ := jsonEncode(resp)
		response.Write(body)
		log.Printf("405 method not allowed :: %s\n", request.RemoteAddr)
		return
	}

	if request.Header.Get("Content-Type") != "application/json" {
		response.WriteHeader(400)
		resp := ErrorResponse{ErrorName: "content type must be application/json"}
		body, _ := jsonEncode(resp)
		response.Write(body)
		log.Printf("400 invalid content type :: %s\n", request.RemoteAddr)
		return
	}

	var requestData Request
	err := json.NewDecoder(request.Body).Decode(&requestData)
	if err != nil {
		response.WriteHeader(400)
		resp := errorResponseFromError("invalid json", err)
		body, _ := jsonEncode(resp)
		response.Write(body)
		log.Printf("400 invalid json :: %s\n", request.RemoteAddr)
		return
	}

	if err := validateURLs(requestData.Urls); err != nil {
		response.WriteHeader(400)
		resp := errorResponseFromError("invalid urls", err)
		body, _ := jsonEncode(resp)
		response.Write(body)
		log.Printf("400 invalid urls :: %s :: %v\n", request.RemoteAddr, err)
		return
	}

	log.Printf("Request to crawl %s from %s\n", requestData.Urls, request.RemoteAddr)

	requestData.BrowserConfig = CRAWL4AI_BROWSER_CONFIG
	requestData.CrawlerConfig = CRAWL4AI_CRAWLER_CONFIG

	body, err := jsonEncode(requestData)
	if err != nil {
		response.WriteHeader(400)
		resp := errorResponseFromError("invalid json", err)
		body, _ := jsonEncode(resp)
		response.Write(body)
		log.Printf("400 invalid json :: %s\n", request.RemoteAddr)
		return
	}
	req, err := http.NewRequest("POST", CRAWL4AI_ENDPOINT, bytes.NewReader(body))
	if err != nil {
		response.WriteHeader(500)
		resp := errorResponseFromError("internal_error", err)
		body, _ := jsonEncode(resp)
		response.Write(body)
		log.Printf("500 internal_error :: %s :: %v\n", request.RemoteAddr, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	if CRAWL4AI_API_TOKEN != "" {
		req.Header.Set("Authorization", "Bearer "+CRAWL4AI_API_TOKEN)
	}

	crawlResponse, err := httpClient.Do(req)
	if err != nil || crawlResponse.StatusCode != 200 {
		response.WriteHeader(502)
		resp := ErrorResponse{ErrorName: "bad gateway"}
		body, _ := jsonEncode(resp)
		response.Write(body)
		log.Printf("502 bad gateway :: %s\n", request.RemoteAddr)
		return
	}

	var crawlData CrawlResponse
	err = json.NewDecoder(crawlResponse.Body).Decode(&crawlData)
	if err != nil {
		response.WriteHeader(502)
		resp := ErrorResponse{ErrorName: "bad gateway", Detail: "invalid json received from crawl api"}
		body, _ := jsonEncode(resp)
		response.Write(body)
		log.Printf("502 bad gateway - invalid json from crawl api :: %s\n", request.RemoteAddr)
		return
	}

	ret := SuccessResponse{}
	if crawlData.Results == nil {
		crawlData.Results = []struct {
			Url      string `json:"url"`
			Markdown struct {
				RawMarkdown string `json:"raw_markdown"`
			} `json:"markdown"`
			Metadata map[string]string `json:"metadata"`
		}{
		}
	}
	for _, result := range crawlData.Results {
		if result.Metadata == nil {
			result.Metadata = map[string]string{}
		}

		for key, value := range result.Metadata {
			if value == "" {
				delete(result.Metadata, key)
			}
		}

		result.Metadata["source"] = result.Url

		ret = append(ret, SuccessResponseItem{
			PageContent: result.Markdown.RawMarkdown,
			Metadata:    result.Metadata,
		})
	}

	response.WriteHeader(200)
	body, err = jsonEncode(ret)
	if err != nil {
		log.Printf("500 internal_error :: %s :: %v\n", request.RemoteAddr, err)
		return
	}
	response.Write(body)
	log.Printf("200 :: %s :: %d results\n", request.RemoteAddr, len(ret))
}

func validateURLs(urls []string) error {
	if len(urls) == 0 {
		return fmt.Errorf("urls array is empty")
	}
	for i, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			return fmt.Errorf("url at index %d is empty", i)
		}
		parsed, err := url.Parse(u)
		if err != nil {
			return fmt.Errorf("url at index %d is invalid: %v", i, err)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fmt.Errorf("url at index %d must use http or https scheme", i)
		}
	}
	return nil
}

func main() {
	ReadEnvironment()

	http.HandleFunc("/crawl", CrawlEndpoint)

	listenAddress := fmt.Sprintf("%s:%d", LISTEN_IP, LISTEN_PORT)
	log.Printf("Listening on %s\n", listenAddress)

	err := http.ListenAndServe(listenAddress, nil)
	if err != nil {
		log.Println(err)
	}
}
