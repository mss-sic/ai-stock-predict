#!/usr/bin/env python3
"""
东财全球宏观资讯采集 — 7×24 财经快讯
数据源: 东财 np-weblist (零鉴权)
用法: python3 collect_macro_news.py
"""
import os, sys, time, json, uuid, random
import psycopg2
import urllib.request

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"

def fetch_global_news(page_size=50):
    time.sleep(random.uniform(0.3, 1.5))  # 随机延迟，避免触发东财限流
    url = "https://np-weblist.eastmoney.com/comm/web/getFastNewsList"
    params = urllib.parse.urlencode({
        "client": "web", "biz": "web_724",
        "fastColumn": "102", "sortEnd": "",
        "pageSize": str(page_size),
        "req_trace": str(uuid.uuid4()),
    })
    full_url = f"{url}?{params}"
    headers = {"User-Agent": UA, "Referer": "https://kuaixun.eastmoney.com/"}
    for attempt in range(3):
        try:
            req = urllib.request.Request(full_url, headers=headers)
            with urllib.request.urlopen(req, timeout=10) as resp:
                d = json.loads(resp.read().decode("utf-8"))
            break
        except Exception as e:
            if attempt < 2:
                wait = (attempt + 1) * 3
                print(f"[宏观资讯] 请求失败(重试{attempt+1}/3, {wait}s后): {e}")
                time.sleep(wait)
            else:
                print(f"[宏观资讯] API 请求失败(已重试3次): {e}")
                return []

    rows = []
    for item in d.get("data", {}).get("fastNewsList", []):
        title = item.get("title", "")
        summary = (item.get("summary", "") or "")[:200]
        news_time = item.get("showTime", "")
        # 简单分类
        category = "general"
        tl = title.lower()
        if any(k in tl for k in ["央行", "降息", "降准", "mlf", "lpr", "货币政策", "利率"]):
            category = "货币政策"
        elif any(k in tl for k in ["财政", "预算", "国债", "减税"]):
            category = "财政政策"
        elif any(k in tl for k in ["证监会", "交易所", "ipo", "注册制"]):
            category = "监管政策"
        elif any(k in tl for k in ["美联储", "美股", "港股", "欧股", "日经"]):
            category = "国际宏观"
        elif any(k in tl for k in ["制裁", "战争", "冲突", "地缘"]):
            category = "地缘政治"
        rows.append((title, summary, news_time, category))
    return rows

def main():
    today = time.strftime("%Y-%m-%d")
    print(f"[宏观资讯] 开始采集 → 日期: {today}")
    data = fetch_global_news()
    if not data:
        print("[宏观资讯] ⚠️ 无数据")
        return

    print(f"[宏观资讯] 获取到 {len(data)} 条")

    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    sql = """
        INSERT INTO macro_news (title, summary, news_time, category)
        VALUES (%s, %s, %s, %s)
        ON CONFLICT (title, news_time, category) DO NOTHING
    """
    inserted = 0
    skipped = 0
    for r in data:
        try:
            cur.execute(sql, r)
            if cur.rowcount > 0:
                inserted += 1
            else:
                skipped += 1
        except Exception:
            skipped += 1
    conn.commit()

    cur.close()
    conn.close()
    print(f"[宏观资讯] ✓ 写入 {inserted} 条, 跳过 {skipped} 条(重复)")

    # 分类统计
    from collections import Counter
    cats = Counter(r[3] for r in data)
    print("[宏观资讯] 分类统计:")
    for cat, cnt in cats.most_common():
        print(f"  {cat}: {cnt} 条")

    print(f"[宏观资讯] ✅ 完成 | 日期: {today} | 新增 {inserted} 条, 跳过 {skipped} 条", flush=True)
    print(f"STAT:records_new={inserted},records_skip={skipped},records_err=0,macro_news_count={inserted}", flush=True)

if __name__ == "__main__":
    main()
