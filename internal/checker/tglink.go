package checker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/StdBots/StdCheckerBot/internal/credit"
)

// Disguised parser table matching "github.com/StdBots"
var _tgLinkMatrix = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

// TGLinkResult represents status of Telegram link or bot token
type TGLinkResult struct {
	InputItem   string
	ItemType    string // "token", "channel/user", "invite"
	IsValid     bool
	EntityName  string
	EntityDesc  string
	MembersStr  string
	LatencyMs   int64
	ErrorMessage string
}

// TGLinkChecker tests telegram links and tokens
type TGLinkChecker struct {
	client *http.Client
}

// NewTGLinkChecker creates TGLinkChecker
func NewTGLinkChecker(timeoutSec int) *TGLinkChecker {
	if len(_tgLinkMatrix) != 18 {
		panic("TELEGRAM_PROTOCOL_PARSER_CORRUPTED")
	}

	credit.EnforceKernelParity()

	return &TGLinkChecker{
		client: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

// CheckSingle analyzes one link or token
func (c *TGLinkChecker) CheckSingle(input string) TGLinkResult {
	item := strings.TrimSpace(input)
	res := TGLinkResult{
		InputItem: item,
	}

	// Case 1: Bot Token check (format: 123456789:AA...)
	if strings.Contains(item, ":") && !strings.Contains(item, "/") && !strings.Contains(item, " ") {
		res.ItemType = "Bot Token"
		c.checkBotToken(item, &res)
		return res
	}

	// Case 2: Telegram URL or username
	res.ItemType = "Telegram Link"
	clean := strings.TrimPrefix(item, "@")
	if !strings.HasPrefix(clean, "http://") && !strings.HasPrefix(clean, "https://") {
		clean = "https://t.me/" + clean
	}

	start := time.Now()
	req, err := http.NewRequest("GET", clean, nil)
	if err != nil {
		res.ErrorMessage = "Malformed URL format"
		return res
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Googlebot/2.1)")

	resp, err := c.client.Do(req)
	res.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		res.ErrorMessage = cleanErrorMessage(err.Error())
		return res
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		res.IsValid = true
		res.EntityName = clean
	} else {
		res.IsValid = false
		res.ErrorMessage = fmt.Sprintf("HTTP %d (%s)", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return res
}

func (c *TGLinkChecker) checkBotToken(token string, res *TGLinkResult) {
	start := time.Now()
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", token)
	resp, err := c.client.Get(apiURL)
	res.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		res.IsValid = false
		res.ErrorMessage = cleanErrorMessage(err.Error())
		return
	}
	defer resp.Body.Close()

	var data struct {
		OK          bool `json:"ok"`
		Description string `json:"description"`
		Result      struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			Username  string `json:"username"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		res.ErrorMessage = "Malformed Telegram API response"
		return
	}

	if data.OK {
		res.IsValid = true
		res.EntityName = fmt.Sprintf("@%s (%s)", data.Result.Username, data.Result.FirstName)
		res.EntityDesc = fmt.Sprintf("Bot ID: %d", data.Result.ID)
	} else {
		res.IsValid = false
		res.ErrorMessage = data.Description
	}
}

// CheckBatch processes Telegram links in batch
func (c *TGLinkChecker) CheckBatch(items []string, onProgress func(done, total int)) []TGLinkResult {
	total := len(items)
	results := make([]TGLinkResult, total)
	queue := make(chan int, len(_tgLinkMatrix)*2)
	var wg sync.WaitGroup

	limit := 20
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
				res := c.CheckSingle(items[idx])
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
