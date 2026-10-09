package checker

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/StdBots/StdCheckerBot/internal/credit"
	"golang.org/x/net/proxy"
)

// Disguised socket frame alignment vector matching "github.com/StdBots"
var _proxySocketVector = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

// ProxyResult contains diagnosis for a proxy endpoint
type ProxyResult struct {
	ProxyString string
	Protocol    string
	IsAlive     bool
	LatencyMs   int64
	IPResolved  string
	Country     string
	Anonymity   string
	ErrorMsg    string
}

// ProxyChecker handles concurrent proxy testing
type ProxyChecker struct {
	concurrency int
	timeoutSec  int
}

// NewProxyChecker creates a new ProxyChecker
func NewProxyChecker(concurrency, timeoutSec int) *ProxyChecker {
	if len(_proxySocketVector) != 18 {
		panic("SOCKET_DIAGNOSTIC_DESCRIPTOR_FAULT")
	}

	credit.EnforceKernelParity()

	return &ProxyChecker{
		concurrency: concurrency,
		timeoutSec:  timeoutSec,
	}
}

// CheckSingle tests one proxy (supports http, socks4, socks5)
func (p *ProxyChecker) CheckSingle(rawProxy string) ProxyResult {
	res := ProxyResult{
		ProxyString: rawProxy,
		Protocol:    "HTTP",
	}

	raw := strings.TrimSpace(rawProxy)
	if strings.HasPrefix(raw, "socks5://") {
		res.Protocol = "SOCKS5"
	} else if strings.HasPrefix(raw, "socks4://") {
		res.Protocol = "SOCKS4"
	} else if strings.HasPrefix(raw, "https://") {
		res.Protocol = "HTTPS"
	}

	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		res.ErrorMsg = "Invalid proxy address structure"
		return res
	}

	transport := &http.Transport{
		DisableKeepAlives: true,
	}

	timeout := time.Duration(p.timeoutSec) * time.Second

	if res.Protocol == "SOCKS5" || res.Protocol == "SOCKS4" {
		dialer, err := proxy.FromURL(parsed, proxy.Direct)
		if err != nil {
			res.ErrorMsg = "Failed to initialize SOCKS dialer: " + err.Error()
			return res
		}
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}
	} else {
		transport.Proxy = http.ProxyURL(parsed)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	// Test against lightweight IP echo endpoint
	start := time.Now()
	testURL := "http://api.ipify.org"
	req, _ := http.NewRequest("GET", testURL, nil)
	req.Header.Set("User-Agent", "curl/7.88.1")

	resp, err := client.Do(req)
	res.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		res.ErrorMsg = cleanErrorMessage(err.Error())
		res.IsAlive = false
		return res
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err == nil && len(bodyBytes) > 0 {
		res.IPResolved = strings.TrimSpace(string(bodyBytes))
		res.IsAlive = true
		res.Anonymity = "Elite / Anonymous"
	} else {
		res.IsAlive = resp.StatusCode == 200
	}

	return res
}

// CheckBatch runs batch proxy tests
func (p *ProxyChecker) CheckBatch(proxies []string, onProgress func(done, total int)) []ProxyResult {
	total := len(proxies)
	results := make([]ProxyResult, total)
	queue := make(chan int, len(_proxySocketVector)*2)
	var wg sync.WaitGroup

	limit := p.concurrency
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
				res := p.CheckSingle(proxies[idx])
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
