package handler

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

// kdFactors holds multi-horizon factors extracted from KD curves.
type kdFactors struct {
	ConsensusD5  int
	ExpReturnD5  float64
	MomentumD5   float64
	ConsensusD10 int
	ExpReturnD10 float64
	MomentumD10  float64
	ConsensusD20 int
	ExpReturnD20 float64
	MomentumD20  float64
	StddevD20    float64 // 20-day stddev across curves (for risk ratio)
}

type ScreeningRow struct {
	Code               string  `json:"code"`
	Name               string  `json:"name"`
	Industry           string  `json:"industry"`
	IndustryL2         string  `json:"industryL2"`
	BoardType          string  `json:"boardType"`
	LatestPrice        float64 `json:"latestPrice"`
	DirectionConsensus int     `json:"directionConsensus"` // dynamic: based on selected horizon
	ExpectedReturn     float64 `json:"expectedReturn"`     // dynamic
	Momentum           float64 `json:"momentum"`           // dynamic
	RiskRatio          float64 `json:"riskRatio"`
	// Fixed multi-horizon fields
	RetD5        float64 `json:"retD5"`
	ConsensusD5  int     `json:"consensusD5"`
	RetD10       float64 `json:"retD10"`
	ConsensusD10 int     `json:"consensusD10"`
	RetD20       float64 `json:"retD20"`
	ConsensusD20 int     `json:"consensusD20"`
	SignalValue  float64 `json:"signalValue"`
	SignalSource string  `json:"signalSource"`
}

type IndustryStat struct {
	Industry     string  `json:"industry"`
	Count        int     `json:"count"`
	AvgReturn    float64 `json:"avgReturn"`
	AvgConsensus float64 `json:"avgConsensus"`
	AvgMomentum  float64 `json:"avgMomentum"`
	BullCount    int     `json:"bullCount"`
	TopStock     string  `json:"topStock"`
	TopStockName string  `json:"topStockName"`
	TopReturn    float64 `json:"topReturn"`
}

