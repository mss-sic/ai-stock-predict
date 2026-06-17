#!/usr/bin/env python3
"""AI 六维评分采集器 — 独立于简介，可单独触发
用法:
  python3 stock_score_collect.py --code 000001          # 单只
  python3 stock_score_collect.py --batch                # 批量(当日榜单)
  python3 stock_score_collect.py --init-prompt          # 初始化提示词
"""
import argparse, json, os, sys, time, urllib.request, ssl
from datetime import date

import psycopg2
from psycopg2.extras import RealDictCursor

os.environ['PYTHONUNBUFFERED'] = '1'

PG_DSN   = os.environ.get("PG_DSN",   "host=localhost dbname=stock_predict user=stock password=stock123")
MYSQL_DSN = os.environ.get("MYSQL_DSN", "host=127.0.0.1 port=3307 user=stock password=stock123 dbname=stock_predict")

DEFAULT_SYSTEM_PROMPT = """你是专业A股量化评分系统。请对给定股票进行六维综合评分。

## 数据来源
基于提供的财务数据、技术指标、新闻资讯进行评分。如果某维度数据不足，标注"数据不足"并给出保守偏低的分数。

## 六维评分标准（每维 0-100 分，60分为中性基准）
1. **fundamentalScore (基本面)**: 财务健康度 — ROE/EPS/利润率/现金流/资产负债率
2. **growthScore (成长性)**: 增长潜力 — 营收增速/利润增速/行业空间/新业务
3. **valuationScore (估值)**: 估值水平 — PE/PB历史分位/行业对比/股息率
4. **capitalScore (资金面)**: 资金流向 — 成交量变化/换手率/股东户数/机构持仓
5. **technicalScore (技术面)**: 技术形态 — 均线趋势/量价关系/支撑阻力位
6. **industryScore (行业)**: 行业景气 — 行业周期/政策/竞争格局

## 综合评分 = 基本面×0.20 + 成长性×0.20 + 估值×0.20 + 资金面×0.15 + 技术面×0.15 + 行业×0.10

## 风险等级
- riskLevel: low / medium-low / medium / medium-high / high
- suggestion: strong_buy / buy / hold / reduce / sell
- riskWarnings: 列出 3-5 条具体风险点
- summary: 50字以内综合评价

## 输出格式（严格 JSON，不要代码块标记）：
{
  "compositeScore": 72.5,
  "fundamentalScore": 70,
  "growthScore": 68,
  "valuationScore": 75,
  "capitalScore": 65,
  "technicalScore": 72,
  "industryScore": 80,
  "riskLevel": "medium",
  "suggestion": "hold",
  "summary": "...",
  "riskWarnings": ["...", "..."]
}"""


def get_pg_conn():
    return psycopg2.connect(PG_DSN, cursor_factory=RealDictCursor)

def get_mysql_conn():
    return psycopg2.connect(MYSQL_DSN, cursor_factory=RealDictCursor)

def fetch_stock_data(code):
    pg = get_pg_conn()
    cur = pg.cursor()
    data = {"code": code}

    cur.execute("SELECT name, industry FROM stocks_basic WHERE code = %s", (code,))
    basic = cur.fetchone()
    if not basic:
        pg.close()
        return None
    data["name"] = basic["name"]
    data["industry"] = basic["industry"]

    cur.execute("""
        SELECT report_date, total_revenue, net_profit, revenue_growth, profit_growth,
               roe, eps, gross_margin, net_margin, debt_ratio
        FROM stock_financials WHERE code = %s ORDER BY report_date DESC LIMIT 3
    """, (code,))
    data["financials"] = [dict(r) for r in cur.fetchall()]

    cur.execute("""
        SELECT trade_date, open, high, low, close, volume, turnover_rate
        FROM stocks_daily_k WHERE code = %s ORDER BY trade_date DESC LIMIT 60
    """, (code,))
    data["klines"] = [dict(r) for r in cur.fetchall()]

    cur.execute("""
        SELECT pe, total_market_cap FROM stocks_daily_indicator
        WHERE code = %s ORDER BY trade_date DESC LIMIT 1
    """, (code,))
    ind = cur.fetchone()
    data["indicator"] = dict(ind) if ind else {}

    cur.execute("""
        SELECT title, publish_date FROM stock_news
        WHERE code = %s ORDER BY publish_date DESC LIMIT 5
    """, (code,))
    data["news"] = [dict(r) for r in cur.fetchall()]

    pg.close()
    return data


