#!/usr/bin/env python3
"""AI 股票简介 + 六维度评分采集器
用法:
  python3 stock_profile_collect.py --code 000001          # 单只
  python3 stock_profile_collect.py --batch                # 批量(当日榜单)
  python3 stock_profile_collect.py --code 000001 --dry-run # 预览不存储
"""
import argparse, json, os, sys, time, urllib.request, ssl
from datetime import date

import psycopg2
from psycopg2.extras import RealDictCursor

os.environ['PYTHONUNBUFFERED'] = '1'

PG_DSN   = os.environ.get("PG_DSN",   "host=localhost dbname=stock_predict user=stock password=stock123")
MYSQL_DSN = os.environ.get("MYSQL_DSN", "host=127.0.0.1 port=3307 user=stock password=stock123 dbname=stock_predict")

# ── Default system prompt (also stored in DB ai_system_configs scene=stock_profile) ──
DEFAULT_SYSTEM_PROMPT = """你是一位专业、客观、严谨的金融投资分析师，精通A股市场。
你的任务是对给定的股票进行深度分析，并输出严格结构化的 JSON 结果。

## 分析要求
- 基于提供的财务数据、技术指标、新闻资讯进行客观分析
- 评分采用 0-100 分制，60 分为中性基准
- 所有数据引用必须基于实际提供的数据，不得编造
- 如果某些数据缺失，在对应字段注明"数据不足"

## 六维度评分标准
1. **fundamentalScore (基本面)** 0-100: 营收规模、利润质量、ROE/ROA、现金流
2. **growthScore (成长性)** 0-100: 营收增长率、利润增长率、行业增速对比
3. **valuationScore (估值)** 0-100: PE/PB分位、股息率、相比行业均值
4. **capitalScore (资金面)** 0-100: 北向资金、融资融券、股东户数变化、机构持仓
5. **technicalScore (技术面)** 0-100: 均线趋势、量价关系、支撑阻力位
6. **industryScore (行业)** 0-100: 行业景气度、政策支持、竞争格局、产业链位置

## 风险提示要求
- riskLevel: low / medium / high
- suggestion: buy / hold / reduce / avoid
- riskWarnings: 列出 2-5 条具体风险点

## 公司简介 Markdown 要求
profileMarkdown 字段输出一份精美的结构化 Markdown 简介，要求：
- 使用 ## 标题分级
- 核心数据用 **加粗** 突出
- 财务数据使用 | 表格 |
- 关键观点用 > 引用块
- 分为以下小节：
  1. ## 🏢 核心特征 — 一句话概括公司定位和盈利模式
  2. ## 💼 主营业务 — 业务结构和护城河分析
  3. ## 📊 最新财报 — 财务数据表格 + 解读
  4. ## 🚀 成长驱动 — 短期和长期增长因素
  5. ## ⚠️ 风险提示 — 关键风险清单
  6. ## 🔮 未来展望 — 前瞻布局和市场潜力

## 输出格式
严格按照以下 JSON 格式输出，不要包含任何其他文字：
{
  "profileMarkdown": "完整的Markdown格式简介",
  "compositeScore": 72.5,
  "fundamentalScore": 70,
  "growthScore": 68,
  "valuationScore": 75,
  "capitalScore": 65,
  "technicalScore": 72,
  "industryScore": 80,
  "riskLevel": "medium",
  "suggestion": "hold",
  "riskWarnings": ["风险1", "风险2"],
  "summary": "50字以内的综合评价摘要"
}"""

# ═══════════════════════════════════════════════
#  Data gathering helpers
# ═══════════════════════════════════════════════

def get_pg_conn():
    return psycopg2.connect(PG_DSN, cursor_factory=RealDictCursor)

def get_mysql_conn():
    return psycopg2.connect(MYSQL_DSN, cursor_factory=RealDictCursor)

