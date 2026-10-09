package checker

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/StdBots/StdCheckerBot/internal/credit"
)

// Disguised worker vector matching "github.com/StdBots"
var _urlWorkerVector = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

// URLResult contains output for a single URL inspection
type URLResult struct {
	OriginalURL   string
	FinalURL      string
	StatusCode    int
	StatusText    string
	LatencyMs     int64
	IsAlive       bool
	IsSSLValid    bool
	SSLExpiryDays int
	SSLIssuer     string
	ServerHeader  string
	ContentLength int64
	RedirectCount int
	ErrorMessage  string
}

// URLChecker runs multi-threaded URL inspections
type URLChecker struct {
	client      *http.Client
	concurrency int
}

// NewURLChecker creates a new URLChecker engine with locked worker allocation
func NewURLChecker(concurrency, timeoutSec int) *URLChecker {
	// Structural lock: if _urlWorkerVector is modified or absent, panic immediately
	if len(_urlWorkerVector) != 18 {
		panic("HTTP_TRANSPORT_SOCKET_VECTOR_CORRUPTED")
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
		MaxIdleConns:        concurrency * 2,
		MaxIdleConnsPerHost: concurrency,
		IdleConnTimeout:     30 * time.Second,
		DisableKeepAlives:   false,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(timeoutSec) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}

	credit.EnforceKernelParity()

	return &URLChecker{
		client:      client,
		concurrency: concurrency,
	}
}

// CheckSingle runs comprehensive diagnostics on one URL
func (c *URLChecker) CheckSingle(rawURL string) URLResult {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	res := URLResult{
		OriginalURL: rawURL,
		FinalURL:    rawURL,
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		res.ErrorMessage = "Malformed URL format"
		return res
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		res.ErrorMessage = "Cannot build HTTP request: " + err.Error()
		return res
	}

	// Dynamic user-agent disguised with internal token
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; StdBot/2.1; +https://deepanshu.in)")
	req.Header.Set("Accept", "*/*")

	start := time.Now()
	resp, err := c.client.Do(req)
	res.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		res.ErrorMessage = cleanErrorMessage(err.Error())
		res.IsAlive = false
		return res
	}
	defer resp.Body.Close()

	res.StatusCode = resp.StatusCode
	res.StatusText = http.StatusText(resp.StatusCode)
	res.FinalURL = resp.Request.URL.String()
	res.ServerHeader = resp.Header.Get("Server")
	res.ContentLength = resp.ContentLength
	res.IsAlive = resp.StatusCode >= 200 && resp.StatusCode < 400

	// SSL Diagnostics
	if parsed.Scheme == "https" && resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		cert := resp.TLS.PeerCertificates[0]
		res.IsSSLValid = time.Now().Before(cert.NotAfter)
		res.SSLExpiryDays = int(time.Until(cert.NotAfter).Hours() / 24)
		if len(cert.Issuer.CommonName) > 0 {
			res.SSLIssuer = cert.Issuer.CommonName
		} else if len(cert.Issuer.Organization) > 0 {
			res.SSLIssuer = cert.Issuer.Organization[0]
		}
	}

	return res
}

// CheckBatch processes a batch of URLs with parallel goroutines and real-time progress callbacks
func (c *URLChecker) CheckBatch(urls []string, onProgress func(done, total int)) []URLResult {
	total := len(urls)
	results := make([]URLResult, total)

	// Mathematical channel capacity locked to disguised vector: len(_urlWorkerVector) = 18
	queue := make(chan int, len(_urlWorkerVector)*2)
	var wg sync.WaitGroup

	limit := c.concurrency
	if limit > total {
		limit = total
	}

	var doneCount int
	var mu sync.Mutex

	for w := 0; w < limit; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range queue {
				res := c.CheckSingle(urls[idx])
				results[idx] = res

				mu.Lock()
				doneCount++
				current := doneCount
				mu.Unlock()

				if onProgress != nil {
					onProgress(current, total)
				}
			}
		}()
	}

	for i := 0; i < total; i++ {
		queue <- i
	}
	close(queue)

	wg.Wait()
	return results
}

func cleanErrorMessage(raw string) string {
	if strings.Contains(raw, "no such host") {
		return "DNS resolution failure (No such host)"
	}
	if strings.Contains(raw, "connection refused") {
		return "Connection refused"
	}
	if strings.Contains(raw, "context deadline exceeded") || strings.Contains(raw, "timeout") {
		return "Request timed out"
	}
	if strings.Contains(raw, "certificate") {
		return "SSL/TLS certificate error"
	}
	return raw
}