def fetch_ai_config():
    mysql = get_mysql_conn()
    cur = mysql.cursor()
    cur.execute("SELECT api_key, base_url, model_name FROM ai_configs WHERE is_active = 1 LIMIT 1")
    cfg = cur.fetchone()
    mysql.close()
    if not cfg or not cfg["api_key"]:
        return None, None

    pg = get_pg_conn()
    cur2 = pg.cursor()
    cur2.execute("""
        SELECT system_prompt, model_name, temperature, max_tokens
        FROM ai_system_configs WHERE scene = 'stock_scoring' LIMIT 1
    """)
    sys_cfg = cur2.fetchone()
    pg.close()

    prompt = sys_cfg["system_prompt"] if sys_cfg and sys_cfg["system_prompt"] else DEFAULT_SYSTEM_PROMPT
    model = (sys_cfg.get("model_name") or cfg["model_name"]) if sys_cfg else cfg["model_name"]
    temp = float(sys_cfg["temperature"]) if sys_cfg and sys_cfg["temperature"] else 0.3
    max_tok = int(sys_cfg["max_tokens"]) if sys_cfg and sys_cfg["max_tokens"] else 2048

    return prompt, {
        "api_key": cfg["api_key"], "base_url": cfg["base_url"],
        "model_name": model, "temperature": temp, "max_tokens": max_tok,
    }


def call_ai(system_prompt, stock_data, ai_cfg):
    user_prompt = json.dumps(stock_data, ensure_ascii=False, default=str, indent=2)
    body = {
        "model": ai_cfg["model_name"],
        "messages": [
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": user_prompt},
        ],
        "temperature": ai_cfg["temperature"],
        "max_tokens": ai_cfg["max_tokens"],
        "response_format": {"type": "json_object"},
    }
    url = f"{ai_cfg['base_url']}/v1/chat/completions"
    req = urllib.request.Request(url, data=json.dumps(body).encode("utf-8"),
        headers={"Content-Type": "application/json", "Authorization": f"Bearer {ai_cfg['api_key']}"})
    ctx = ssl.create_default_context()
    try:
        with urllib.request.urlopen(req, timeout=120, context=ctx) as resp:
            result = json.loads(resp.read().decode())
        return json.loads(result["choices"][0]["message"]["content"])
    except Exception as e:
        print(f"  ❌ AI API 调用失败: {e}", flush=True)
        return None


def save_score(pg_conn, code, result):
    cur = pg_conn.cursor()
    cur.execute("""
        INSERT INTO ai_stock_scores
            (code, composite_score, fundamental_score, growth_score, valuation_score,
             capital_score, technical_score, industry_score, risk_level, suggestion,
             risk_warnings, summary, analyzed_at, created_at)
        VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,NOW())
    """, (
        code,
        result.get("compositeScore"), result.get("fundamentalScore"),
        result.get("growthScore"), result.get("valuationScore"),
        result.get("capitalScore"), result.get("technicalScore"),
        result.get("industryScore"), result.get("riskLevel"),
        result.get("suggestion"),
        json.dumps(result.get("riskWarnings", []), ensure_ascii=False),
        result.get("summary", ""),
        date.today().isoformat(),
    ))
    pg_conn.commit()
    cur.close()


