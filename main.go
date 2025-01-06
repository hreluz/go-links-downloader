package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"golang.org/x/net/html"
)

type Node struct {
	Name     string
	Link     string
	IsFolder bool
	IsValid  bool
	IsRoot   bool
	Path     string
	Children []Node
}

func getEnvVariable(key string) string {
	// Load environment variables from the .env file
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	return os.Getenv(key)
}

func main() {
	// Base URL to fetch and validate against
	baseURL := getEnvVariable("BASE_URL")
	downloadsFolder := getEnvVariable("DOWNLOADS_FOLDERS")

	n := &Node{
		Link:   baseURL,
		IsRoot: true,
		Path:   downloadsFolder,
	}

	n.ProcessInitialLink()
}

func (n *Node) ProcessInitialLink() {

	links, err := extractLinks(n.Link)

	if err != nil {
		fmt.Println("Error extracting links:", err)
		return
	}

	n.ProcessLinks(links)
}

func (r *Node) ProcessLinks(links []string) {
	avoidInitialLinks, _ := strconv.Atoi(getEnvVariable("AVOID_INITIAL_LINKS"))
	r.CreateDirectoryIfNotExists()

	i := 0

	for _, link := range links {
		if i < avoidInitialLinks && avoidInitialLinks > 0 {
			i++
			continue
		}

		if isValidLink(r.Link, link) {
			fmt.Println("------------------------------------------------------------")
			fmt.Println()

			n := &Node{
				Link: link,
				Path: r.GeneratePath(),
			}

			n.AnalyzeLink(link)
			r.Children = append(r.Children, *n)

			if !n.IsFolder {
				n.DownloadFile()
			} else {
				n.ProcessInitialLink()
			}

		} else {
			fmt.Printf("The link is: %s (Invalid or goes back)\n", link)
		}
	}
}

func (n *Node) GeneratePath() string {
	name := n.GetName()
	return filepath.Join(n.Path, name)
}

func (n *Node) CreateDirectoryIfNotExists() {
	dirPath := n.GeneratePath()

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		log.Fatalf("Failed to create directory %q: %v", dirPath, err)
	}
}

func (n *Node) GetName() string {
	u, err := url.Parse(n.Link)

	if err != nil {
		panic(err)
	}

	return path.Base(u.Path)
}

func (n *Node) CheckFileExists() bool {

	if n.IsFolder {
		return false
	}

	filename := n.GeneratePath()

	_, err := os.Stat(filename)

	if os.IsNotExist(err) {
		// The file does not exist
		fmt.Printf("File %q does not exist.\n", filename)
		return false
	} else if err != nil {
		// Some other error occurred (e.g., permissions, etc.)
		fmt.Printf("Error checking the file (%s): %v\n", filename, err)
		return false
	} else {
		// The file exists
		fmt.Printf("File %q exists.\n", filename)
		return true
	}
}

func (n *Node) AnalyzeLink(url string) {
	resp, err := http.Head(url)
	if err != nil {
		log.Fatalf("Failed to make HEAD request: %v", err)
	}
	defer resp.Body.Close()

	// Check final URL in case of redirects
	finalURL := resp.Request.URL.String()
	fmt.Println("Final URL after redirect:", finalURL)

	// Inspect the status code
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Non-200 status code: %d\n", resp.StatusCode)
		return
	}

	n.IsValid = true

	// Get the Content-Type header
	contentType := resp.Header.Get("Content-Type")
	fmt.Println("Content-Type:", contentType)

	// Depending on the content type, guess if it's likely a file or an HTML page
	if strings.Contains(contentType, "text/html") {
		n.IsFolder = true
		fmt.Println("This likely leads to an HTML page with links.")
	} else {
		fmt.Println("This likely leads to a file for download.")
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

func (n *Node) DownloadFile() error {

	if n.CheckFileExists() {
		return nil
	}

	fmt.Printf("Downloading file %s", n.GetName())

	localFilePath := n.GeneratePath()

	fileURL := n.Link

	// Perform the HTTP GET request.
	resp, err := http.Get(fileURL)

	if err != nil {
		return fmt.Errorf("failed to GET file from %s: %w", fileURL, err)
	}
	defer resp.Body.Close()

	// Check the response status code.
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	// Create the local file.
	outFile, err := os.Create(localFilePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", localFilePath, err)
	}
	defer outFile.Close()

	// Copy the response body to the file.
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
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
