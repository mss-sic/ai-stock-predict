#!/usr/bin/env python3
"""实时行情采集 — 腾讯 qt.gtimg.cn，批量拉取 + upsert 到 stock_realtime_quote"""
import os, sys, time, ssl, urllib.request
import psycopg2
from psycopg2.extras import execute_values

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
BATCH_SIZE = 80  # 腾讯单次最多约 80 只

def fetch_quotes(codes):
    """Batch-fetch real-time quotes from Tencent. Returns {code: {fields...}}."""
    results = {}
    for i in range(0, len(codes), BATCH_SIZE):
        batch = codes[i:i + BATCH_SIZE]
        symbols = []
        for c in batch:
            prefix = "sh" if c.startswith(("6", "9")) else "sz"
            symbols.append(f"{prefix}{c}")
        url = f"http://qt.gtimg.cn/q={','.join(symbols)}"
        req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
        ctx = ssl.create_default_context()
        try:
            with urllib.request.urlopen(req, timeout=10, context=ctx) as resp:
                text = resp.read().decode("gbk", errors="replace")
            for line in text.strip().split("\n"):
                if '="' not in line:
                    continue
                try:
                    code_part = line.split("_")[1].split('="')[0] if "_" in line else ""
                    code = code_part[2:] if len(code_part) > 2 else code_part
                    fields = line.split('="')[1].rstrip('";').split("~")
                    if len(fields) < 45:
                        continue
                    results[code] = {
                        "name": fields[1],
                        "price": float(fields[3]) if fields[3] else 0,
                        "prev_close": float(fields[4]) if fields[4] else 0,
                        "open": float(fields[5]) if fields[5] else 0,
                        "volume": int(fields[6]) if fields[6] else 0,
                        "change_pct": float(fields[31]) if fields[31] else 0,
                        "high": float(fields[32]) if fields[32] else 0,
                        "low": float(fields[33]) if fields[33] else 0,
                        "amount": float(fields[36]) if fields[36] else 0,
                        "turnover": float(fields[38]) if fields[38] else 0,
                        "pe": float(fields[39]) if fields[39] else 0,
                        "total_mcap": float(fields[44]) if fields[44] else 0,
                        "circulating_mcap": float(fields[45]) if fields[45] else 0,
                        "pb": float(fields[46]) if fields[46] else 0,
                        "amplitude": float(fields[42]) if fields[42] else 0,
                    }
                except Exception:
                    continue
        except Exception as e:
            print(f"  ⚠️ 行情批量请求失败: {e}", flush=True)
    return results


def gather_monitor_codes(cur):
    """Gather watchlist + holdings + board stocks from DB (for scheduled task)."""
    codes = set()
    
    # 1. Watchlist stocks (all users)
    try:
        cur.execute("""
            SELECT DISTINCT stock_code FROM watchlist_items
        """)
        for r in cur.fetchall():
            code = r[0].strip()
            if len(code) == 6:
                codes.add(code)
    except Exception as e:
        print(f"  ⚠️ 获取自选股失败: {e}", flush=True)
    
    # 2. Holdings stocks (all users, non-zero position)
    try:
        cur.execute("""
            SELECT DISTINCT b.stock_code FROM (
                SELECT stock_code, SUM(CASE WHEN trade_type='buy' THEN quantity ELSE -quantity END) as net
                FROM trade_records GROUP BY stock_code
            ) b WHERE b.net > 0
        """)
        for r in cur.fetchall():
            code = r[0].strip()
            if len(code) == 6:
                codes.add(code)
    except Exception as e:
        print(f"  ⚠️ 获取持仓股失败: {e}", flush=True)
    
    # 3. Board picks from last 2 trading days
    try:
        cur.execute("""
            SELECT DISTINCT code FROM board_picks
            WHERE trade_date >= (SELECT MAX(trade_date) FROM board_picks WHERE trade_date < CURRENT_DATE)
        """)
        for r in cur.fetchall():
            code = r[0].strip()
            if len(code) == 6:
                codes.add(code)
    except Exception as e:
        print(f"  ⚠️ 获取榜单股失败: {e}", flush=True)
    
    return sorted(codes)


