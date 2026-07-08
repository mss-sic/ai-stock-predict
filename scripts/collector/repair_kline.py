#!/usr/bin/env python3
"""
修复单只股票历史数据 — 删除后重新采集（前复权）
- K线: 腾讯财经 API (前复权 qfq)
- 换手率: 从腾讯实时行情获取流通市值÷股价=流通股本，再计算换手率
- 成交额: close × volume
- PE/PB/PS: 基于财报 + K线实时计算
"""
import os, sys, time, json, ssl, urllib.request
from datetime import date
import psycopg2
from psycopg2.extras import execute_values

os.environ['PYTHONUNBUFFERED'] = '1'

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

def log(msg):
    print(msg, flush=True)

def get_circulating_shares(code, prefix, price):
    """从腾讯实时行情获取流通股本 = 流通市值(亿) / 股价"""
    try:
        url = f"http://qt.gtimg.cn/q={prefix}{code}"
        req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
        ctx = ssl.create_default_context()
        with urllib.request.urlopen(req, timeout=10, context=ctx) as resp:
            text = resp.read().decode('gbk', errors='replace')
        for line in text.strip().split('\n'):
            if '="' not in line:
                continue
            fields = line.split('="')[1].rstrip('";').split('~')
            if len(fields) > 45:
                cmcap_yi = float(fields[45]) if fields[45] else 0  # 流通市值(亿)
                if cmcap_yi > 0 and price > 0:
                    # 流通市值(亿) × 100000000 / 股价 = 流通股本(股)
                    return int(cmcap_yi * 1e8 / price)
    except:
        pass
    return 0

