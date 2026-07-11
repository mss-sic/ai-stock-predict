package handler

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type ScreeningRow struct {
	Code              string  `json:"code"`
	Name              string  `json:"name"`
	Industry          string  `json:"industry"`
	BoardType         string  `json:"boardType"`
	LatestPrice       float64 `json:"latestPrice"`
	DirectionConsensus int    `json:"directionConsensus"`
	ExpectedReturn    float64 `json:"expectedReturn"`
	ReturnStddev      float64 `json:"returnStddev"`
	Divergence        float64 `json:"divergence"`
	Momentum          float64 `json:"momentum"`
	RiskRatio         float64 `json:"riskRatio"`
	SignalValue       float64 `json:"signalValue"`
	SignalSource      string  `json:"signalSource"`
}

type IndustryStat struct {
	Industry      string  `json:"industry"`
	Count         int     `json:"count"`
	AvgReturn     float64 `json:"avgReturn"`
	AvgConsensus  float64 `json:"avgConsensus"`
	AvgMomentum   float64 `json:"avgMomentum"`
	AvgRiskRatio  float64 `json:"avgRiskRatio"`
	BullCount     int     `json:"bullCount"`
	TopStock      string  `json:"topStock"`
	TopStockName  string  `json:"topStockName"`
	TopReturn     float64 `json:"topReturn"`
}

type BoardStat struct {
	BoardType     string  `json:"boardType"`
	Count         int     `json:"count"`
	AvgReturn     float64 `json:"avgReturn"`
	AvgConsensus  float64 `json:"avgConsensus"`
}

