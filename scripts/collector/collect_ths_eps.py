#!/usr/bin/env python3
"""
同花顺一致预期EPS采集 — 机构预测年度EPS
数据源: 同花顺 basic.10jqka.com.cn (需UA伪装)
用法: python3 collect_ths_eps.py [--sample | CODE]  (默认: 全市场采集)
注意: 小盘/次新/ST无机构覆盖则无数据，正常
"""
import os, sys, time, ssl, random
import psycopg2
from psycopg2.extras import execute_values
import urllib.request
import pandas as pd
from io import StringIO

# 跨交易所样本池：沪市主板+科创板 + 深市主板+中小板+创业板
SAMPLE_CODES = ['600519','601318','688017','000001','002475','300750','600036','688981']

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE

_ths_last = [0.0]

def fetch_ths_eps(code):
    wait = 1.0 - (time.time() - _ths_last[0])
    if wait > 0:
        time.sleep(wait + random.uniform(0.1, 0.5))
    try:
        url = f"https://basic.10jqka.com.cn/new/{code}/worth.html"
        headers = {
            "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
            "Referer": "https://basic.10jqka.com.cn/",
        }
        req = urllib.request.Request(url, headers=headers)
        with urllib.request.urlopen(req, timeout=15, context=ctx) as resp:
            html = resp.read().decode("gbk", errors="replace")
        dfs = pd.read_html(StringIO(html))
        for df in dfs:
            cols = [str(c) for c in df.columns]
            if any("每股收益" in c or "均值" in c for c in cols):
                return df
        return dfs[0] if dfs else pd.DataFrame()
    except Exception as e:
        return None
    finally:
        _ths_last[0] = time.time()

def parse_eps_df(df, code):
    if df is None or df.empty:
        return []
    rows = []
    for _, row in df.iterrows():
        vals = [v for v in row.values if v is not None and str(v) != "nan"]
        if len(vals) < 3:
            continue
        try:
            # 年份可能是 float (2026.0)，先转 int 再转 string
            year_raw = vals[0]
            if isinstance(year_raw, float) and year_raw == int(year_raw):
                year_str = str(int(year_raw))
            else:
                year_str = str(year_raw).strip()
            if not year_str.isdigit():
                continue
            inst_cnt = int(float(vals[1])) if len(vals) > 1 else 0
            eps_min = float(vals[2]) if len(vals) > 2 else 0
            eps_avg = float(vals[3]) if len(vals) > 3 else 0
            eps_max = float(vals[4]) if len(vals) > 4 else 0
            if eps_avg > 0:
                rows.append((code, year_str, inst_cnt, eps_min, eps_avg, eps_max))
        except (ValueError, TypeError):
            continue
    return rows

def main():
    print("[一致预期] 开始采集...")
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    if '--sample' in sys.argv:
        codes = SAMPLE_CODES
        print(f"[一致预期] 样本模式: {len(codes)} 只")
    elif len(sys.argv) > 1:
        codes = [sys.argv[1]]
        print(f"[一致预期] 单股模式: {codes[0]}")
    else:
        cur.execute("SELECT code FROM stocks_basic WHERE code IS NOT NULL AND code != '' ORDER BY code")
        codes = [row[0] for row in cur.fetchall()]
        print(f"[一致预期] 全量模式: {len(codes)} 只")

    total, skip, errors = 0, 0, 0
    for i, code in enumerate(codes):
        if (i + 1) % 50 == 0:
            print(f"[一致预期] 进度: {i+1}/{len(codes)} (新增 {total}, 跳过 {skip}, 错误 {errors})")
        try:
            df = fetch_ths_eps(code)
            rows = parse_eps_df(df, code)
            if not rows:
                skip += 1
                continue
            sql = """
                INSERT INTO ths_eps_forecast (code, year, institution_count, eps_min, eps_avg, eps_max)
                VALUES %s
                ON CONFLICT (code, year) DO UPDATE SET
                    institution_count = EXCLUDED.institution_count, eps_min = EXCLUDED.eps_min,
                    eps_avg = EXCLUDED.eps_avg, eps_max = EXCLUDED.eps_max
            """
            execute_values(cur, sql, rows, page_size=200)
            conn.commit()
            total += len(rows)
        except Exception as e:
            print(f"[一致预期] ⚠️ {code} 采集失败: {e}")
            errors += 1
            conn.rollback()
            continue

    cur.close()
    conn.close()
    print(f"[一致预期] 采集完成: 新增 {total} 条, 跳过 {skip} 只, 错误 {errors} 只")
    print(f"STAT:records_new={total},records_skip={skip},records_err={errors},ths_eps_new={total}", flush=True)


if __name__ == "__main__":
    main()