def main():
    if len(sys.argv) < 2:
        log(json.dumps({"error": "请提供股票代码"}))
        sys.exit(1)

    code = sys.argv[1]
    if code.startswith("92"):
        prefix = "nq"
    elif code.startswith(("6", "9")):
        prefix = "sh"
    elif code.startswith("8"):
        prefix = "nq"
    else:
        prefix = "sz"

    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # ── Step 0: 获取最新的收盘价用于计算流通股本 ──
    cur.execute("SELECT close FROM stocks_daily_k WHERE code = %s ORDER BY trade_date DESC LIMIT 1", (code,))
    kr = cur.fetchone()
    current_close = float(kr[0]) if kr and kr[0] else 0

    # ── Step 1: 从实时行情获取流通股本（更准确）──
    # 如果实时API获取失败，退而求其次用 stocks_basic 的 total_shares
    cur.execute("SELECT total_shares FROM stocks_basic WHERE code = %s", (code,))
    row = cur.fetchone()
    db_total_shares = int(row[0]) if row and row[0] else 0

    circ_shares = get_circulating_shares(code, prefix, current_close)
    if circ_shares > 0:
        log(f"[{code}] 流通股本(实时): {circ_shares:,} 股 | stocks_basic: {db_total_shares:,} 股")
    elif db_total_shares > 0:
        circ_shares = db_total_shares
        log(f"[{code}] 实时API获取失败，使用 stocks_basic: {circ_shares:,} 股")
    else:
        circ_shares = 0
        log(f"[{code}] 无法获取股本，换手率将为0")

    # ── Step 2: 删除现有数据 ──
    cur.execute("SELECT COUNT(*) FROM stocks_daily_k WHERE code = %s", (code,))
    old_k_count = cur.fetchone()[0]
    cur.execute("SELECT COUNT(*) FROM stocks_daily_indicator WHERE code = %s", (code,))
    old_i_count = cur.fetchone()[0]
    log(f"[{code}] 现有K线: {old_k_count} 条 | 指标: {old_i_count} 条")

    cur.execute("DELETE FROM stocks_daily_k WHERE code = %s", (code,))
    cur.execute("DELETE FROM stocks_daily_indicator WHERE code = %s", (code,))
    conn.commit()
    log(f"[{code}] 已清除历史数据")

    # ── Step 3: 从腾讯API拉取全量前复权K线 ──
    url = f"https://ifzq.gtimg.cn/appstock/app/fqkline/get?param={prefix}{code},day,,,1100,qfq"
    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
    ctx = ssl.create_default_context()
    try:
        with urllib.request.urlopen(req, timeout=30, context=ctx) as resp:
            data = json.loads(resp.read().decode())
    except Exception as e:
        log(json.dumps({"error": f"API请求失败: {e}"}))
        cur.close(); conn.close()
        sys.exit(1)

    stock_data = data.get("data", {})
    klines = []
    for pfx in [prefix, "sh", "sz", "nq"]:
        sd = stock_data.get(f"{pfx}{code}", {})
        klines = sd.get("qfqday", []) or sd.get("day", [])
        if klines:
            break

    if not klines:
        log(json.dumps({"error": "API返回空数据"}))
        cur.close(); conn.close()
        sys.exit(1)

    log(f"[{code}] API返回: {len(klines)} 条K线")

    # ── Step 4: 解析并计算成交量/成交额/换手率 ──
    rows = []
    for row in klines:
        if len(row) < 6:
            continue
        trade_date = row[0]
        open_p = float(row[1])
        high_p = float(row[3])
        low_p = float(row[4])
        close_p = float(row[2])
        vol_shou = float(row[5])
        volume = int(vol_shou) if code.startswith("688") else int(vol_shou * 100)
        amount = close_p * volume

        # 换手率 = 成交量(股) / 流通股本 × 100
        turnover = 0.0
        if circ_shares > 0 and volume > 0:
            turnover = round(volume / circ_shares, 6)

        rows.append((
            code, trade_date, open_p, high_p, low_p, close_p,
            volume, amount, turnover,
        ))

    execute_values(cur, """
        INSERT INTO stocks_daily_k (code, trade_date, open, high, low, close, volume, amount, turnover_rate, updated_at)
        VALUES %s
        ON CONFLICT (code, trade_date) DO UPDATE SET
            open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low,
            close = EXCLUDED.close, volume = EXCLUDED.volume, amount = EXCLUDED.amount,
            turnover_rate = EXCLUDED.turnover_rate, updated_at = NOW()
    """, rows)
    conn.commit()
    log(f"[{code}] 入库: {len(rows)} 条 (含成交量+成交额+换手率)")

    # ── Step 5: 用腾讯实时行情补充最新交易日的准确换手率/PE ──
    try:
        quote_url = f"http://qt.gtimg.cn/q={prefix}{code}"
        req2 = urllib.request.Request(quote_url, headers={"User-Agent": "Mozilla/5.0"})
        with urllib.request.urlopen(req2, timeout=10, context=ctx) as resp:
            qtext = resp.read().decode('gbk', errors='replace')
        for line in qtext.strip().split('\n'):
            if '="' not in line:
                continue
            fields = line.split('="')[1].rstrip('";').split('~')
            if len(fields) > 45:
                live_turnover = float(fields[38]) if fields[38] else 0
                live_pe = float(fields[39]) if fields[39] else 0
                live_mcap = float(fields[44]) if fields[44] else 0
                live_cmcap = float(fields[45]) if fields[45] else 0

                # 更新最新交易日的换手率为实时准确值
                if live_turnover > 0:
                    cur.execute("""
                        UPDATE stocks_daily_k SET turnover_rate = %s / 100.0
                        WHERE code = %s AND trade_date = (
                            SELECT MAX(trade_date) FROM stocks_daily_k WHERE code = %s
                        )
                    """, (live_turnover, code, code))
                    conn.commit()
                    log(f"[{code}] 实时换手率: {live_turnover}% PE: {live_pe}")

                # 更新或插入今日指标
                today = date.today().isoformat()
                cur.execute("""
                    INSERT INTO stocks_daily_indicator (code, trade_date, pe, pb, ps, total_market_cap, circulating_market_cap)
                    VALUES (%s, %s, %s, 0, 0, %s, %s)
                    ON CONFLICT (code, trade_date) DO UPDATE SET
                        pe = EXCLUDED.pe,
                        total_market_cap = EXCLUDED.total_market_cap,
                        circulating_market_cap = EXCLUDED.circulating_market_cap
                """, (code, today, live_pe, live_mcap, live_cmcap))
                conn.commit()
    except Exception as e:
        log(f"[{code}] 实时行情获取失败(可忽略): {e}")

    # ── Step 6: 批量重新计算历史 PE/PB/PS 指标 ──
    log(f"[{code}] 重新计算历史 PE/PB/PS 指标...")
    cur.execute("""
        INSERT INTO stocks_daily_indicator (code, trade_date, pe, pb, ps, total_market_cap, circulating_market_cap)
        SELECT
            k.code, k.trade_date,
            CASE WHEN fin.eps > 0 THEN ROUND((k.close / fin.eps)::numeric, 2) ELSE 0 END as pe,
            CASE WHEN fin.bps > 0 THEN ROUND((k.close / fin.bps)::numeric, 2) ELSE 0 END as pb,
            CASE WHEN fin.revenue_per_share > 0 THEN ROUND((k.close / fin.revenue_per_share)::numeric, 2) ELSE 0 END as ps,
            CASE WHEN fin.eps > 0 AND fin.net_profit > 0
                 THEN ROUND((k.close * (fin.net_profit / fin.eps))::numeric, 2) ELSE 0 END as total_market_cap,
            0 as circulating_market_cap
        FROM stocks_daily_k k
        JOIN LATERAL (
            SELECT f.eps, f.bps, f.total_revenue, f.net_profit, f.net_assets,
                   CASE WHEN f.total_revenue > 0 AND f.net_assets > 0 AND f.bps > 0
                        THEN f.total_revenue / (f.net_assets / f.bps) ELSE 0 END as revenue_per_share
            FROM stock_financials f
            WHERE f.code = k.code AND f.report_date <= k.trade_date::text AND (f.eps > 0 OR f.bps > 0)
            ORDER BY f.report_date DESC LIMIT 1
        ) fin ON true
        WHERE k.code = %s
        ON CONFLICT (code, trade_date) DO UPDATE SET
            pe = EXCLUDED.pe, pb = EXCLUDED.pb, ps = EXCLUDED.ps,
            total_market_cap = EXCLUDED.total_market_cap
    """, (code,))
    conn.commit()

    # ── Step 7: 验证结果 ──
    cur.execute("SELECT COUNT(*), MIN(trade_date), MAX(trade_date) FROM stocks_daily_k WHERE code = %s", (code,))
    new_k, min_d, max_d = cur.fetchone()

    cur.execute("SELECT COUNT(*), COUNT(*) FILTER (WHERE turnover_rate > 0) FROM stocks_daily_k WHERE code = %s", (code,))
    total_k, has_to = cur.fetchone()

    cur.execute("SELECT COUNT(*) FROM stocks_daily_indicator WHERE code = %s AND pe > 0", (code,))
    ind_count = cur.fetchone()[0]

    cur.close()
    conn.close()

    result = {
        "code": code,
        "oldKline": old_k_count,
        "oldIndicator": old_i_count,
        "circulatingShares": circ_shares,
        "fetched": len(klines),
        "inserted": new_k,
        "dateRange": f"{min_d} ~ {max_d}",
        "withTurnover": has_to,
        "indicatorsRecalc": ind_count,
    }
    log(json.dumps(result, ensure_ascii=False))

if __name__ == "__main__":
    main()