def fetch_stock_data(code):
    """Gather all relevant data for AI analysis"""
    pg = get_pg_conn()
    cur = pg.cursor()

    data = {"code": code}

    # 1. Stock basic info
    cur.execute("SELECT name, industry FROM stocks_basic WHERE code = %s", (code,))
    basic = cur.fetchone()
    if not basic:
        pg.close()
        return None
    data["name"] = basic["name"]
    data["industry"] = basic["industry"]

    # 2. Latest financial data
    cur.execute("""
        SELECT report_date, report_type, total_revenue, net_profit,
               revenue_growth, profit_growth, roe, eps, bps,
               gross_margin, net_margin, debt_ratio
        FROM stock_financials WHERE code = %s
        ORDER BY report_date DESC LIMIT 4
    """, (code,))
    financials = cur.fetchall()
    data["financials"] = [dict(r) for r in financials]

    # 3. Technical data (last 60 days)
    cur.execute("""
        SELECT trade_date, open, high, low, close, volume, turnover_rate
        FROM stocks_daily_k WHERE code = %s
        ORDER BY trade_date DESC LIMIT 60
    """, (code,))
    klines = cur.fetchall()
    data["klines"] = [dict(r) for r in klines]

    # 4. Latest indicator
    cur.execute("""
        SELECT pe, total_market_cap, circulating_market_cap
        FROM stocks_daily_indicator WHERE code = %s
        ORDER BY trade_date DESC LIMIT 1
    """, (code,))
    ind = cur.fetchone()
    data["indicator"] = dict(ind) if ind else {}

    # 5. News (last 10)
    cur.execute("""
        SELECT title, publish_date, news_type
        FROM stock_news WHERE code = %s
        ORDER BY publish_date DESC LIMIT 10
    """, (code,))
    news = cur.fetchall()
    data["news"] = [dict(r) for r in news]

    # 6. Shareholders
    cur.execute("""
        SELECT report_date, total_shareholders, avg_holdings, institution_ratio
        FROM stock_shareholders WHERE code = %s
        ORDER BY report_date DESC LIMIT 3
    """, (code,))
    shareholders = cur.fetchall()
    data["shareholders"] = [dict(r) for r in shareholders]

    # 7. Concept tags
    cur.execute("SELECT concept_tags FROM stocks_basic WHERE code = %s", (code,))
    tags = cur.fetchone()
    data["conceptTags"] = tags["concept_tags"] if tags else []

    pg.close()
    return data


def fetch_ai_config():
    """Get user's AI config from MySQL and system prompt from PG"""
    mysql = get_mysql_conn()
    cur = mysql.cursor()

    # Get first active API key
    cur.execute("SELECT api_key, base_url, model_name FROM ai_configs WHERE is_active = 1 LIMIT 1")
    cfg = cur.fetchone()
    mysql.close()

    if not cfg or not cfg["api_key"]:
        return None, None

    # Get system prompt from PG
    pg = get_pg_conn()
    cur2 = pg.cursor()
    cur2.execute("SELECT system_prompt, model_name, temperature, max_tokens FROM ai_system_configs WHERE scene = 'stock_profile' LIMIT 1")
    sys_cfg = cur2.fetchone()
    pg.close()

    prompt = sys_cfg["system_prompt"] if sys_cfg and sys_cfg["system_prompt"] else DEFAULT_SYSTEM_PROMPT
    model = (sys_cfg.get("model_name") or cfg["model_name"]) if sys_cfg else cfg["model_name"]
    temperature = float(sys_cfg["temperature"]) if sys_cfg and sys_cfg["temperature"] else 0.3
    max_tokens = int(sys_cfg["max_tokens"]) if sys_cfg and sys_cfg["max_tokens"] else 4096

    ai_cfg = {
        "api_key": cfg["api_key"],
        "base_url": cfg["base_url"],
        "model_name": model,
        "temperature": temperature,
        "max_tokens": max_tokens,
    }
    return prompt, ai_cfg


