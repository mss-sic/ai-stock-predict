#!/usr/bin/env python3
"""
行情同步守护进程 — 单次连接，交易时段每30秒刷新
用法: python3 quotes_daemon.py
"""
import os, psycopg2, time, signal, sys
from datetime import datetime, time as dt_time
os.environ['NO_PROXY'] = '*'

PG_DSN = "host=localhost dbname=stock_predict user=stock password=stock123"

# 交易时段（北京时间）
TRADING_START = dt_time(9, 15)
TRADING_END   = dt_time(15, 5)
REFRESH_SECS  = 30   # 盘中刷新间隔
IDLE_SECS     = 300  # 非交易时段休眠间隔

running = True

def shutdown(sig, frame):
    global running
    running = False
    print("\n⏸️  收到退出信号，安全关闭...")

signal.signal(signal.SIGINT, shutdown)
signal.signal(signal.SIGTERM, shutdown)

def is_trading_time() -> bool:
    now = datetime.now()
    if now.weekday() >= 5:  # 周六日
        return False
    t = now.time()
    return TRADING_START <= t <= TRADING_END

def main():
    from mootdx.quotes import Quotes
    
    print("📡 初始化 mootdx 连接...")
    client = Quotes.factory(market='std')
    
    # 一次性缓存流通股本
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()
    cur.execute("SELECT code FROM stocks_basic ORDER BY code")
    codes = [r[0] for r in cur.fetchall()]
    
    print(f"📊 获取 {len(codes)} 只流通股本...")
    float_shares = {}
    for code in codes:
        try:
            fin = client.finance(code)
            if fin is not None and len(fin) > 0:
                ltg = float(fin.iloc[-1].get('liutongguben', 0) or 0)
                if ltg > 0:
                    float_shares[code] = ltg
        except: pass
    print(f"   {len(float_shares)} 只有效")
    
    # 每日凌晨刷新股本（可能有送转）
    last_share_refresh = datetime.now().date()
    
    print("🚀 行情守护进程启动")
    loops = 0
    
    while running:
        loops += 1
        now = datetime.now()
        trading = is_trading_time()
        
        # 每日刷新一次股本
        if now.date() > last_share_refresh:
            print(f"  📅 新交易日，刷新股本...")
            last_share_refresh = now.date()
            # ... (可补充股本刷新逻辑)
        
        try:
            for i in range(0, len(codes), 80):
                batch = codes[i:i+80]
                df = client.quotes(symbol=batch)
                for _, row in df.iterrows():
                    code = str(row.get('code', '')).zfill(6)
                    if code not in batch: continue
                    
                    price = float(row.get('price', 0) or 0)
                    vol = int(float(row.get('volume', 0) or 0))
                    amt = float(row.get('amount', 0) or 0)
                    b_vol = int(float(row.get('b_vol', 0) or 0))
                    s_vol = int(float(row.get('s_vol', 0) or 0))
                    turnover = 0
                    fsh = float_shares.get(code, 0)
                    if fsh > 0 and vol > 0:
                        turnover = round(vol * 10000 / fsh, 4)
                    
                    cur.execute("""
                        INSERT INTO stock_quotes (code, price, open, high, low, volume, amount, bid_vol, ask_vol, turnover, updated_at)
                        VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s)
                        ON CONFLICT (code) DO UPDATE SET
                            price=EXCLUDED.price, volume=EXCLUDED.volume,
                            amount=EXCLUDED.amount, bid_vol=EXCLUDED.bid_vol,
                            ask_vol=EXCLUDED.ask_vol, turnover=EXCLUDED.turnover,
                            updated_at=EXCLUDED.updated_at
                    """, (code, price,
                          float(row.get('open', 0) or 0),
                          float(row.get('high', 0) or 0),
                          float(row.get('low', 0) or 0),
                          vol, amt, b_vol, s_vol, turnover, now))
            conn.commit()
        except Exception as e:
            print(f"  ⚠️ 采集异常: {e}")
            time.sleep(5)
        
        sleep_time = REFRESH_SECS if trading else IDLE_SECS
        status = "🟢 盘中" if trading else "⏸️ 休市"
        print(f"  [{now.strftime('%H:%M:%S')}] {status} 第{loops}轮 | 下次刷新: {sleep_time}s")
        time.sleep(sleep_time)
    
    cur.close()
    conn.close()
    print("👋 守护进程已退出")

if __name__ == "__main__":
    main()
