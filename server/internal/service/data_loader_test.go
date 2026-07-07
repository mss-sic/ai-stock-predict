package service

import (
	"testing"
)

// ═══════════════════════════════════════════════════════════════
// Test: KlineCache.GetClose
// ═══════════════════════════════════════════════════════════════

func TestKlineCacheGetClose(t *testing.T) {
	kc := &KlineCache{
		DateIdx:  map[string]int{"2024-01-03": 1},
		CloseMap: map[string][]float64{"000001": {10.0, 10.5}},
	}

	if kc.GetClose("000001", "2024-01-03") != 10.5 {
		t.Error("GetClose should return correct value")
	}
	if kc.GetClose("000002", "2024-01-03") != 0 {
		t.Error("GetClose for unknown code should return 0")
	}
	if kc.GetClose("000001", "2024-12-31") != 0 {
		t.Error("GetClose for unknown date should return 0")
	}
	if kc.GetClose("000001", "") != 0 {
		t.Error("GetClose for empty date should return 0")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test: KlineCache forward-fill verification (simulated)
// ═══════════════════════════════════════════════════════════════

func TestKlineCacheForwardFill(t *testing.T) {
	// Simulate what LoadKlineCache does: forward-fill gaps
	arr := []float64{0, 10.0, 0, 0, 10.5, 0}
	var last float64
	for i := 0; i < len(arr); i++ {
		if arr[i] > 0 {
			last = arr[i]
		} else {
			arr[i] = last
		}
	}

	expected := []float64{0, 10.0, 10.0, 10.0, 10.5, 10.5}
	for i, v := range arr {
		if v != expected[i] {
			t.Errorf("forward-fill[%d] = %v, want %v", i, v, expected[i])
		}
	}
}

// ═══════════════════════════════════════════════════════════════
// Test: StockInfo type
// ═══════════════════════════════════════════════════════════════

func TestStockInfo(t *testing.T) {
	si := StockInfo{Code: "000001", Name: "平安银行"}
	if si.Code != "000001" {
		t.Error("StockInfo.Code mismatch")
	}
	if si.Name != "平安银行" {
		t.Error("StockInfo.Name mismatch")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test: DataLoaderService construction
// ═══════════════════════════════════════════════════════════════

func TestNewDataLoaderService(t *testing.T) {
	svc := NewDataLoaderService()
	if svc == nil {
		t.Fatal("NewDataLoaderService should not return nil")
	}
	if svc.klineRepo == nil {
		t.Error("klineRepo should not be nil")
	}
	if svc.indicatorRepo == nil {
		t.Error("indicatorRepo should not be nil")
	}
}
