#!/usr/bin/env python3
"""
巨潮公告采集 — 公告全文检索
数据源: cninfo.com.cn (动态orgId映射, 零鉴权)
用法: python3 collect_cninfo.py [--sample | CODE]  (默认: 全市场采集)
"""
import os, sys, time, json
import psycopg2
from psycopg2.extras import execute_values
import urllib.request

# 跨交易所样本池：沪市主板+科创板 + 深市主板+中小板+创业板
SAMPLE_CODES = ['600519','601318','688017','000001','002475','300750','600036','688981']

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"

# 模块级 orgId 缓存
_ORGID_MAP = {}

def load_orgid_map():
    global _ORGID_MAP
    if _ORGID_MAP:
        return
    try:
        url = "http://www.cninfo.com.cn/new/data/szse_stock.json"
        req = urllib.request.Request(url, headers={"User-Agent": UA})
        with urllib.request.urlopen(req, timeout=15) as resp:
            stock_list = json.loads(resp.read().decode("utf-8")).get("stockList", [])
        _ORGID_MAP = {s["code"]: s["orgId"] for s in stock_list}
        print(f"[巨潮] 加载 orgId 映射表: {len(_ORGID_MAP)} 只")
    except Exception as e:
        print(f"[巨潮] ⚠️ orgId 映射表拉取失败: {e}")

def get_orgid(code):
    load_orgid_map()
    org = _ORGID_MAP.get(code)
    if org:
        return org
    # fallback 硬编码规则
    if code.startswith("6"):
        return f"gssh0{code}"
    elif code.startswith("8") or code.startswith("4"):
        return f"gsbj0{code}"
    return f"gssz0{code}"

def fetch_announcements(code, page_size=30):
    org_id = get_orgid(code)
    url = "https://www.cninfo.com.cn/new/hisAnnouncement/query"
    payload = urllib.parse.urlencode({
        "stock": f"{code},{org_id}",
        "tabName": "fulltext",
        "pageSize": str(page_size),
        "pageNum": "1",
        "column": "",
        "category": "",
        "plate": "",
        "seDate": "",
        "searchkey": "",
        "secid": "",
        "sortName": "",
        "sortType": "",
        "isHLtitle": "true",
    }).encode("utf-8")
    headers = {
        "User-Agent": UA,
        "Content-Type": "application/x-www-form-urlencoded",
        "Referer": "https://www.cninfo.com.cn/new/disclosure",
        "Origin": "https://www.cninfo.com.cn",
    }
    try:
        req = urllib.request.Request(url, data=payload, headers=headers)
        with urllib.request.urlopen(req, timeout=15) as resp:
            d = json.loads(resp.read().decode("utf-8"))
    except Exception as e:
        print(f"[巨潮] ⚠️ {code} API 请求失败: {e}")
        return []

    rows = []
    for item in d.get("announcements", []) or []:
        ts = item.get("announcementTime")
        from datetime import datetime
        if isinstance(ts, (int, float)):
            date_str = datetime.fromtimestamp(ts / 1000).strftime("%Y-%m-%d")
        else:
            date_str = str(ts)[:10] if ts else ""
        rows.append((
            code,
            item.get("announcementTitle", "") or "",
            item.get("announcementTypeName", "") or "",
            date_str,
            f"https://static.cninfo.com.cn/finalpage/{date_str}/{item.get('announcementId', '')}.PDF",
        ))
    return rows

def main():
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    if '--sample' in sys.argv:
        codes = SAMPLE_CODES
        print(f"[巨潮] 样本模式: {len(codes)} 只")
    elif len(sys.argv) > 1:
        codes = [sys.argv[1]]
        print(f"[巨潮] 单股模式: {codes[0]}")
    else:
        cur.execute("SELECT code FROM stocks_basic WHERE code IS NOT NULL AND code != '' ORDER BY code")
        codes = [row[0] for row in cur.fetchall()]
        print(f"[巨潮] 全量模式: {len(codes)} 只")
    print(f"[巨潮] 共 {len(codes)} 只股票")

    total, skip, errors = 0, 0, 0
    for i, code in enumerate(codes):
        if (i + 1) % 50 == 0:
            print(f"[巨潮] 进度: {i+1}/{len(codes)} (新增 {total}, 跳过 {skip}, 错误 {errors})")
        try:
            rows = fetch_announcements(code)
            if not rows:
                skip += 1
                continue
            for r in rows:
                cur.execute("""
                    INSERT INTO cninfo_announcements (code, title, ann_type, ann_date, ann_url)
                    VALUES (%s, %s, %s, %s, %s)
                    ON CONFLICT DO NOTHING
                """, r)
            conn.commit()
            total += len(rows)
        except Exception as e:
            print(f"[巨潮] ⚠️ {code} 采集失败: {e}")
            errors += 1
            conn.rollback()
            continue
        time.sleep(0.3)

    cur.close()
    conn.close()
    print(f"[巨潮] 采集完成: 新增 {total} 条, 跳过 {skip} 只, 错误 {errors} 只")
    print(f"STAT:records_new={total},records_skip={skip},records_err={errors},cninfo_new={total}", flush=True)


if __name__ == "__main__":
    main()