type BoardStat struct {
	BoardType    string  `json:"boardType"`
	Count        int     `json:"count"`
	AvgReturn    float64 `json:"avgReturn"`
	AvgConsensus float64 `json:"avgConsensus"`
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
	horizon := c.DefaultQuery("horizon", "10")             // 5, 10, or 20
	excludeStBj := c.DefaultQuery("excludeStBj", "true")   // default: exclude ST + 北交所
	industryLevel := c.DefaultQuery("industryLevel", "l1") // l1 or l2

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Stock basic info + precomputed factors + signals (single query, no JSONB parsing)
	type stockData struct {
		Code, Name, IndL1, IndL2, Board string
		IsSt                            bool
		F                               kdFactors
		Signal                          float64
		SigSrc                          string
	}
	var allStocks []stockData

	rows, err := db.PG.Raw(`
		SELECT pf.code,
			COALESCE(sb.name, pf.code) AS name,
			COALESCE(sb.sw_l1, '') AS ind_l1,
			COALESCE(sb.sw_l2, '') AS ind_l2,
			COALESCE(sb.board_type, '') AS board,
			COALESCE(sb.is_st, false) AS is_st,
			pf.consensus_d5, pf.exp_return_d5, pf.momentum_d5,
			pf.consensus_d10, pf.exp_return_d10, pf.momentum_d10,
			pf.consensus_d20, pf.exp_return_d20, pf.momentum_d20,
			pf.stddev_d20,
			COALESCE(ss.signal_value, 0) AS signal,
			COALESCE(ss.source, '') AS sig_src
		FROM prediction_factors pf
		LEFT JOIN stocks_basic sb ON sb.code = pf.code
		LEFT JOIN stock_signals ss ON ss.code = pf.code
	`).Rows()
	if err == nil && rows != nil {
		defer rows.Close()
		for rows.Next() {
			var s stockData
			rows.Scan(&s.Code, &s.Name, &s.IndL1, &s.IndL2, &s.Board, &s.IsSt,
				&s.F.ConsensusD5, &s.F.ExpReturnD5, &s.F.MomentumD5,
				&s.F.ConsensusD10, &s.F.ExpReturnD10, &s.F.MomentumD10,
				&s.F.ConsensusD20, &s.F.ExpReturnD20, &s.F.MomentumD20,
				&s.F.StddevD20, &s.Signal, &s.SigSrc)
			if excludeStBj == "true" && (s.IsSt || s.Board == "bj") {
				continue
			}
			allStocks = append(allStocks, s)
		}
	}

	if len(allStocks) == 0 {
		response.Success(c, gin.H{
			"list": []ScreeningRow{}, "total": 0, "industries": []string{},
			"industryAnalysis": []IndustryStat{}, "boardAnalysis": []BoardStat{},
			"consensusDistribution": make([]int, 8),
			"summary": gin.H{
				"totalStocks": 0, "avgReturn": 0, "strongConsensus": 0,
				"predictionDate": "", "bullRatio": 0, "avgMomentum": 0,
			},
		})
		return
	}

	// Prediction date
	var predDate string
	db.PG.Raw("SELECT to_char(MAX(updated_at), 'YYYY-MM-DD') FROM prediction_factors").Scan(&predDate)

	// Helper: get effective industry based on level
	getInd := func(s stockData) string {
		if industryLevel == "l2" && s.IndL2 != "" {
			return s.IndL2
		}
		if s.IndL1 != "" {
			return s.IndL1
		}
		return "其他"
	}

	// Helper: get horizon-specific values
	getRet := func(f kdFactors) float64 {
		switch horizon {
		case "5":
			return f.ExpReturnD5
		case "20":
			return f.ExpReturnD20
		default:
			return f.ExpReturnD10
		}
	}
	getConsensus := func(f kdFactors) int {
		switch horizon {
		case "5":
			return f.ConsensusD5
		case "20":
			return f.ConsensusD20
		default:
			return f.ConsensusD10
		}
	}
	getMom := func(f kdFactors) float64 {
		switch horizon {
		case "5":
			return f.MomentumD5
		case "20":
			return f.MomentumD20
		default:
			return f.MomentumD10
		}
	}

	// ── Industry Analysis (full dataset) ──
	indStatsMap := make(map[string]*IndustryStat)
	boardStatsMap := make(map[string]*BoardStat)
	consensusDist := make([]int, 8)

	for _, s := range allStocks {
		ind := getInd(s)
		ret := getRet(s.F)
		con := getConsensus(s.F)

		if _, ok := indStatsMap[ind]; !ok {
			indStatsMap[ind] = &IndustryStat{Industry: ind}
		}
		st := indStatsMap[ind]
		st.Count++
		st.AvgReturn += ret
		st.AvgConsensus += float64(con)
		st.AvgMomentum += getMom(s.F)
		if ret > 0 {
			st.BullCount++
		}
		if ret > st.TopReturn {
			st.TopReturn = ret
			st.TopStock = s.Code
			st.TopStockName = s.Name
		}

		bt := s.Board
		if bt == "" {
			bt = "其他"
		}
		if _, ok := boardStatsMap[bt]; !ok {
			boardStatsMap[bt] = &BoardStat{BoardType: bt}
		}
		bs := boardStatsMap[bt]
		bs.Count++
		bs.AvgReturn += ret
		bs.AvgConsensus += float64(con)

		if con >= 0 && con <= 7 {
			consensusDist[con]++
		}
	}

	industryList := make([]IndustryStat, 0, len(indStatsMap))
	for _, v := range indStatsMap {
		n := float64(v.Count)
		v.AvgReturn = math.Round(v.AvgReturn/n*100) / 100
		v.AvgConsensus = math.Round(v.AvgConsensus/n*100) / 100
		v.AvgMomentum = math.Round(v.AvgMomentum/n*100) / 100
		industryList = append(industryList, *v)
	}
	sort.Slice(industryList, func(i, j int) bool { return industryList[i].AvgReturn > industryList[j].AvgReturn })

	boardList := make([]BoardStat, 0, len(boardStatsMap))
	for _, v := range boardStatsMap {
		n := float64(v.Count)
		v.AvgReturn = math.Round(v.AvgReturn/n*100) / 100
		v.AvgConsensus = math.Round(v.AvgConsensus/n*100) / 100
		boardList = append(boardList, *v)
	}
	sort.Slice(boardList, func(i, j int) bool { return boardList[i].AvgReturn > boardList[j].AvgReturn })

	// ── Summary ──
	totalAll := len(allStocks)
	sumRet, sumMom := 0.0, 0.0
	strongCount := 0
	bullCount := 0
	for _, s := range allStocks {
		ret := getRet(s.F)
		sumRet += ret
		sumMom += getMom(s.F)
		if getConsensus(s.F) >= 5 {
			strongCount++
		}
		if ret > 0 {
			bullCount++
		}
	}
	summary := gin.H{
		"totalStocks":     totalAll,
		"avgReturn":       math.Round(sumRet/float64(totalAll)*100) / 100,
		"strongConsensus": strongCount,
		"predictionDate":  predDate,
		"bullRatio":       math.Round(float64(bullCount)/float64(totalAll)*10000) / 100,
		"avgMomentum":     math.Round(sumMom/float64(totalAll)*100) / 100,
		"horizon":         horizon,
	}

	// ── Filter ──
	var filtered []stockData
	for _, s := range allStocks {
		ind := getInd(s)
		if industry != "" && ind != industry {
			continue
		}
		if board != "" && s.Board != board {
			continue
		}
		con := getConsensus(s.F)
		if minConsensus > 0 && con < minConsensus {
			continue
		}
		if keyword != "" {
			kw := strings.ToLower(keyword)
			if !strings.Contains(strings.ToLower(s.Code), kw) && !strings.Contains(strings.ToLower(s.Name), kw) {
				continue
			}
		}
		filtered = append(filtered, s)
	}

	// Sort
	sort.Slice(filtered, func(i, j int) bool {
		a, b := filtered[i], filtered[j]
		var cmp float64
		switch sortBy {
		case "consensus":
			cmp = float64(getConsensus(a.F) - getConsensus(b.F))
		case "signal":
			cmp = a.Signal - b.Signal
		case "momentum":
			cmp = getMom(a.F) - getMom(b.F)
		case "risk_ratio":
			ra, rb := 0.0, 0.0
			if a.F.StddevD20 > 0.001 {
				ra = getRet(a.F) / a.F.StddevD20
			}
			if b.F.StddevD20 > 0.001 {
				rb = getRet(b.F) / b.F.StddevD20
			}
			cmp = ra - rb
		case "ret_d5":
			cmp = a.F.ExpReturnD5 - b.F.ExpReturnD5
		case "ret_d10":
			cmp = a.F.ExpReturnD10 - b.F.ExpReturnD10
		case "ret_d20":
			cmp = a.F.ExpReturnD20 - b.F.ExpReturnD20
		case "code":
			cmp = float64(strings.Compare(a.Code, b.Code))
		default:
			cmp = getRet(a.F) - getRet(b.F)
		}
		if order == "desc" {
			cmp = -cmp
		}
		return cmp < 0
	})

	total := len(filtered)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	paged := filtered[start:end]

	// Convert to response
	list := make([]ScreeningRow, 0, len(paged))
	for _, s := range paged {
		rr := 0.0
		if s.F.StddevD20 > 0.001 {
			rr = getRet(s.F) / s.F.StddevD20
		}
		list = append(list, ScreeningRow{
			Code: s.Code, Name: s.Name,
			Industry: s.IndL1, IndustryL2: s.IndL2,
			BoardType:          s.Board,
			DirectionConsensus: getConsensus(s.F),
			ExpectedReturn:     math.Round(getRet(s.F)*100) / 100,
			Momentum:           math.Round(getMom(s.F)*100) / 100,
			RiskRatio:          math.Round(rr*100) / 100,
			RetD5:              math.Round(s.F.ExpReturnD5*100) / 100,
			ConsensusD5:        s.F.ConsensusD5,
			RetD10:             math.Round(s.F.ExpReturnD10*100) / 100,
			ConsensusD10:       s.F.ConsensusD10,
			RetD20:             math.Round(s.F.ExpReturnD20*100) / 100,
			ConsensusD20:       s.F.ConsensusD20,
			SignalValue:        math.Round(s.Signal*10000) / 10000,
			SignalSource:       s.SigSrc,
		})
	}

	indNames := make([]string, 0, len(indStatsMap))
	for k := range indStatsMap {
		indNames = append(indNames, k)
	}
	sort.Strings(indNames)

	response.Success(c, gin.H{
		"list": list, "total": total, "page": page, "pageSize": pageSize,
		"industries": indNames, "summary": summary,
		"industryAnalysis": industryList, "boardAnalysis": boardList,
		"consensusDistribution": consensusDist, "horizon": horizon,
	})
}
