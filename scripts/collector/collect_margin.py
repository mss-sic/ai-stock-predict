#!/usr/bin/env python3
"""
融资融券数据采集 — 全市场每日两融明细
数据源: 东财 datacenter RPTA_WEB_RZRQ_GGMX (零鉴权)
用法: python3 collect_margin.py [--sample | CODE]  (默认: 全市场采集)
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

def collect_margin_for_code(code):
    data = datacenter_query(
        "RPTA_WEB_RZRQ_GGMX",
        filter_str=f'(SCODE="{code}")',
        page_size=5, sort_columns="DATE", sort_types="-1",
    )
    rows = []
    for row in data:
        rows.append((
            code,
            str(row.get("DATE", ""))[:10],
            row.get("RZYE", 0),
            row.get("RZMRE", 0),
            row.get("RZCHE", 0),
            row.get("RQYE", 0),
            row.get("RQMCL", 0),
            row.get("RQCHL", 0),
            row.get("RZRQYE", 0),
        ))
    return rows

def main():
    print("[融资融券] 开始采集...")
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # 获取所有股票代码
    if '--sample' in sys.argv:
        codes = SAMPLE_CODES
        print(f"[融资融券] 样本模式: {len(codes)} 只")
    elif len(sys.argv) > 1:
        codes = [sys.argv[1]]
        print(f"[融资融券] 单股模式: {codes[0]}")
    else:
        cur.execute("SELECT code FROM stocks_basic WHERE code IS NOT NULL AND code != '' ORDER BY code")
        codes = [row[0] for row in cur.fetchall()]
        print(f"[融资融券] 全量模式: {len(codes)} 只")

    total, skip, errors = 0, 0, 0
    for i, code in enumerate(codes):
        if (i + 1) % 100 == 0:
            print(f"[融资融券] 进度: {i+1}/{len(codes)} (新增 {total}, 跳过 {skip}, 错误 {errors})")
        try:
            rows = collect_margin_for_code(code)
            if not rows:
                skip += 1
                continue
            sql = """
                INSERT INTO margin_trading (code, trade_date, rzye, rzmre, rzche, rqye, rqmcl, rqchl, rzrqye)
                VALUES %s
                ON CONFLICT (code, trade_date) DO UPDATE SET
                    rzye = EXCLUDED.rzye, rzmre = EXCLUDED.rzmre, rzche = EXCLUDED.rzche,
                    rqye = EXCLUDED.rqye, rqmcl = EXCLUDED.rqmcl, rqchl = EXCLUDED.rqchl,
                    rzrqye = EXCLUDED.rzrqye
            """
            execute_values(cur, sql, rows, page_size=200)
            conn.commit()
            total += len(rows)
        except Exception as e:
            print(f"[融资融券] ⚠️ {code} 采集失败: {e}")
            errors += 1
            conn.rollback()
            continue

    cur.close()
    conn.close()
    print(f"[融资融券] 采集完成: 新增 {total} 条, 跳过 {skip} 只, 错误 {errors} 只")

    # STAT already tracked in total/skip/errors vars
    print(f"STAT:records_new={total},records_skip={skip},records_err={errors},margin_total_new={total}", flush=True)

if __name__ == "__main__":
    main()
