#!/usr/bin/env python3
"""
修复历史K线成交量/成交额单位不一致问题

问题：Collector 从腾讯 API 获取数据时，volume 存储为 手(未×100)，
      amount 使用错误公式 close×vol/100 (应为 close×vol×100)。
      CSV 导入的 volume 为 股 且 amount 正确。

修复：volume × 100 (手→股)，amount = close × volume_gu (重新计算)
判定：amount / volume < 1 → 旧数据(手)；否则已是 股，跳过。
"""
import os, sys, time, psycopg2

os.environ['PYTHONUNBUFFERED'] = '1'
PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

def main():
    dry_run = '--dry-run' in sys.argv
    limit = None
    for a in sys.argv[1:]:
        if a.isdigit():
            limit = int(a)
    
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()
    
    # Count affected rows
    cur.execute("""
        SELECT COUNT(*) FROM stocks_daily_k
        WHERE volume > 0 AND amount > 0 
          AND amount / NULLIF(volume, 0) < 1 
          AND volume < 10000000
    """)
    total = cur.fetchone()[0]
    print(f"待修复行数: {total}")
    
    if total == 0:
        print("没有需要修复的数据")
        cur.close(); conn.close()
        return
    
    if dry_run:
        # Show sample
        print("\n[Dry Run] 修复前样本:")
        cur.execute("""
            SELECT code, trade_date, close, volume, amount
            FROM stocks_daily_k
            WHERE volume > 0 AND amount > 0 
              AND amount / NULLIF(volume, 0) < 1 
              AND volume < 10000000
            ORDER BY trade_date DESC, code
            LIMIT 5
        """)
        for r in cur.fetchall():
            code, d, c, v, a = r
            new_v = v * 100
            new_a = float(c) * new_v
            print(f"  {code} {d} V={v}→{new_v} A={a:.0f}→{new_a:.0f}")
        cur.close(); conn.close()
        return
    
    # Batch fix with transaction
    limit_clause = f"LIMIT {limit}" if limit else ""
    
    start = time.time()
    cur.execute(f"""
        UPDATE stocks_daily_k SET
            volume = volume * 100,
            amount = close * (volume * 100)
        WHERE volume > 0 AND amount > 0 
          AND amount / NULLIF(volume, 0) < 1 
          AND volume < 10000000
        {limit_clause}
    """)
    fixed = cur.rowcount
    conn.commit()
    
    elapsed = time.time() - start
    print(f"✅ 已修复: {fixed} 行 | {elapsed:.0f}s")
    
    # Verify
    cur.execute("""
        SELECT COUNT(*) FROM stocks_daily_k
        WHERE volume > 0 AND amount > 0 
          AND amount / NULLIF(volume, 0) < 1 
          AND volume < 10000000
    """)
    remaining = cur.fetchone()[0]
    print(f"剩余待修复: {remaining} 行")
    
    # Sample verification
    cur.execute("""
        SELECT code, trade_date, close, volume, amount
        FROM stocks_daily_k WHERE code='601398'
        ORDER BY trade_date DESC LIMIT 3
    """)
    print("\n工商银行 验证:")
    for r in cur.fetchall():
        d, c, v, a = r[1], float(r[2]), r[3], float(r[4])
        ratio = a / v if v > 0 else 0
        print(f"  {d} V={v:,} A={a:,.0f} A/V={ratio:.2f} (应≈{float(c):.2f})")
    
    cur.close()
    conn.close()

if __name__ == "__main__":
    main()