def main():
    codes_arg = sys.argv[1] if len(sys.argv) > 1 else ""
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    if codes_arg == "--all":
        codes = gather_monitor_codes(cur)
        print(f"📊 实时行情监控模式 | 数据源: 腾讯 qt.gtimg.cn", flush=True)
        print(f"📋 自选+持仓+榜单共 {len(codes)} 只股票", flush=True)
    elif codes_arg:
        codes = [c.strip() for c in codes_arg.split(",") if c.strip()]
    else:
        print("用法: python3 realtime_quotes.py <code1,code2,...>", flush=True)
        print("       python3 realtime_quotes.py --all  (监控模式:自选+持仓+榜单)", flush=True)
        print("      或由 Go 服务端传入自选股+持仓+榜单股票列表", flush=True)
        return

    if not codes:
        print("没有需要采集的股票", flush=True)
        return

    print(f"📊 实时行情采集 | 数据源: 腾讯 qt.gtimg.cn", flush=True)
    print(f"📋 目标: {len(codes)} 只股票", flush=True)

    t0 = time.time()
    quotes = fetch_quotes(codes)
    upsert_count = 0

    if quotes:
        rows = []
        for code, q in quotes.items():
            rows.append((
                code, q["name"], q["price"], q["prev_close"], q["open"],
                q["high"], q["low"], q["volume"], q["amount"],
                q["change_pct"], q["turnover"], q["pe"], q["pb"],
                q["total_mcap"], q["circulating_mcap"], q["amplitude"],
            ))

        sql = """
            INSERT INTO stock_realtime_quote
                (code, name, price, prev_close, open, high, low, volume, amount,
                 change_pct, turnover_rate, pe, pb, total_market_cap, circulating_mcap, amplitude)
            VALUES %s
            ON CONFLICT (code) DO UPDATE SET
                name = EXCLUDED.name, price = EXCLUDED.price,
                prev_close = EXCLUDED.prev_close, open = EXCLUDED.open,
                high = EXCLUDED.high, low = EXCLUDED.low,
                volume = EXCLUDED.volume, amount = EXCLUDED.amount,
                change_pct = EXCLUDED.change_pct, turnover_rate = EXCLUDED.turnover_rate,
                pe = EXCLUDED.pe, pb = EXCLUDED.pb,
                total_market_cap = EXCLUDED.total_market_cap,
                circulating_mcap = EXCLUDED.circulating_mcap,
                amplitude = EXCLUDED.amplitude, updated_at = NOW()
        """
        execute_values(cur, sql, rows, page_size=100)
        conn.commit()
        upsert_count = len(rows)

    elapsed = time.time() - t0
    failed = len(codes) - upsert_count
    print(f"✅ 实时行情完成: 成功 {upsert_count} 只 | 失败 {failed} 只 | 耗时 {elapsed:.1f}s", flush=True)
    if failed > 0:
        missing = set(codes) - set(quotes.keys())
        if missing:
            print(f"   ⚠️  未获取到数据: {', '.join(sorted(missing)[:10])}", flush=True)


    # ── 同步更新 stocks_daily_k.turnover_rate 和 stocks_daily_indicator ──
    turnover_updated = 0
    indicator_updated = 0
    for code, q in quotes.items():
        if q.get("turnover", 0) > 0:
            cur.execute("""
                UPDATE stocks_daily_k SET turnover_rate = %s
                WHERE code = %s AND trade_date = (
                    SELECT MAX(trade_date) FROM stocks_daily_k WHERE code = %s
                )
            """, (q["turnover"] / 100, code, code))
            if cur.rowcount > 0:
                turnover_updated += 1
        if q.get("pe", 0) > 0 or q.get("total_mcap", 0) > 0:
            cur.execute("SELECT MAX(trade_date) FROM stocks_daily_k WHERE code = %s", (code,))
            latest = cur.fetchone()[0]
            if latest:
                try:
                    cur.execute("""
                        INSERT INTO stocks_daily_indicator (code, trade_date, pe, pb, total_market_cap, circulating_market_cap)
                        VALUES (%s, %s, %s, %s, %s, %s)
                        ON CONFLICT (code, trade_date) DO UPDATE SET
                            pe = EXCLUDED.pe, pb = EXCLUDED.pb,
                            total_market_cap = EXCLUDED.total_market_cap,
                            circulating_market_cap = EXCLUDED.circulating_market_cap
                    """, (code, latest, q.get("pe", 0), q.get("pb", 0),
                          q.get("total_mcap", 0), q.get("circulating_mcap", 0)))
                    if cur.rowcount > 0:
                        indicator_updated += 1
                except Exception as e:
                    print(f"  \u26a0\ufe0f  指标写入失败 {code}: {e}", flush=True)
    conn.commit()
    if turnover_updated > 0 or indicator_updated > 0:
        print(f"  \U0001f4ca 同步: 换手率 {turnover_updated} 只 | PE/市值 {indicator_updated} 只", flush=True)

    cur.close()
    conn.close()


if __name__ == "__main__":
    main()
