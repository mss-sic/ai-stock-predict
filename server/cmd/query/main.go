package main

import (
	"fmt"
	"github.com/ai-stock-predict/server/internal/config"
	"github.com/ai-stock-predict/server/internal/db"
)

func main() {
	cfg := config.Load()
	db.InitPostgres(cfg.PostgresDSN)
	db.InitMySQL(cfg.MySQLDSN)

	if db.MySQL == nil {
		fmt.Println("MySQL not initialized")
		return
	}

	// Account 2
	var id uint; var name, broker, at, bm, status string
	var ic, ta, ac, tmv, td2, tw2 float64
	db.MySQL.Raw("SELECT id, name, broker, account_type, broker_mode, initial_capital, total_assets, available_cash, total_market_value, total_deposit, total_withdraw, status FROM trading_accounts WHERE id=2").Row().Scan(&id, &name, &broker, &at, &bm, &ic, &ta, &ac, &tmv, &td2, &tw2, &status)
	fmt.Printf("ACCOUNT 2: id=%d name=%s broker=%s type=%s mode=%s status=%s\n", id, name, broker, at, bm, status)
	fmt.Printf("  init_cap=%.2f total_assets=%.2f avail_cash=%.2f total_mv=%.2f\n", ic, ta, ac, tmv)
	fmt.Printf("  deposit=%.2f withdraw=%.2f\n", td2, tw2)

	// Strategy runs  
	rows, _ := db.MySQL.Raw("SELECT id, name, status, initial_capital, available_cash, position_value, current_equity, total_return FROM strategy_runs WHERE account_id=2").Rows()
	defer rows.Close()
	fmt.Println("\n=== STRATEGY RUNS ===")
	for rows.Next() {
		var rid uint; var rn, rs string
		var ric, rac, rpv, rce, rtr float64
		rows.Scan(&rid, &rn, &rs, &ric, &rac, &rpv, &rce, &rtr)
		fmt.Printf("RUN %d: name=%s status=%s\n", rid, rn, rs)
		fmt.Printf("  init_cap=%.2f avail_cash=%.2f pos_val=%.2f equity=%.2f return=%.2f\n", ric, rac, rpv, rce, rtr)
		gap := ric - rac - rpv
		fmt.Printf("  EQUITY GAP=%.2f\n", gap)
	}

	// Cash flows
	rows2, _ := db.MySQL.Raw("SELECT id, strategy_run_id, flow_type, amount, before_cash, after_cash, COALESCE(reason,''), created_at FROM strategy_cash_flows WHERE account_id=2 ORDER BY created_at").Rows()
	defer rows2.Close()
	fmt.Println("\n=== CASH FLOWS ===")
	var td, tw float64
	for rows2.Next() {
		var fid, srid uint; var ft string; var amt, bc, ac2 float64; var reason, ca string
		rows2.Scan(&fid, &srid, &ft, &amt, &bc, &ac2, &reason, &ca)
		fmt.Printf("CF %d: run=%d type=%s amt=%.2f before=%.2f after=%.2f reason=%s %s\n", fid, srid, ft, amt, bc, ac2, reason, ca)
		if ft == "deposit" { td += amt }
		if ft == "withdraw" { tw += amt }
	}
	fmt.Printf("Total deposits=%.2f withdraws=%.2f\n", td, tw)

	// Live positions
	rows3, _ := db.MySQL.Raw("SELECT stock_code, stock_name, quantity, avg_cost, current_price FROM live_positions WHERE strategy_run_id IN (SELECT id FROM strategy_runs WHERE account_id=2) AND quantity > 0").Rows()
	defer rows3.Close()
	fmt.Println("\n=== LIVE POSITIONS ===")
	tpv := 0.0
	has := false
	for rows3.Next() {
		var sc, sn string; var qty int; var avg, cur float64
		rows3.Scan(&sc, &sn, &qty, &avg, &cur)
		mv := float64(qty) * cur
		tpv += mv
		fmt.Printf("%s %s: qty=%d avg=%.2f cur=%.2f mv=%.2f\n", sc, sn, qty, avg, cur, mv)
		has = true
	}
	if !has { fmt.Println("(none)") }

	// Holdings
	rows4, _ := db.MySQL.Raw("SELECT stock_code, stock_name, quantity, cost_price FROM holdings WHERE account_id=2 AND quantity > 0").Rows()
	defer rows4.Close()
	fmt.Println("\n=== HOLDINGS ===")
	has2 := false
	for rows4.Next() {
		var sc, sn string; var qty int; var cp float64
		rows4.Scan(&sc, &sn, &qty, &cp)
		fmt.Printf("%s %s: qty=%d cost=%.2f\n", sc, sn, qty, cp)
		has2 = true
	}
	if !has2 { fmt.Println("(none)") }
}
