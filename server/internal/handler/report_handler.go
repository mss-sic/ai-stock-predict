package handler

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

const pdfCacheDir = "data/reports/pdf"

func init() {
	os.MkdirAll(pdfCacheDir, 0755)
}

// ServeReportProxy opens East Money report page or PDF
func ServeReportProxy(c *gin.Context) {
	infoCode := c.Query("infoCode")
	if infoCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing infoCode"})
		return
	}
	// PDF URL format: https://pdf.dfcfw.com/pdf/H3_{infoCode}_1.pdf
	fullURL := fmt.Sprintf("https://pdf.dfcfw.com/pdf/H3_%s_1.pdf?%d.pdf", infoCode, time.Now().UnixMilli())

	// Check local cache
	cacheKey := fmt.Sprintf("%x", md5.Sum([]byte(fullURL)))
	cachePath := filepath.Join(pdfCacheDir, cacheKey+".pdf")

	if info, err := os.Stat(cachePath); err == nil && info.Size() > 0 {
		log.Printf("[report-pdf] cache hit: %s (%d bytes)", cacheKey, info.Size())
		c.File(cachePath)
		return
	}

	// Download from East Money
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "request build failed")
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://data.eastmoney.com/")

	resp, err := client.Do(req)
	if err != nil {
		c.String(http.StatusBadGateway, "download failed")
		return
	}
	defer resp.Body.Close()

	log.Printf("[report-pdf] East Money response: status=%d, content-type=%s, len=%d", resp.StatusCode, resp.Header.Get("Content-Type"), resp.ContentLength)

	if resp.StatusCode != http.StatusOK {
		// Fallback: redirect to report detail page
		redirectURL := fmt.Sprintf("https://data.eastmoney.com/report/info/%s.html", infoCode)
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil || len(body) < 4 || string(body[:4]) != "%PDF" {
		redirectURL := fmt.Sprintf("https://data.eastmoney.com/report/info/%s.html", infoCode)
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	// Save to cache
	if err := os.WriteFile(cachePath, body, 0644); err != nil {
		log.Printf("[report-pdf] cache write error: %v", err)
	} else {
		log.Printf("[report-pdf] cached: %s (%d bytes)", cacheKey, len(body))
	}

	c.Data(http.StatusOK, "application/pdf", body)
}

func ServeReportPDF(c *gin.Context) {
	infoCode := c.Query("infoCode")
	if infoCode == "" {
		infoCode = c.Query("url") // backward compat
	}
	if infoCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing infoCode"})
		return
	}

	fullURL := fmt.Sprintf("https://pdf.dfcfw.com/pdf/H3_%s_1.pdf?%d.pdf", infoCode, time.Now().UnixMilli())

	cacheKey := fmt.Sprintf("%x", md5.Sum([]byte(fullURL)))
	cachePath := filepath.Join(pdfCacheDir, cacheKey+".pdf")

	if info, err := os.Stat(cachePath); err == nil && info.Size() > 0 {
		log.Printf("[report-pdf] cache hit: %s (%d bytes)", cacheKey, info.Size())
		c.File(cachePath)
		return
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "request build failed")
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://data.eastmoney.com/")

	resp, err := client.Do(req)
	if err != nil {
		c.String(http.StatusBadGateway, "download failed")
		return
	}
	defer resp.Body.Close()

	log.Printf("[report-pdf] East Money response: status=%d, content-type=%s, len=%d", resp.StatusCode, resp.Header.Get("Content-Type"), resp.ContentLength)

	if resp.StatusCode != http.StatusOK {
		c.String(http.StatusNotFound, "PDF not available")
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, "read failed")
		return
	}

	if len(body) > 4 && string(body[:4]) == "%PDF" {
		os.WriteFile(cachePath, body, 0644)
		log.Printf("[report-pdf] cached: %s (%d bytes)", cacheKey, len(body))
	}

	c.Data(http.StatusOK, "application/pdf", body)
}