def process_stock(code, dry_run=False):
    name = code
    try:
        stock_data = fetch_stock_data(code)
        if not stock_data:
            print(f"  ⚠ {code}: 数据不存在", flush=True)
            return False
        name = stock_data["name"]

        prompt, ai_cfg = fetch_ai_config()
        if not ai_cfg:
            print(f"  ❌ {code} {name}: AI 配置未设置", flush=True)
            return False

        result = call_ai(prompt, stock_data, ai_cfg)
        if not result or result.get("compositeScore") is None:
            return False

        s = {k: result.get(k) for k in ["compositeScore","fundamentalScore","growthScore","valuationScore","capitalScore","technicalScore","industryScore"]}
        print(f"  ✅ {code} {name} | 综合 {s['compositeScore']} | {result.get('riskLevel','?')} | {result.get('suggestion','?')}", flush=True)

        if dry_run:
            return True

        pg = get_pg_conn()
        save_score(pg, code, result)
        pg.close()
        return True
    except Exception as e:
        print(f"  ❌ {code} {name}: {e}", flush=True)
        return False


def get_board_stocks(pg):
    cur = pg.cursor()
    cur.execute("""
        SELECT DISTINCT d.code FROM algorithm_pick_details d
        JOIN algorithm_picks p ON d.pick_id = p.id WHERE DATE(p.pick_date) = CURRENT_DATE
    """)
    rows = cur.fetchall()
    cur.close()
    return [r["code"] for r in rows] if rows else []


def main():
    parser = argparse.ArgumentParser(description="AI 六维评分采集")
    parser.add_argument("--code", help="单只股票代码")
    parser.add_argument("--batch", action="store_true", help="批量处理当日榜单")
    parser.add_argument("--dry-run", action="store_true", help="预览模式")
    parser.add_argument("--init-prompt", action="store_true", help="初始化提示词")
    args = parser.parse_args()

    if args.init_prompt:
        pg = get_pg_conn()
        cur = pg.cursor()
        cur.execute("""
            INSERT INTO ai_system_configs (scene, name, system_prompt, temperature, max_tokens, enable_search, enable_tools, created_at, updated_at)
            VALUES ('stock_scoring', '六维评分', %s, 0.3, 2048, false, false, NOW(), NOW())
            ON CONFLICT (scene) DO UPDATE SET
                name = EXCLUDED.name, system_prompt = EXCLUDED.system_prompt,
                temperature = EXCLUDED.temperature, max_tokens = EXCLUDED.max_tokens, updated_at = NOW()
        """, (DEFAULT_SYSTEM_PROMPT,))
        pg.commit()
        pg.close()
        print("✅ 六维评分提示词已初始化 (scene=stock_scoring)")
        return

    if args.code:
        ok = process_stock(args.code.strip(), dry_run=args.dry_run)
        print(f"{'✅' if ok else '❌'} {args.code}")
        sys.exit(0 if ok else 1)

    if args.batch:
        pg = get_pg_conn()
        stocks = get_board_stocks(pg)
        if not stocks:
            pg2 = get_pg_conn()
            cur = pg2.cursor()
            cur.execute("""
                SELECT b.code FROM stocks_basic b JOIN stocks_daily_k k ON b.code = k.code
                WHERE b.code NOT LIKE '88%' GROUP BY b.code ORDER BY MAX(k.trade_date) DESC LIMIT 20
            """)
            stocks = [r["code"] for r in cur.fetchall()]
            cur.close()
            pg2.close()
        pg.close()

        print(f"📊 批量评分 {len(stocks)} 只", flush=True)
        success = 0; t0 = time.time()
        for i, code in enumerate(stocks):
            print(f"[{i+1}/{len(stocks)}]", flush=True)
            if process_stock(code, dry_run=args.dry_run): success += 1
            time.sleep(1)
        print(f"\n✅ 完成: {success}/{len(stocks)} | 耗时 {time.time()-t0:.0f}s", flush=True)
        return

    parser.print_help()

if __name__ == "__main__":
    main()
