#!/usr/bin/env python3
"""资讯数据采集 — 个股新闻+公告，来源: 东财+巨潮"""
import os, sys, json, re, time, random, psycopg2, requests

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"

# ── 东财防封 ──
EM_SESSION = requests.Session()
EM_SESSION.headers.update({"User-Agent": UA})
EM_MIN_INTERVAL = 1.0
_em_last_call = [0.0]

def em_get(url, params=None, headers=None, timeout=15, **kwargs):
    wait = EM_MIN_INTERVAL - (time.time() - _em_last_call[0])
    if wait > 0:
        time.sleep(wait + random.uniform(0.1, 0.5))
    try:
        return EM_SESSION.get(url, params=params, headers=headers, timeout=timeout, **kwargs)
    finally:
        _em_last_call[0] = time.time()

# ── 巨潮 orgId 缓存 ──
_CNINFO_ORGID_MAP = {}

def _cninfo_orgid(code):
    global _CNINFO_ORGID_MAP
    if not _CNINFO_ORGID_MAP:
        try:
            r = requests.get("http://www.cninfo.com.cn/new/data/szse_stock.json",
                             headers={"User-Agent": UA}, timeout=15)
            _CNINFO_ORGID_MAP = {s["code"]: s["orgId"]
                                 for s in r.json().get("stockList", [])}
        except Exception as e:
            print(f"[WARN] 巨潮 orgId 映射表拉取失败: {e}", flush=True)
    org = _CNINFO_ORGID_MAP.get(code)
    if org:
        return org
    if code.startswith("6"):
        return f"gssh0{code}"
    elif code.startswith("8") or code.startswith("4"):
        return f"gsbj0{code}"
    return f"gssz0{code}"

def fetch_news_eastmoney(code, pages=1):
    """Fetch stock news from eastmoney (JSONP API)"""
    news = []
    try:
        cb = "jQuery_news"
        inner_params = json.dumps({
            "uid": "", "keyword": code, "type": ["cmsArticleWebOld"],
            "client": "web", "clientType": "web", "clientVersion": "curr",
            "param": {"cmsArticleWebOld": {
                "searchScope": "default", "sort": "default",
                "pageIndex": 1, "pageSize": 20, "preTag": "", "postTag": ""
            }},
        }, separators=(',', ':'))
        params = {"cb": cb, "param": inner_params}
        headers = {"Referer": "https://so.eastmoney.com/"}
        r = em_get("https://search-api-web.eastmoney.com/search/jsonp",
                   params=params, headers=headers, timeout=15)
        text = r.text
        json_str = text[text.index("(") + 1 : text.rindex(")")]
        d = json.loads(json_str)
        articles = d.get("result", {}).get("cmsArticleWebOld", []) or []
        for a in articles:
            news.append({
                'title': re.sub(r'<[^>]+>', '', a.get("title", "")),
                'summary': re.sub(r'<[^>]+>', '', a.get("content", ""))[:500],
                'source': 'eastmoney',
                'newsType': 'news',
                'url': a.get("url", ""),
                'publishDate': a.get("date", "")[:10],
            })
    except Exception as e:
        pass
    return news

def fetch_announcements_cninfo(code):
    """Fetch announcements from巨潮 via POST"""
    announcements = []
    try:
        org_id = _cninfo_orgid(code)
        payload = {
            "stock": f"{code},{org_id}",
            "tabName": "fulltext", "pageSize": "10", "pageNum": "1",
            "column": "", "category": "", "plate": "", "seDate": "",
            "searchkey": "", "secid": "", "sortName": "", "sortType": "", "isHLtitle": "true",
        }
        headers = {
            "Content-Type": "application/x-www-form-urlencoded",
            "Referer": "https://www.cninfo.com.cn/new/disclosure",
            "Origin": "https://www.cninfo.com.cn",
        }
        r = requests.post("https://www.cninfo.com.cn/new/hisAnnouncement/query",
                          data=payload, headers=headers, timeout=15)
        d = r.json()
        from datetime import datetime
        for item in d.get("announcements", []) or []:
            ts = item.get("announcementTime")
            if isinstance(ts, (int, float)):
                pub_date = datetime.fromtimestamp(ts / 1000).strftime("%Y-%m-%d")
            else:
                pub_date = str(ts)[:10] if ts else ""
            announcements.append({
                'title': item.get("announcementTitle", ""),
                'summary': '',
                'source': 'cninfo',
                'newsType': 'announcement',
                'url': f'https://www.cninfo.com.cn/new/disclosure/detail?annoId={item.get("announcementId","")}',
                'publishDate': pub_date,
            })
    except Exception as e:
        pass
    return announcements

def main():
    codes_arg = sys.argv[1] if len(sys.argv) > 1 else ''
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()
    
    if codes_arg:
        codes = [c.strip() for c in codes_arg.split(',') if c.strip()]
    else:
        # 增量策略:
        # 1. 优先拉取从未采集过资讯的股票 (LIMIT 50)
        # 2. 再拉取最新资讯超过2天的股票 (LIMIT 50)
        cur.execute("""
            SELECT b.code FROM stocks_basic b
            LEFT JOIN stock_news n ON b.code = n.code
            WHERE n.code IS NULL
            ORDER BY b.code
            LIMIT 50
        """)
        codes_new = [r[0] for r in cur.fetchall()]

        cur.execute("""
            SELECT b.code FROM stocks_basic b
            INNER JOIN (
                SELECT code, MAX(publish_date) as latest FROM stock_news GROUP BY code
            ) n ON b.code = n.code
            WHERE n.latest::date < CURRENT_DATE - INTERVAL '2 days'
            ORDER BY n.latest ASC
            LIMIT 50
        """)
        codes_stale = [r[0] for r in cur.fetchall()]

        # Deduplicate and merge
        codes = list(dict.fromkeys(codes_new + codes_stale))
    
    if not codes:
        print("资讯数据已是最新", flush=True)
        return
    
    print(f"采集资讯数据: {len(codes)} 只", flush=True)
    total = 0
    start = time.time()
    
    # Fetch orgId map first
    print("加载巨潮 orgId 映射...", flush=True)
    _cninfo_orgid("000001")
    
    for i, code in enumerate(codes):
        items = fetch_news_eastmoney(code)
        time.sleep(0.3)
        items += fetch_announcements_cninfo(code)
        
        for item in items:
            if not item['title'] or not item['publishDate']:
                continue
            try:
                cur.execute("""
                    INSERT INTO stock_news (code, title, summary, source, news_type, url, publish_date)
                    VALUES (%s,%s,%s,%s,%s,%s,%s)
                    ON CONFLICT DO NOTHING
                """, (code, item['title'][:500], item['summary'], item['source'],
                      item['newsType'], item['url'][:500], item['publishDate']))
                total += 1
            except:
                pass
        
        if (i + 1) % 20 == 0:
            conn.commit()
            elapsed = time.time() - start
            print(f"  {i+1}/{len(codes)} | {total} items | {elapsed:.0f}s", flush=True)
        time.sleep(0.2)
    
    conn.commit()
    cur.close()
    conn.close()
    print(f"✅ 资讯数据: {total} items | {time.time()-start:.0f}s")

if __name__ == "__main__":
    main()
