#!/usr/bin/env python3
"""
同花顺热点强势股采集 — 当日强势股 + 题材归因 reason tags
数据源: 同花顺 stockhot API (零鉴权, 73ms, 约125只/日)
用法: python3 collect_ths_hot.py [YYYY-MM-DD]  (默认今天)
"""
import os, sys, time
import psycopg2
from psycopg2.extras import execute_values
import urllib.request, json

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

def fetch_ths_hot(date_str=None):
    if date_str is None:
        date_str = time.strftime("%Y-%m-%d")

    url = (
        f"http://zx.10jqka.com.cn/event/api/getharden/"
        f"date/{date_str}/orderby/date/orderway/desc/charset/GBK/"
    )
    headers = {
        "User-Agent": (
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
            "Chrome/117.0.0.0 Safari/537.36"
        )
    }
    try:
        req = urllib.request.Request(url, headers=headers)
        with urllib.request.urlopen(req, timeout=10) as resp:
            data = json.loads(resp.read().decode("utf-8"))
    except Exception as e:
        print(f"[同花顺热点] API 请求失败: {e}")
        return []

    if data.get("errocode", 0) != 0:
        print(f"[同花顺热点] API 错误: {data.get('errormsg', '')}")
        return []

    rows = data.get("data") or []
    return rows

def main():
    today = sys.argv[1] if len(sys.argv) > 1 else time.strftime("%Y-%m-%d")
    print(f"[同花顺热点] 采集日期: {today}")

    data = fetch_ths_hot(today)
    if not data:
        print("[同花顺热点] ⚠️ 无数据（非交易日或接口异常）")
        return

    print(f"[同花顺热点] 获取到 {len(data)} 只强势股")

    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    rows = []
    for item in data:
        rows.append((
            item.get("code", ""),
            item.get("name", "") or "",
            today,
            item.get("close", 0) or 0,
            item.get("zhangdie", 0) or 0,
            item.get("zhangfu", 0) or 0,
            item.get("huanshou", 0) or 0,
            item.get("chengjiaoliang", 0) or 0,
            item.get("chengjiaoe", 0) or 0,
            item.get("ddejingliang", 0) or 0,
            item.get("reason", "") or "",
            item.get("market", "") or "",
        ))

    sql = """
        INSERT INTO ths_hot_stocks (code, name, trade_date, close_price, change_amount, change_pct,
            turnover_pct, volume, amount, dde_net_amount, reason_tags, market)
        VALUES %s
        ON CONFLICT DO NOTHING
    """
    execute_values(cur, sql, rows, page_size=200)
    conn.commit()

    cur.close()
    conn.close()
    print(f"[同花顺热点] ✓ 写入 {len(rows)} 条")

    # 打印题材统计
    from collections import Counter
    all_tags = []
    for r in rows:
        tags = r[10] if r[10] else ""
        for t in tags.split("+"):
            t = t.strip()
            if t:
                all_tags.append(t)
    top = Counter(all_tags).most_common(10)
    print(f"[同花顺热点] 今日热门题材 TOP10:")
    for tag, cnt in top:
        print(f"  {tag}: {cnt} 只")

    hot_count = len(rows) if 'rows' in dir() else 0
    print(f"STAT:records_new={hot_count},records_skip=0,records_err=0,ths_hot_count={hot_count}", flush=True)

if __name__ == "__main__":
    main()