func PredictionScreening(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	sortBy := c.DefaultQuery("sort", "expected_return")
	order := c.DefaultQuery("order", "desc")
	industry := c.Query("industry")
	board := c.Query("board")
	minConsensus, _ := strconv.Atoi(c.DefaultQuery("minConsensus", "0"))
	keyword := c.Query("keyword")

	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 20 }

	type kdRow struct{ Code string; KDData []byte }
	var kdRows []kdRow
	db.PG.Raw("SELECT code, kd_data FROM prediction_kdist WHERE kd_data IS NOT NULL").Scan(&kdRows)

	if len(kdRows) == 0 {
		response.Success(c, gin.H{"list": []ScreeningRow{}, "total": 0, "industries": []string{}, "industryAnalysis": []IndustryStat{}})
		return
	}

	kdCodes := make([]string, 0, len(kdRows))
	for _, r := range kdRows { kdCodes = append(kdCodes, r.Code) }
	codesStr := "'" + strings.Join(kdCodes, "','") + "'"

	// Stock basic info
	nameMap := make(map[string]string)
	industryMap := make(map[string]string)
	boardMap := make(map[string]string)
	var basicRows []struct{ Code, Name, SwL1, BoardType string }
	db.PG.Raw(fmt.Sprintf(`SELECT code, COALESCE(name,code) AS name, COALESCE(sw_l1,'') AS sw_l1, COALESCE(board_type,'') AS board_type FROM stocks_basic WHERE code IN (%s)`, codesStr)).Scan(&basicRows)
	for _, b := range basicRows {
		nameMap[b.Code] = b.Name
		industryMap[b.Code] = b.SwL1
		boardMap[b.Code] = b.BoardType
	}

	// Signals
	signalMap := make(map[string]float64)
	sourceMap := make(map[string]string)
	sigRows, _ := db.PG.Raw(fmt.Sprintf(`SELECT code, COALESCE(signal_value,0), COALESCE(source,'') FROM stock_signals WHERE code IN (%s)`, codesStr)).Rows()
	if sigRows != nil {
		defer sigRows.Close()
		for sigRows.Next() {
			var code string; var sv float64; var src string
			sigRows.Scan(&code, &sv, &src)
			signalMap[code] = sv; sourceMap[code] = src
		}
	}

	// Prediction date
	var predDate string
	db.PG.Raw("SELECT to_char(MAX(updated_at), 'YYYY-MM-DD') FROM prediction_kdist").Scan(&predDate)

	// Compute all stock factors
	type stockFactor struct {
		Code              string
		Name              string
		Industry          string
		BoardType         string
		Consensus         int
		ExpReturn         float64
		RetStddev         float64
		Momentum          float64
		SignalValue       float64
		SignalSource      string
	}
	var allStocks []stockFactor

	for _, r := range kdRows {
		consensus, expRet, retStd, momentum := parseKDFactors(r.KDData)
		allStocks = append(allStocks, stockFactor{
			Code: r.Code, Name: nameMap[r.Code],
			Industry: industryMap[r.Code], BoardType: boardMap[r.Code],
			Consensus: consensus, ExpReturn: expRet, RetStddev: retStd,
			Momentum: momentum,
			SignalValue: signalMap[r.Code], SignalSource: sourceMap[r.Code],
		})
	}

	// ── Industry Analysis (on full dataset) ──
	indStatsMap := make(map[string]*IndustryStat)
	boardStatsMap := make(map[string]*BoardStat)
	consensusDist := make([]int, 8)

	for _, s := range allStocks {
		ind := s.Industry
		if ind == "" { ind = "其他" }
		if _, ok := indStatsMap[ind]; !ok {
			indStatsMap[ind] = &IndustryStat{Industry: ind}
		}
		st := indStatsMap[ind]
		st.Count++
		st.AvgReturn += s.ExpReturn
		st.AvgConsensus += float64(s.Consensus)
		st.AvgMomentum += s.Momentum
		if s.ExpReturn > 0 { st.BullCount++ }
		if s.ExpReturn > st.TopReturn {
			st.TopReturn = s.ExpReturn
			st.TopStock = s.Code
			st.TopStockName = s.Name
		}

		// Board stats
		bt := s.BoardType
		if bt == "" { bt = "其他" }
		if _, ok := boardStatsMap[bt]; !ok {
			boardStatsMap[bt] = &BoardStat{BoardType: bt}
		}
		bs := boardStatsMap[bt]
		bs.Count++
		bs.AvgReturn += s.ExpReturn
		bs.AvgConsensus += float64(s.Consensus)

		// Consensus distribution
		if s.Consensus >= 0 && s.Consensus <= 7 {
			consensusDist[s.Consensus]++
		}
	}

	// Finalize industry stats
	industryList := make([]IndustryStat, 0, len(indStatsMap))
	for _, v := range indStatsMap {
		n := float64(v.Count)
		v.AvgReturn = math.Round(v.AvgReturn/n*100) / 100
		v.AvgConsensus = math.Round(v.AvgConsensus/n*100) / 100
		v.AvgMomentum = math.Round(v.AvgMomentum/n*100) / 100
		div := 0.0
		if math.Abs(v.AvgReturn) > 0.001 { div = 0.0 }
		if v.AvgReturn > 0.001 {
			v.AvgRiskRatio = math.Round(v.AvgReturn/(div+0.01)*100) / 100
		}
		industryList = append(industryList, *v)
	}
	sort.Slice(industryList, func(i, j int) bool { return industryList[i].AvgReturn > industryList[j].AvgReturn })

	// Finalize board stats
	boardList := make([]BoardStat, 0, len(boardStatsMap))
	for _, v := range boardStatsMap {
		n := float64(v.Count)
		v.AvgReturn = math.Round(v.AvgReturn/n*100) / 100
		v.AvgConsensus = math.Round(v.AvgConsensus/n*100) / 100
		boardList = append(boardList, *v)
	}
	sort.Slice(boardList, func(i, j int) bool { return boardList[i].AvgReturn > boardList[j].AvgReturn })

	// ── Summary (full dataset) ──
	totalAll := len(allStocks)
	summary := struct {
		TotalStocks      int     `json:"totalStocks"`
		AvgReturn        float64 `json:"avgReturn"`
		StrongConsensus  int     `json:"strongConsensus"`
		PredictionDate   string  `json:"predictionDate"`
		BullRatio        float64 `json:"bullRatio"`
		AvgMomentum      float64 `json:"avgMomentum"`
	}{TotalStocks: totalAll, PredictionDate: predDate}

	sumRet, sumMom := 0.0, 0.0
	for _, s := range allStocks {
		sumRet += s.ExpReturn
		sumMom += s.Momentum
		if s.Consensus >= 5 { summary.StrongConsensus++ }
	}
	if totalAll > 0 {
		summary.AvgReturn = math.Round(sumRet/float64(totalAll)*100) / 100
		summary.AvgMomentum = math.Round(sumMom/float64(totalAll)*100) / 100
		bull := 0
		for _, s := range allStocks { if s.ExpReturn > 0 { bull++ } }
		summary.BullRatio = math.Round(float64(bull)/float64(totalAll)*10000) / 100
	}

	// ── Filter for list ──
	var filtered []stockFactor
	for _, s := range allStocks {
		if industry != "" && s.Industry != industry { continue }
		if board != "" && s.BoardType != board { continue }
		if minConsensus > 0 && s.Consensus < minConsensus { continue }
		if keyword != "" {
			kw := strings.ToLower(keyword)
			if !strings.Contains(strings.ToLower(s.Code), kw) && !strings.Contains(strings.ToLower(s.Name), kw) {
				continue
			}
		}
		filtered = append(filtered, s)
	}

	// Sort filtered
	sort.Slice(filtered, func(i, j int) bool {
		a, b := filtered[i], filtered[j]
		var cmp int
		switch sortBy {
		case "consensus": cmp = a.Consensus - b.Consensus
		case "signal":
			if a.SignalValue < b.SignalValue { cmp = -1 } else if a.SignalValue > b.SignalValue { cmp = 1 }
		case "momentum":
			if a.Momentum < b.Momentum { cmp = -1 } else if a.Momentum > b.Momentum { cmp = 1 }
		case "risk_ratio":
			ra := 0.0; if a.RetStddev > 0.001 { ra = a.ExpReturn / a.RetStddev }
			rb := 0.0; if b.RetStddev > 0.001 { rb = b.ExpReturn / b.RetStddev }
			if ra < rb { cmp = -1 } else if ra > rb { cmp = 1 }
		case "code": cmp = strings.Compare(a.Code, b.Code)
		default:
			if a.ExpReturn < b.ExpReturn { cmp = -1 } else if a.ExpReturn > b.ExpReturn { cmp = 1 }
		}
		if order == "desc" { cmp = -cmp }
		return cmp < 0
	})

	total := len(filtered)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total { start = total }
	if end > total { end = total }
	paged := filtered[start:end]

	// Convert to response rows
	list := make([]ScreeningRow, 0, len(paged))
	for _, s := range paged {
		div := 0.0
		if math.Abs(s.ExpReturn) > 0.001 { div = s.RetStddev / math.Abs(s.ExpReturn) }
		rr := 0.0
		if s.RetStddev > 0.001 { rr = s.ExpReturn / s.RetStddev }
		list = append(list, ScreeningRow{
			Code: s.Code, Name: s.Name, Industry: s.Industry, BoardType: s.BoardType,
			LatestPrice: 0, DirectionConsensus: s.Consensus,
			ExpectedReturn: math.Round(s.ExpReturn*100)/100,
			ReturnStddev: math.Round(s.RetStddev*100)/100,
			Divergence: math.Round(div*100)/100,
			Momentum: math.Round(s.Momentum*100)/100,
			RiskRatio: math.Round(rr*100)/100,
			SignalValue: math.Round(s.SignalValue*10000)/10000,
			SignalSource: s.SignalSource,
		})
	}

	// Industry names list (for filter dropdown)
	indNames := make([]string, 0, len(indStatsMap))
	for k := range indStatsMap { indNames = append(indNames, k) }
	sort.Strings(indNames)

	response.Success(c, gin.H{
		"list": list, "total": total, "page": page, "pageSize": pageSize,
		"industries": indNames, "summary": summary,
		"industryAnalysis": industryList,
		"boardAnalysis": boardList,
		"consensusDistribution": consensusDist,
	})
}

