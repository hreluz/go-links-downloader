package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

func main() {
	// Base URL to fetch and validate against
	baseURL := "http://example.com/folders/abc"

	links, err := extractLinks(baseURL)
	if err != nil {
		fmt.Println("Error extracting links:", err)
		return
	}

	fmt.Println("Links found:")
	for _, link := range links {
		if isValidLink(baseURL, link) {
			fmt.Printf("The link is: %s (Valid and does not go back)\n", link)
		} else {
			fmt.Printf("The link is: %s (Invalid or goes back)\n", link)
		}
	}
}

// Function to extract all links from the target URL
func extractLinks(targetURL string) ([]string, error) {
	resp, err := http.Get(targetURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch URL: %s (status code: %d)", targetURL, resp.StatusCode)
	}

	links := []string{}
	z := html.NewTokenizer(resp.Body)
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			// End of the document
			return links, nil
		case html.StartTagToken:
			token := z.Token()
			if token.Data == "a" {
				for _, attr := range token.Attr {
					if attr.Key == "href" {
						// Resolve relative URLs to absolute ones
						resolvedURL, err := resolveURL(targetURL, attr.Val)
						if err == nil {
							links = append(links, resolvedURL)
						}
					}
				}
			}
		}
	}
}

// Function to resolve relative URLs to absolute URLs
func resolveURL(baseURL, href string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(parsed).String(), nil
}

// Function to validate if the link does not "go back"
func isValidLink(baseURL, link string) bool {
	base, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	parsedLink, err := url.Parse(link)
	if err != nil {
		return false
	}

	// Ensure the scheme and host are the same
	if base.Scheme != parsedLink.Scheme || base.Host != parsedLink.Host {
		return false
	}

	// Ensure the path of the link starts with the base path and is not "shorter"
	if !strings.HasPrefix(parsedLink.Path, base.Path) {
		return false
	}

	return len(parsedLink.Path) >= len(base.Path)
}
