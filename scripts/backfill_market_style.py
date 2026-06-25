#!/usr/bin/env python3
"""
market_style_daily 历史数据回填脚本

遍历 market_sentiment 表的每一天，调用 API 计算市场风格并写入 market_style_daily。
用法：
  python3 scripts/backfill_market_style.py                    # 全量回填
  python3 scripts/backfill_market_style.py 2026-06-01         # 从指定日期开始
  python3 scripts/backfill_market_style.py --dry-run          # 预览不执行
"""
import os, sys, json, time, argparse
import psycopg2

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
API_BASE = os.environ.get("API_BASE", "http://localhost:8080/api/v1")

def get_dates(conn, from_date=None):
    """获取 market_sentiment 中所有交易日，按日期排序"""
    sql = "SELECT DISTINCT trade_date::text FROM market_sentiment"
    if from_date:
        sql += f" WHERE trade_date >= '{from_date}'"
    sql += " ORDER BY trade_date"
    cur = conn.cursor()
    cur.execute(sql)
    return [r[0] for r in cur.fetchall()]

def call_api(date):
    """调用市场风格计算 API"""
    import urllib.request
    url = f"{API_BASE}/market/compute-style?date={date}"
    try:
        req = urllib.request.Request(url, method='POST')
        # Try to get token
        login_url = f"{API_BASE}/auth/login"
        login_data = json.dumps({"username": "admin", "password": "admin123"}).encode()
        login_req = urllib.request.Request(login_url, data=login_data,
            headers={'Content-Type': 'application/json'}, method='POST')
        login_resp = urllib.request.urlopen(login_req, timeout=10)
        login_body = json.loads(login_resp.read())
        token = login_body.get('data', {}).get('accessToken', '')
        if token:
            req.add_header('Authorization', f'Bearer {token}')
        
        resp = urllib.request.urlopen(req, timeout=120)
        body = json.loads(resp.read())
        return body.get('code') == 0, body.get('message', '')
    except Exception as e:
        return False, str(e)

def main():
    parser = argparse.ArgumentParser(description='回填 market_style_daily 历史数据')
    parser.add_argument('start_date', nargs='?', default=None, help='起始日期 YYYY-MM-DD')
    parser.add_argument('--dry-run', action='store_true', help='预览不执行')
    args = parser.parse_args()

    conn = psycopg2.connect(PG_DSN)
    dates = get_dates(conn, args.start_date)
    conn.close()

    total = len(dates)
    print(f"共 {total} 个交易日需要回填")

    if args.dry_run:
        print(f"预览: {dates[0]} → {dates[-1]}")
        return

    success = 0
    fail = 0
    for i, date in enumerate(dates):
        ok, msg = call_api(date)
        if ok:
            success += 1
            if (i + 1) % 10 == 0 or i == total - 1:
                print(f"[{i+1}/{total}] {date} OK  (成功{success}, 失败{fail})")
        else:
            fail += 1
            print(f"[{i+1}/{total}] {date} FAIL: {msg}")
        time.sleep(0.5)  # 避免压垮服务

    print(f"\n完成: 成功 {success}, 失败 {fail}, 总计 {total}")

if __name__ == '__main__':
    main()