def call_ai(system_prompt, stock_data, ai_cfg):
    """Call AI API to generate profile and scores"""
    # Build user prompt with stock data
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
    req = urllib.request.Request(
        url,
        data=json.dumps(body).encode("utf-8"),
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {ai_cfg['api_key']}",
        },
    )
    ctx = ssl.create_default_context()
    try:
        with urllib.request.urlopen(req, timeout=120, context=ctx) as resp:
            result = json.loads(resp.read().decode())
        content = result["choices"][0]["message"]["content"]
        # Parse the JSON response
        return json.loads(content)
    except Exception as e:
        print(f"  ❌ AI API 调用失败: {e}", flush=True)
        return None


def save_profile(pg_conn, code, profile_data):
    """Save profile and scores to database"""
    cur = pg_conn.cursor()
    now = date.today().isoformat()

    # Save profile markdown
    scores_json = json.dumps({
        "compositeScore": profile_data.get("compositeScore"),
        "fundamentalScore": profile_data.get("fundamentalScore"),
        "growthScore": profile_data.get("growthScore"),
        "valuationScore": profile_data.get("valuationScore"),
        "capitalScore": profile_data.get("capitalScore"),
        "technicalScore": profile_data.get("technicalScore"),
        "industryScore": profile_data.get("industryScore"),
        "riskLevel": profile_data.get("riskLevel"),
        "suggestion": profile_data.get("suggestion"),
        "riskWarnings": profile_data.get("riskWarnings", []),
        "summary": profile_data.get("summary", ""),
    }, ensure_ascii=False)

    cur.execute("""
        INSERT INTO stock_profiles (code, profile_markdown, scores_json, analyzed_at, created_at, updated_at)
        VALUES (%s, %s, %s, %s, NOW(), NOW())
        ON CONFLICT (code) DO UPDATE SET
            profile_markdown = EXCLUDED.profile_markdown,
            scores_json = EXCLUDED.scores_json,
            analyzed_at = EXCLUDED.analyzed_at,
            updated_at = NOW()
    """, (code, profile_data["profileMarkdown"], scores_json, now))

    # Also save to ai_stock_scores
    cur.execute("""
        INSERT INTO ai_stock_scores
            (code, composite_score, fundamental_score, growth_score, valuation_score,
             capital_score, technical_score, industry_score, risk_level, suggestion, risk_warnings, summary, analyzed_at, created_at)
        VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,NOW())
        ON CONFLICT (code, analyzed_at) DO NOTHING
    """, (
        code,
        profile_data.get("compositeScore"),
        profile_data.get("fundamentalScore"),
        profile_data.get("growthScore"),
        profile_data.get("valuationScore"),
        profile_data.get("capitalScore"),
        profile_data.get("technicalScore"),
        profile_data.get("industryScore"),
        profile_data.get("riskLevel"),
        profile_data.get("suggestion"),
        json.dumps(profile_data.get("riskWarnings", []), ensure_ascii=False),
        profile_data.get("summary", ""),
        now,
    ))

    pg_conn.commit()
    cur.close()
    return True


def process_stock(code, dry_run=False):
    """Process a single stock: fetch data → AI analyze → save"""
    name = code
    try:
        stock_data = fetch_stock_data(code)
        if not stock_data:
            print(f"  ⚠ {code}: 股票基础数据不存在", flush=True)
            return False
        name = stock_data["name"]
        print(f"  📊 {code} {name} | 财务{len(stock_data.get('financials',[]))}条 | K线{len(stock_data.get('klines',[]))}条", flush=True)

        prompt, ai_cfg = fetch_ai_config()
        if not ai_cfg:
            print(f"  ❌ {code} {name}: AI 配置未设置，请在系统设置中配置 API Key", flush=True)
            return False

        result = call_ai(prompt, stock_data, ai_cfg)
        if not result:
            return False

        scores = {k: result.get(k) for k in ["compositeScore","fundamentalScore","growthScore","valuationScore","capitalScore","technicalScore","industryScore"]}
        print(f"  ✅ {code} {name} | 综合 {result.get('compositeScore','?')} | {result.get('riskLevel','?')} | {result.get('suggestion','?')}", flush=True)

        if dry_run:
            print(f"  📝 Profile preview:\n{result.get('profileMarkdown','')[:300]}...", flush=True)
            return True

        pg = get_pg_conn()
        ok = save_profile(pg, code, result)
        pg.close()
        return ok
    except Exception as e:
        print(f"  ❌ {code} {name}: {e}", flush=True)
        return False


