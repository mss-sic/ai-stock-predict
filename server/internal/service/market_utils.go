package service

import "strings"

// StockMarket returns the market/exchange for a stock code.
func StockMarket(code string) string {
	if len(code) < 3 {
		return ""
	}
	prefix := code[:3]
	switch {
	case strings.HasPrefix(prefix, "688"), strings.HasPrefix(prefix, "689"):
		return "科创板"
	case strings.HasPrefix(prefix, "300"), strings.HasPrefix(prefix, "301"):
		return "创业板"
	case strings.HasPrefix(code, "0"), strings.HasPrefix(code, "3"):
		return "深市"
	case strings.HasPrefix(code, "6"):
		return "沪市"
	case strings.HasPrefix(code, "4"), strings.HasPrefix(code, "8"):
		return "北交所"
	}
	return ""
}

// BoardLotSize returns the minimum trading unit (board lot) for a stock.
// 科创板(688/689): 200 shares, others: 100 shares.
func BoardLotSize(code string) int {
	mkt := StockMarket(code)
	if mkt == "科创板" {
		return 200
	}
	return 100
}

// FormatBoardLot rounds quantity to the nearest board lot multiple.
func FormatBoardLot(code string, qty int) int {
	lot := BoardLotSize(code)
	qty = (qty / lot) * lot
	if qty < lot {
		qty = lot
	}
	return qty
}

// MarketTag returns a short display tag for the market.
func MarketTag(code string) string {
	mkt := StockMarket(code)
	switch mkt {
	case "科创板":
		return "科创"
	case "创业板":
		return "创业"
	case "北交所":
		return "北交"
	default:
		return ""
	}
}
