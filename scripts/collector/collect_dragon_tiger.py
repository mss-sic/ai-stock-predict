#!/usr/bin/env python3
"""
龙虎榜数据采集 — 全市场每日榜单 + 个股席位明细
数据源: 东财 datacenter-web (零鉴权)
用法: python3 collect_dragon_tiger.py [YYYY-MM-DD]  (默认今天)
"""
import os, sys, time, ssl, random, urllib.request, json
import sys
import psycopg2
from psycopg2.extras import execute_values

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
DTL_URL = "https://datacenter-web.eastmoney.com/api/data/v1/get"
UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"

# 限流控制
_last_call = [0.0]
def em_req(url, params, timeout=15):
    wait = 1.0 - (time.time() - _last_call[0])
    if wait > 0:
        time.sleep(wait + random.uniform(0.1, 0.5))
    try:
        req = urllib.request.Request(url, headers={"User-Agent": UA, "Referer": "https://data.eastmoney.com/"})
        data = urllib.parse.urlencode(params, doseq=True).encode("utf-8")
        with urllib.request.urlopen(req, data=data, timeout=timeout) as resp:
            result = json.loads(resp.read().decode("utf-8"))
        return result
    finally:
        _last_call[0] = time.time()

def datacenter_query(report_name, filter_str="", page_size=50, sort_columns="", sort_types="-1"):
    params = {
        "reportName": report_name, "columns": "ALL",
        "filter": filter_str, "pageNumber": "1", "pageSize": str(page_size),
        "sortColumns": sort_columns, "sortTypes": sort_types,
        "source": "WEB", "client": "WEB",
    }
    r = em_req(DTL_URL, params)
    data = r.get("result", {})
    if data and data.get("data"):
        return data["data"]
    return []

def daily_dragon_tiger(trade_date):
    """全市场龙虎榜汇总"""
    data = datacenter_query(
        "RPT_DAILYBILLBOARD_DETAILSNEW",
        filter_str=f"(TRADE_DATE>='{trade_date}')(TRADE_DATE<='{trade_date}')",
        page_size=500,
        sort_columns="BILLBOARD_NET_AMT", sort_types="-1",
    )
    stocks = []
    for row in data:
        stocks.append((
            row.get("SECURITY_CODE", ""),
            row.get("SECURITY_NAME_ABBR", ""),
            str(row.get("TRADE_DATE", ""))[:10],
            row.get("EXPLANATION", "") or "",
            row.get("CLOSE_PRICE") or 0,
            round(float(row.get("CHANGE_RATE") or 0), 2),
            round((row.get("BILLBOARD_NET_AMT") or 0) / 10000, 1),
            round((row.get("BILLBOARD_BUY_AMT") or 0) / 10000, 1),
            round((row.get("BILLBOARD_SELL_AMT") or 0) / 10000, 1),
            round(float(row.get("TURNOVERRATE") or 0), 2),
        ))
    return stocks

def collect_dragon_tiger_detail(code, trade_date):
    """个股上榜席位明细 — 合并买卖双方同一席位"""
    seats = []
    # 买入席位（用 BUY 接口获取完整买卖数据）
    buy_data = datacenter_query(
        "RPT_BILLBOARD_DAILYDETAILSBUY",
        filter_str=f"(TRADE_DATE='{trade_date}')(SECURITY_CODE=\"{code}\")",
        page_size=10, sort_columns="BUY", sort_types="-1",
    )
    for row in buy_data:
        seat_code = str(row.get("OPERATEDEPT_CODE", ""))
        buy_amt = round((row.get("BUY") or 0) / 10000, 1)
        sell_amt = round((row.get("SELL") or 0) / 10000, 1)
        net = round((row.get("NET") or 0) / 10000, 1)
        seats.append((
            code, trade_date,
            row.get("OPERATEDEPT_NAME", ""),
            seat_code,
            "buy" if net >= 0 else "sell",
            buy_amt, sell_amt, net,
            seat_code == "0",
        ))
    return seats

def main():
    today = sys.argv[1] if len(sys.argv) > 1 else time.strftime("%Y-%m-%d")
    print(f"[龙虎榜] 采集日期: {today}", flush=True)

    # 1. 采集全市场龙虎榜
    print("[龙虎榜] 正在获取全市场龙虎榜...")
    dt_stocks = daily_dragon_tiger(today)
    print(f"[龙虎榜] 获取到 {len(dt_stocks)} 只上榜股票", flush=True)

    if not dt_stocks:
        print("[龙虎榜] ⚠️ 无上榜数据（非交易日或盘后未更新）")
        return

    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # 去重: 同一 code + trade_date 只保留一条
    seen_list = set()
    dt_stocks_dedup = []
    for s in dt_stocks:
        key = (s[0], s[2])
        if key not in seen_list:
            seen_list.add(key)
            dt_stocks_dedup.append(s)
    dt_stocks = dt_stocks_dedup

    # 写入龙虎榜汇总
    sql_list = """
        INSERT INTO dragon_tiger_list (code, name, trade_date, reason, close_price, change_pct,
            net_buy_amt, buy_amt, sell_amt, turnover_pct)
        VALUES %s
        ON CONFLICT (code, trade_date) DO UPDATE SET
            name = EXCLUDED.name, reason = EXCLUDED.reason, close_price = EXCLUDED.close_price,
            change_pct = EXCLUDED.change_pct, net_buy_amt = EXCLUDED.net_buy_amt,
            buy_amt = EXCLUDED.buy_amt, sell_amt = EXCLUDED.sell_amt, turnover_pct = EXCLUDED.turnover_pct
    """
    execute_values(cur, sql_list, dt_stocks, page_size=200)
    conn.commit()
    print(f"[龙虎榜] ✓ 写入汇总 {len(dt_stocks)} 条", flush=True)

    # 2. 逐只采集席位明细
    all_seats = []
    for i, s in enumerate(dt_stocks):
        code = s[0]
        if (i + 1) % 20 == 0:
            print(f"[龙虎榜] 席位采集进度: {i+1}/{len(dt_stocks)}", flush=True)
        try:
            seats = collect_dragon_tiger_detail(code, today)
            all_seats.extend(seats)
        except Exception as e:
            print(f"[龙虎榜] ⚠️ {code} 席位采集失败: {e}", flush=True)
            continue

    if all_seats:
        # 去重: code + trade_date + seat_code
        seen = set()
        unique_seats = []
        for seat in all_seats:
            key = (seat[0], seat[1], seat[3])
            if key not in seen:
                seen.add(key)
                unique_seats.append(seat)

        sql_detail = """
            INSERT INTO dragon_tiger_detail (code, trade_date, seat_name, seat_code, side,
                buy_amt, sell_amt, net_amt, is_institution)
            VALUES %s
        """
        execute_values(cur, sql_detail, unique_seats, page_size=200)
        conn.commit()
        print(f"[龙虎榜] ✓ 写入席位明细 {len(unique_seats)} 条", flush=True)

    cur.close()
    conn.close()
    print("[龙虎榜] 采集完成", flush=True)

    # STAT output for engine: report actual counts
    dt_count = len(dt_stocks) if 'dt_stocks' in dir() else 0
    seat_count = len(unique_seats) if 'unique_seats' in dir() else 0
    print(f"STAT:records_new={dt_count + seat_count},records_skip=0,records_err=0,dragon_tiger_list={dt_count},dragon_tiger_detail={seat_count}", flush=True)

if __name__ == "__main__":
    main()
