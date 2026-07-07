#!/usr/bin/env python3
"""
分红送转数据采集 — 历史每股派息/送股/转增
数据源: 东财 datacenter RPT_SHAREBONUS_DET (零鉴权)
用法: python3 collect_dividend.py [--sample | CODE]  (默认: 全市场采集)
"""
import os, sys, time, ssl, random, urllib.request, json
import psycopg2
from psycopg2.extras import execute_values

# 跨交易所样本池：沪市主板+科创板 + 深市主板+中小板+创业板
SAMPLE_CODES = ['600519','601318','688017','000001','002475','300750','600036','688981']

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
DTL_URL = "https://datacenter-web.eastmoney.com/api/data/v1/get"
UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"

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
    return data.get("data", []) if data else []

def collect_dividend_for_code(code, page_size=20):
    data = datacenter_query(
        "RPT_SHAREBONUS_DET",
        filter_str=f'(SECURITY_CODE="{code}")',
        page_size=page_size, sort_columns="EX_DIVIDEND_DATE", sort_types="-1",
    )
    rows = []
    for row in data:
        ex_date = row.get("EX_DIVIDEND_DATE")
        if ex_date is None or str(ex_date).strip() == "" or str(ex_date).strip() == "None":
            continue
        rows.append((
            code,
            str(ex_date)[:10],
            row.get("PRETAX_BONUS_RMB", 0) or 0,
            row.get("TRANSFER_RATIO", 0) or 0,
            row.get("BONUS_RATIO", 0) or 0,
            row.get("ASSIGN_PROGRESS", "") or "",
        ))
    return rows

def main():
    today = time.strftime("%Y-%m-%d")
    print(f"[分红] 开始采集 → 日期: {today}")
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    if '--sample' in sys.argv:
        codes = SAMPLE_CODES
        print(f"[分红] 样本模式: {len(codes)} 只")
    elif len(sys.argv) > 1:
        codes = [sys.argv[1]]
        print(f"[分红] 单股模式: {codes[0]}")
    else:
        cur.execute("SELECT code FROM stocks_basic WHERE code IS NOT NULL AND code != '' ORDER BY code")
        codes = [row[0] for row in cur.fetchall()]
        print(f"[分红] 全量模式: {len(codes)} 只")

    total, skip, errors = 0, 0, 0
    for i, code in enumerate(codes):
        if (i + 1) % 100 == 0:
            print(f"[分红] 进度: {i+1}/{len(codes)} (新增 {total}, 跳过 {skip}, 错误 {errors})")
        try:
            rows = collect_dividend_for_code(code)
            if not rows:
                skip += 1
                continue
            sql = """
                INSERT INTO dividend_history (code, ex_dividend_date, bonus_rmb, transfer_ratio, bonus_ratio, progress)
                VALUES %s
                ON CONFLICT (code, ex_dividend_date) DO UPDATE SET
                    bonus_rmb = EXCLUDED.bonus_rmb, transfer_ratio = EXCLUDED.transfer_ratio,
                    bonus_ratio = EXCLUDED.bonus_ratio, progress = EXCLUDED.progress
            """
            execute_values(cur, sql, rows, page_size=200)
            conn.commit()
            total += len(rows)
        except Exception as e:
            print(f"[分红] ⚠️ {code} 采集失败: {e}")
            errors += 1
            conn.rollback()
            continue

    cur.close()
    conn.close()
    print(f"[分红] ✅ 完成 | 日期: {today} | 新增 {total} 条, 跳过 {skip} 只, 错误 {errors} 只")
    print(f"STAT:records_new={total},records_skip={skip},records_err={errors},dividend_new={total}", flush=True)


if __name__ == "__main__":
    main()
