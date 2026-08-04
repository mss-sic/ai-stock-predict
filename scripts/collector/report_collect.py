#!/usr/bin/env python3
"""研报数据采集 — 批量拉取全量研报入库，来源: 东财 reportapi"""
import os, sys, json, time, random, psycopg2, requests
from datetime import datetime

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
API = "https://reportapi.eastmoney.com/report/list"
HEADERS = {"User-Agent": UA, "Referer": "https://data.eastmoney.com/"}

def store_report(cur, r):
    cur.execute("""
        INSERT INTO stock_reports (info_code, title, stock_code, stock_name, org_name, org_sname,
            publish_date, rating, rating_change, predict_this_year_eps, predict_this_year_pe,
            predict_next_year_eps, predict_next_year_pe, predict_next_two_year_eps, predict_next_two_year_pe,
            author, researcher, industry_name, pdf_url, attach_size, attach_pages)
        VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s)
        ON CONFLICT (info_code) DO UPDATE SET
            title=EXCLUDED.title, rating=EXCLUDED.rating, rating_change=EXCLUDED.rating_change
    """, (
        r.get('infoCode', ''), r.get('title', ''),
        r.get('stockCode', ''), r.get('stockName', ''),
        r.get('orgName', ''), r.get('orgSName', ''),
        (r.get('publishDate', '') or '')[:10],
        r.get('emRatingName', '') or r.get('sRatingName', '') or '',
        r.get('ratingChange', '') or '',
        float(r.get('predictThisYearEps', 0) or 0),
        float(r.get('predictThisYearPe', 0) or 0),
        float(r.get('predictNextYearEps', 0) or 0),
        float(r.get('predictNextYearPe', 0) or 0),
        float(r.get('predictNextTwoYearEps', 0) or 0),
        float(r.get('predictNextTwoYearPe', 0) or 0),
        json.dumps(r.get('author', []), ensure_ascii=False),
        r.get('researcher', ''),
        r.get('indvInduName', '') or r.get('industryName', ''),
        r.get('encodeUrl', ''),
        int(r.get('attachSize', 0) or 0),
        int(r.get('attachPages', 0) or 0),
    ))

def main():
    begin = sys.argv[1] if len(sys.argv) > 1 else '2026-01-01'
    end = sys.argv[2] if len(sys.argv) > 2 else datetime.now().strftime('%Y-%m-%d')
    target_arg = sys.argv[3] if len(sys.argv) > 3 else ''
    target_codes = set(c.strip() for c in target_arg.split(',') if c.strip()) if target_arg else set()

    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    params = {
        'pageSize': '50', 'beginTime': begin, 'endTime': end,
        'pageNo': '1', 'qType': '0', 'p': '1',
    }
    r = requests.get(API, params=params, headers=HEADERS, timeout=20)
    total_hits = r.json().get('hits', 0)
    if total_hits == 0:
        print("没有研报数据", flush=True)
        return

    total_pages = (total_hits // 50) + 1
    print(f"研报: {total_hits}篇, {total_pages}页 ({begin} ~ {end})", flush=True)

    stored, start_ts = 0, time.time()
    for page in range(1, total_pages + 1):
        params['pageNo'] = str(page)
        params['p'] = str(page)
        r = requests.get(API, params=params, headers=HEADERS, timeout=20)
        items = r.json().get('data', []) or []

        for item in items:
            code = item.get('stockCode', '')
            if not code: continue
            if target_codes and code not in target_codes: continue
            try: store_report(cur, item); stored += 1
            except: pass

        if page % 20 == 0:
            conn.commit()
            print(f"  {page}/{total_pages} | {stored} reports | {time.time()-start_ts:.0f}s", flush=True)
        if len(items) < 50: break
        time.sleep(0.3 + random.random() * 0.2)

    conn.commit(); cur.close(); conn.close()
    print(f"✅ {stored} reports | {time.time()-start_ts:.0f}s", flush=True)

if __name__ == '__main__':
    main()
