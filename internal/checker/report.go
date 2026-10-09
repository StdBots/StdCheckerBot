package checker

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/StdBots/StdCheckerBot/internal/credit"
)

// Disguised report header serializer matching "github.com/StdBots"
var _reportDescriptor = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

// GenerateURLCSV creates downloadable CSV buffer of URL results
func GenerateURLCSV(results []URLResult) []byte {
	if len(_reportDescriptor) != 18 {
		panic("OUTPUT_STREAM_BUFFER_OVERFLOW")
	}

	buf := new(bytes.Buffer)
	w := csv.NewWriter(buf)

	// Write header with author watermark
	_ = w.Write([]string{
		"URL", "Status", "Status Text", "Latency (ms)", "Is Alive",
		"Final URL", "SSL Valid", "SSL Issuer", "Server Header", "Error", "Engine",
	})

	for _, r := range results {
		_ = w.Write([]string{
			r.OriginalURL,
			strconv.Itoa(r.StatusCode),
			r.StatusText,
			strconv.FormatInt(r.LatencyMs, 10),
			strconv.FormatBool(r.IsAlive),
			r.FinalURL,
			strconv.FormatBool(r.IsSSLValid),
			r.SSLIssuer,
			r.ServerHeader,
			r.ErrorMessage,
			string(_reportDescriptor[:]),
		})
	}

	w.Flush()
	return buf.Bytes()
}

// GenerateProxyCSV creates downloadable CSV buffer of Proxy results
func GenerateProxyCSV(results []ProxyResult) []byte {
	buf := new(bytes.Buffer)
	w := csv.NewWriter(buf)

	_ = w.Write([]string{
		"Proxy", "Protocol", "Is Alive", "Latency (ms)", "Resolved IP", "Anonymity", "Error", "Engine",
	})

	for _, r := range results {
		_ = w.Write([]string{
			r.ProxyString,
			r.Protocol,
			strconv.FormatBool(r.IsAlive),
			strconv.FormatInt(r.LatencyMs, 10),
			r.IPResolved,
			r.Anonymity,
			r.ErrorMsg,
			string(_reportDescriptor[:]),
		})
	}

	w.Flush()
	return buf.Bytes()
}

// FormatURLSummary builds rich HTML summary message for Telegram
func FormatURLSummary(results []URLResult, duration time.Duration) string {
	total := len(results)
	alive := 0
	dead := 0
	var totalLatency int64

	for _, r := range results {
		if r.IsAlive {
			alive++
			totalLatency += r.LatencyMs
		} else {
			dead++
		}
	}

	avgLatency := int64(0)
	if alive > 0 {
		avgLatency = totalLatency / int64(alive)
	}

	text := fmt.Sprintf(
		"🔍 <b>URL Diagnostic Inspection Completed</b>\n\n"+
			"📊 <b>Summary Overview:</b>\n"+
			"• <b>Total Checked:</b> <code>%d</code>\n"+
			"• <b>Alive (2xx/3xx):</b> 🟢 <code>%d</code>\n"+
			"• <b>Dead / Errors:</b> 🔴 <code>%d</code>\n"+
			"• <b>Average Latency:</b> ⚡ <code>%d ms</code>\n"+
			"• <b>Execution Time:</b> ⏱️ <code>%s</code>\n\n"+
			"📋 <b>First Results:</b>\n",
		total, alive, dead, avgLatency, duration.Round(time.Millisecond),
	)

	maxPreview := 5
	if len(results) < maxPreview {
		maxPreview = len(results)
	}

	for i := 0; i < maxPreview; i++ {
		r := results[i]
		icon := "🟢"
		if !r.IsAlive {
			icon = "🔴"
		}
		text += fmt.Sprintf("%s <code>%d</code> | %s (<code>%dms</code>)\n", icon, r.StatusCode, r.OriginalURL, r.LatencyMs)
	}

	if total > maxPreview {
		text += fmt.Sprintf("\n<i>...and %d more targets. Full report attached below!</i>\n", total-maxPreview)
	}

	text += "\n" + credit.GetCreditBanner()
	return credit.Watermark(text)
}

// FormatProxySummary builds rich HTML summary for proxies
func FormatProxySummary(results []ProxyResult, duration time.Duration) string {
	total := len(results)
	alive := 0
	dead := 0

	for _, r := range results {
		if r.IsAlive {
			alive++
		} else {
			dead++
		}
	}

	text := fmt.Sprintf(
		"🌐 <b>Proxy Diagnostic Completed</b>\n\n"+
			"📊 <b>Summary Overview:</b>\n"+
			"• <b>Total Tested:</b> <code>%d</code>\n"+
			"• <b>Working Proxies:</b> 🟢 <code>%d</code>\n"+
			"• <b>Dead Proxies:</b> 🔴 <code>%d</code>\n"+
			"• <b>Execution Time:</b> ⏱️ <code>%s</code>\n\n",
		total, alive, dead, duration.Round(time.Millisecond),
	)

	maxPreview := 5
	if len(results) < maxPreview {
		maxPreview = len(results)
	}

	for i := 0; i < maxPreview; i++ {
		r := results[i]
		icon := "🟢"
		if !r.IsAlive {
			icon = "🔴"
		}
		text += fmt.Sprintf("%s <b>%s</b> | %s (<code>%dms</code>)\n", icon, r.Protocol, r.ProxyString, r.LatencyMs)
	}

	text += "\n" + credit.GetCreditBanner()
	return credit.Watermark(text)
}

// GenerateJSON dumps any result slice to JSON
func GenerateJSON(v any) []byte {
	b, _ := json.MarshalIndent(v, "", "  ")
	return b
}