func parseKDFactors(kdJSON []byte) (consensus int, expReturn, retStddev, momentum float64) {
	var curves [][]float64
	if err := json.Unmarshal(kdJSON, &curves); err != nil || len(curves) == 0 { return }
	day20 := make([]float64, 0, len(curves))
	moms := make([]float64, 0, len(curves))
	for _, c := range curves {
		if len(c) < 3 { continue }
		lv := 0.0
		for i := len(c) - 1; i >= 0; i-- { if c[i] != 0 || i == 0 { lv = c[i]; break } }
		day20 = append(day20, lv)
		if lv > 0 { consensus++ }
		n := 5; if len(c) < n { n = len(c) }
		first5 := avgFloat(c[:n])
		ls := len(c) - 5; if ls < 0 { ls = 0 }
		last5 := avgFloat(c[ls:])
		moms = append(moms, last5-first5)
	}
	if len(day20) > 0 { expReturn = avgFloat(day20); retStddev = stddevFloat(day20, expReturn) }
	if len(moms) > 0 { momentum = avgFloat(moms) }
	return
}

func avgFloat(vals []float64) float64 {
	if len(vals) == 0 { return 0 }
	s := 0.0
	for _, v := range vals { s += v }
	return s / float64(len(vals))
}

func stddevFloat(vals []float64, mean float64) float64 {
	if len(vals) < 2 { return 0 }
	sq := 0.0
	for _, v := range vals { d := v - mean; sq += d * d }
	return math.Sqrt(sq / float64(len(vals)))
}
