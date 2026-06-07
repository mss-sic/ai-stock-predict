package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const scriptsDir = "scripts/collector"

func scriptsRoot() string {
	if root := os.Getenv("APP_ROOT"); root != "" {
		return filepath.Join(root, scriptsDir)
	}
	_, f, _, _ := runtime.Caller(0)
	base := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(f))))
	return filepath.Join(base, scriptsDir)
}

type KLineData struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	Close  float64 `json:"close"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Volume int64   `json:"volume"`
}

type BasicData struct {
	Code                 string  `json:"code"`
	Name                 string  `json:"name"`
	Price                float64 `json:"price"`
	PE                   float64 `json:"pe"`
	PB                   float64 `json:"pb"`
	MarketCap            float64 `json:"marketCap"`
	CirculatingMarketCap float64 `json:"circulatingMarketCap"`
	TurnoverRate         float64 `json:"turnoverRate"`
	Error                string  `json:"error"`
}

func runPython(script string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmdArgs := append([]string{filepath.Join(scriptsRoot(), script)}, args...)
	cmd := exec.CommandContext(ctx, "python3", cmdArgs...)
	return cmd.Output()
}

func FetchKLine(code string, days int) ([]KLineData, error) {
	out, err := runPython("daily_k.py", "--code", code, "--days", fmt.Sprintf("%d", days))
	if err != nil {
		return nil, fmt.Errorf("python kline error: %w", err)
	}
	var result []KLineData
	if err := json.Unmarshal(out, &result); err != nil {
		var errResp map[string]interface{}
		json.Unmarshal(out, &errResp)
		if msg, ok := errResp["error"]; ok {
			return nil, fmt.Errorf("kline error: %v", msg)
		}
		return nil, err
	}
	return result, nil
}

func FetchBasic(code string) (*BasicData, error) {
	out, err := runPython("stock_basic.py", "--code", code)
	if err != nil {
		return nil, fmt.Errorf("python basic error: %w", err)
	}
	var result BasicData
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, fmt.Errorf("basic error: %s", result.Error)
	}
	return &result, nil
}
