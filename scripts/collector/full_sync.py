#!/usr/bin/env python3
"""全量A股同步 — 混合 mootdx stocks/block + 新浪API 获取全量代码+名称并写入 stocks_basic"""
import os, sys, json, time, urllib.request, psycopg2
os.environ['NO_PROXY'] = '*'
from mootdx.quotes import Quotes

PG_DSN = "host=localhost dbname=stock_predict user=stock password=stock123"

def get_sh_codes(client):
    df = client.stocks()
    codes = df['code'].astype(str).tolist()
    names = df['name'].astype(str).tolist()
    result = {}
    for c, n in zip(codes, names):
        c = str(c).zfill(6)
        if c.startswith('60') or c.startswith('68'):
            result[c] = str(n).strip()
        elif (c.startswith('8') or c.startswith('9')) and not c.startswith('90'):
            if c[1] in '0123456789':
                result[c] = str(n).strip()
    return result

def get_sz_codes_from_block(client):
    blocks = client.block()
    codes = blocks['code'].dropna().unique()
    valid = set()
    for c in codes:
        c = str(c).strip()
        if len(c) == 6 and c.isdigit():
            if c.startswith('00') or c.startswith('30'):
                valid.add(c)
    return sorted(valid)

def _clean_name(name):
    if not name:
        return name
    return name.replace('\x00', '').replace('\ufffd', '').strip()

def fetch_sz_names_sina(codes, batch_size=50):
    names = {}
    headers = {'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)', 'Referer': 'https://finance.sina.com.cn'}
    total = len(codes)
    for i in range(0, total, batch_size):
        batch = codes[i:i+batch_size]
        symbols = ','.join(f'sz{c}' for c in batch)
        url = f'http://hq.sinajs.cn/list={symbols}'
        try:
            req = urllib.request.Request(url, headers=headers)
            with urllib.request.urlopen(req, timeout=15) as resp:
                text = resp.read().decode('gbk', errors='replace')
            for line in text.strip().split('\n'):
                if '=\"\"' in line or '=\"' not in line:
                    continue
                try:
                    code_part = line.split('hq_str_')[1].split('=\"')[0]
                    if not code_part.startswith('sz'):
                        continue
                    code = code_part[2:]
                    fields = line.split('=\"')[1].split(',')
                    name = _clean_name(fields[0])
                    if name and name != code and len(name) < 30:
                        if '指数' in name or name.endswith('指') or name.endswith('成指'):
                            continue
                        names[code] = name
                except:
                    pass
        except Exception as e:
            print(f"  ⚠️ 新浪批量失败 [{i}:{i+batch_size}]: {e}", flush=True)
        batch_num = (i // batch_size) + 1
        if batch_num % 20 == 0:
            print(f"  新浪进度: {min(i+batch_size, total)}/{total}", flush=True)
        time.sleep(0.3)
    return names

def main():
    print("正在连接通达信服务器...", flush=True)
    client = Quotes.factory(market='std')

    print("=" * 60, flush=True)
    print("全量 A 股同步 — 混合数据源", flush=True)
    print("=" * 60, flush=True)

    print("\n📊 步骤1: mootdx stocks() → 上交所+北交所代码+名称", flush=True)
    sh_map = get_sh_codes(client)
    sh60 = sum(1 for c in sh_map if c.startswith('60'))
    sh68 = sum(1 for c in sh_map if c.startswith('68'))
    sh89 = sum(1 for c in sh_map if c.startswith(('8','9')))
    print(f"  上交所+北交所: {len(sh_map)} 只", flush=True)
    print(f"    沪主板 60xxxx: {sh60}", flush=True)
    print(f"    科创板 68xxxx: {sh68}", flush=True)
    print(f"    北交所: {sh89}", flush=True)

    print("\n📊 步骤2: mootdx block() → 提取深交所候选代码", flush=True)
    sz_codes = get_sz_codes_from_block(client)
    sz00 = sum(1 for c in sz_codes if c.startswith('00'))
    sz30 = sum(1 for c in sz_codes if c.startswith('30'))
    print(f"  深交所候选: {len(sz_codes)} 只", flush=True)
    print(f"    深主板 00xxxx: {sz00}", flush=True)
    print(f"    创业板 30xxxx: {sz30}", flush=True)

    print("\n📊 步骤3: 新浪API → 批量验证深交所代码 + 获取名称", flush=True)
    sz_map = fetch_sz_names_sina(sz_codes)
    print(f"  深交所确认: {len(sz_map)} 只（排除指数后）", flush=True)

    all_stocks = {}
    all_stocks.update(sh_map)
    all_stocks.update(sz_map)
    all60 = sum(1 for c in all_stocks if c.startswith('60'))
    all688 = sum(1 for c in all_stocks if c.startswith('688'))
    all689 = sum(1 for c in all_stocks if c.startswith('689'))
    all00 = sum(1 for c in all_stocks if c.startswith('00'))
    all30 = sum(1 for c in all_stocks if c.startswith('30'))
    all89 = sum(1 for c in all_stocks if c.startswith(('8','9')))
    print(f"\n📊 全部 A 股汇总: {len(all_stocks)} 只", flush=True)
    print(f"  沪主板 60xxxx: {all60}", flush=True)
    print(f"  科创板 688xxx: {all688}", flush=True)
    print(f"  科创板 689xxx: {all689}", flush=True)
    print(f"  深主板 00xxxx: {all00}", flush=True)
    print(f"  创业板 30xxxx: {all30}", flush=True)
    print(f"  北交所 8/9: {all89}", flush=True)

    print("\n📊 步骤4: 入库 stocks_basic...", flush=True)
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    cur.execute("DELETE FROM stocks_basic WHERE code LIKE '01%' OR code LIKE '02%'")
    del_bonds = cur.rowcount
    if del_bonds:
        print(f"  🧹 清理债券: {del_bonds} 条", flush=True)

    cur.execute("SELECT code FROM stocks_basic")
    existing = set(r[0] for r in cur.fetchall())

    count_new, count_updated = 0, 0
    for code, name in sorted(all_stocks.items()):
        name = _clean_name(name)
        if not name or name == code:
            continue
        if code not in existing:
            cur.execute("""
                INSERT INTO stocks_basic (code, name, updated_at)
                VALUES (%s, %s, NOW())
                ON CONFLICT (code) DO NOTHING
            """, (code, name))
            count_new += 1
        else:
            cur.execute(
                "UPDATE stocks_basic SET name=%s, updated_at=NOW() WHERE code=%s AND (name=%s OR name='' OR name IS NULL)",
                (name, code, code))
            count_updated += 1

        if (count_new + count_updated) % 500 == 0:
            conn.commit()
            print(f"  已处理 {count_new + count_updated}/{len(all_stocks)}", flush=True)

    conn.commit()
    cur.execute("SELECT COUNT(*) FROM stocks_basic")
    total = cur.fetchone()[0]
    cur.close()
    conn.close()

    print(f"\n✅ 同步完成!", flush=True)
    new_count = upserted
    print(f"   新增: {count_new} 只", flush=True)
    print(f"   更新名称: {count_updated} 只", flush=True)
    print(f"   数据库总计: {total} 只", flush=True)

if __name__ == '__main__':
    main()