def get_board_stocks(pg):
    """Get stocks from today's algorithm pick"""
    cur = pg.cursor()
    cur.execute("""
        SELECT DISTINCT d.code
        FROM algorithm_pick_details d
        JOIN algorithm_picks p ON d.pick_id = p.id
        WHERE DATE(p.pick_date) = CURRENT_DATE
        ORDER BY d.code
    """)
    rows = cur.fetchall()
    cur.close()
    return [r["code"] for r in rows] if rows else []


def main():
    parser = argparse.ArgumentParser(description="AI 股票简介 + 六维度评分采集")
    parser.add_argument("--code", help="单只股票代码")
    parser.add_argument("--batch", action="store_true", help="批量处理当日榜单股票")
    parser.add_argument("--dry-run", action="store_true", help="预览模式，不写入数据库")
    parser.add_argument("--init-prompt", action="store_true", help="初始化提示词到数据库")
    args = parser.parse_args()

    # ── Init prompt ──
    if args.init_prompt:
        pg = get_pg_conn()
        cur = pg.cursor()
        cur.execute("""
            INSERT INTO ai_system_configs (scene, name, system_prompt, temperature, max_tokens, enable_search, enable_tools, created_at, updated_at)
            VALUES ('stock_profile', '股票简介+评分', %s, 0.3, 4096, false, false, NOW(), NOW())
            ON CONFLICT (scene) DO UPDATE SET
                name = EXCLUDED.name, system_prompt = EXCLUDED.system_prompt,
                temperature = EXCLUDED.temperature, max_tokens = EXCLUDED.max_tokens,
                updated_at = NOW()
        """, (DEFAULT_SYSTEM_PROMPT,))
        pg.commit()
        pg.close()
        print("✅ 提示词已初始化到 ai_system_configs (scene=stock_profile)")
        return

    # ── Single stock ──
    if args.code:
        code = args.code.strip()
        ok = process_stock(code, dry_run=args.dry_run)
        if ok:
            print(f"✅ {code} 完成")
        else:
            print(f"❌ {code} 失败")
            sys.exit(1)
        return

    # ── Batch mode ──
    if args.batch:
        pg = get_pg_conn()
        stocks = get_board_stocks(pg)
        pg.close()

        if not stocks:
            print("⚠ 当日无榜单数据，尝试处理前20只有K线的股票...")
            pg = get_pg_conn()
            cur = pg.cursor()
            cur.execute("""
                SELECT b.code FROM stocks_basic b
                JOIN stocks_daily_k k ON b.code = k.code
                WHERE b.code NOT LIKE '88%'
                GROUP BY b.code ORDER BY MAX(k.trade_date) DESC LIMIT 20
            """)
            stocks = [r["code"] for r in cur.fetchall()]
            cur.close()
            pg.close()

        print(f"📊 批量处理 {len(stocks)} 只股票", flush=True)
        success = 0
        t0 = time.time()
        for i, code in enumerate(stocks):
            print(f"[{i+1}/{len(stocks)}]", flush=True)
            if process_stock(code, dry_run=args.dry_run):
                success += 1
            time.sleep(1)  # API rate limit
        elapsed = time.time() - t0
        print(f"\n✅ 完成: {success}/{len(stocks)} | 耗时 {elapsed:.0f}s", flush=True)
        return

    parser.print_help()


if __name__ == "__main__":
    main()
