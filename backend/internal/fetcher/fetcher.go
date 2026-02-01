package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// FetchResult contains the extracted data from a webpage
type FetchResult struct {
	URL        string
	Title      string
	Text       string   // Clean text for the AI Agent
	Links      []string // Outbound links for the Frontier
	StatusCode int
}

type Fetcher struct {
	client *http.Client
}

func NewFetcher(timeout time.Duration) *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Fetch downloads and parses a webpage
func (f *Fetcher) Fetch(ctx context.Context, url string) (*FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set a polite User-Agent (Important for not getting blocked!)
	req.Header.Set("User-Agent", "AI-Research-Agent/1.0")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	result := &FetchResult{
		URL:        url,
		StatusCode: resp.StatusCode,
		Links:      make([]string, 0),
	}

	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	// Parse HTML
	if err := f.parseHTML(resp.Body, result); err != nil {
		return nil, fmt.Errorf("parsing error: %w", err)
	}

	return result, nil
}

// parseHTML extracts text and links using the standard tokenizer
func (f *Fetcher) parseHTML(body io.Reader, result *FetchResult) error {
	tokenizer := html.NewTokenizer(body)
	var textBuilder strings.Builder
	inScript := false
	inStyle := false
	inTitle := false

	for {
		tokenType := tokenizer.Next()

		switch tokenType {
		case html.ErrorToken:
			if tokenizer.Err() == io.EOF {
				result.Text = cleanText(textBuilder.String())
				return nil
			}
			return tokenizer.Err()

		case html.StartTagToken:
			token := tokenizer.Token()
			switch token.Data {
			case "script":
				inScript = true
			case "style":
				inStyle = true
			case "title":
				inTitle = true
			case "a":
				// Extract Href
				for _, attr := range token.Attr {
					if attr.Key == "href" {
						link := cleanLink(attr.Val, result.URL)
						if link != "" {
							result.Links = append(result.Links, link)
						}
					}
				}
			}

		case html.EndTagToken:
			token := tokenizer.Token()
			switch token.Data {
			case "script":
				inScript = false
			case "style":
				inStyle = false
			case "title":
				inTitle = false
			}

		case html.TextToken:
			if inTitle {
				result.Title = tokenizer.Token().Data
			}
			if !inScript && !inStyle {
				text := strings.TrimSpace(tokenizer.Token().Data)
				if text != "" {
					textBuilder.WriteString(text + " ")
				}
			}
		}
	}
}

// cleanText removes excessive whitespace
func cleanText(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

// cleanLink handles relative URLs
func cleanLink(href, baseURL string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
		return ""
	}
	
	// Handle absolute URLs directly
	if strings.HasPrefix(href, "http") {
		return href
	}
	
	// Handle relative URLs
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	
	return base.ResolveReference(ref).String()
}
