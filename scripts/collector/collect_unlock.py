#!/usr/bin/env python3
"""
限售解禁数据采集 — 历史解禁 + 未来90天待解禁
数据源: 东财 datacenter RPT_LIFT_STAGE (零鉴权)
用法: python3 collect_unlock.py [--sample | CODE]  (默认: 全市场采集)
"""
import os, sys, time, ssl, random, urllib.request, json
from datetime import datetime, timedelta
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

def collect_unlock_for_code(code, forward_days=90):
    today = datetime.now().strftime("%Y-%m-%d")
    end_date = (datetime.now() + timedelta(days=forward_days)).strftime("%Y-%m-%d")

    all_rows = []

    # 历史解禁
    hist = datacenter_query(
        "RPT_LIFT_STAGE",
        filter_str=f'(SECURITY_CODE="{code}")',
        page_size=15, sort_columns="FREE_DATE", sort_types="-1",
    )
    for row in hist:
        all_rows.append((
            code,
            str(row.get("FREE_DATE", ""))[:10],
            row.get("LIMITED_STOCK_TYPE", "") or "",
            row.get("FREE_SHARES_NUM", 0),
            row.get("FREE_RATIO", 0),
            True,
        ))

    # 未来待解禁
    upcoming = datacenter_query(
        "RPT_LIFT_STAGE",
        filter_str=f'(SECURITY_CODE="{code}")(FREE_DATE>=\'{today}\')(FREE_DATE<=\'{end_date}\')',
        page_size=20, sort_columns="FREE_DATE", sort_types="1",
    )
    for row in upcoming:
        all_rows.append((
            code,
            str(row.get("FREE_DATE", ""))[:10],
            row.get("LIMITED_STOCK_TYPE", "") or "",
            row.get("FREE_SHARES_NUM", 0),
            row.get("FREE_RATIO", 0),
            False,
        ))

    return all_rows

def main():
    print("[解禁] 开始采集...")
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    if '--sample' in sys.argv:
        codes = SAMPLE_CODES
        print(f"[解禁] 样本模式: {len(codes)} 只")
    elif len(sys.argv) > 1:
        codes = [sys.argv[1]]
        print(f"[解禁] 单股模式: {codes[0]}")
    else:
        cur.execute("SELECT code FROM stocks_basic WHERE code IS NOT NULL AND code != '' ORDER BY code")
        codes = [row[0] for row in cur.fetchall()]
        print(f"[解禁] 全量模式: {len(codes)} 只")

    total, skip, errors = 0, 0, 0
    for i, code in enumerate(codes):
        if (i + 1) % 100 == 0:
            print(f"[解禁] 进度: {i+1}/{len(codes)} (新增 {total}, 跳过 {skip}, 错误 {errors})")
        try:
            rows = collect_unlock_for_code(code)
            if not rows:
                skip += 1
                continue
            for r in rows:
                cur.execute("""
                    INSERT INTO restricted_share_unlock (code, free_date, stock_type, shares, ratio, is_history)
                    VALUES (%s, %s, %s, %s, %s, %s)
                    ON CONFLICT DO NOTHING
                """, r)
            conn.commit()
            total += len(rows)
        except Exception as e:
            print(f"[解禁] ⚠️ {code} 采集失败: {e}")
            errors += 1
            conn.rollback()
            continue

    cur.close()
    conn.close()
    print(f"[解禁] 采集完成: 新增 {total} 条, 跳过 {skip} 只, 错误 {errors} 只")
    print(f"STAT:records_new={total},records_skip={skip},records_err={errors},unlock_new={total}", flush=True)


if __name__ == "__main__":
    main()
