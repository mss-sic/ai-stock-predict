#!/usr/bin/env python3
"""
行业数据填充 — 从 mootdx 获取行业分类并写入 stocks_basic
支持多源行业分类：通达信 (TDX) 为主，后续可扩展申万/东财
"""
import os, psycopg2
os.environ['NO_PROXY'] = '*'

# 通达信行业代码 → 名称映射（基于通达信标准行业分类）
TDX_INDUSTRY_MAP = {
    1: "银行保险",
    2: "证券",
    3: "钢铁",
    4: "建材",
    5: "石油化工",
    6: "化工",
    7: "汽车",
    8: "交通运输",
    9: "医疗保健",
    10: "旅游",
    11: "房地产",
    12: "商业连锁",
    13: "外贸",
    14: "食品饮料",
    15: "纺织服饰",
    16: "电力",
    17: "农林牧渔",
    18: "传媒娱乐",
    19: "化工新材料",
    20: "煤炭",
    21: "有色",
    22: "水泥",
    23: "家用电器",
    24: "通信设备",
    25: "IT设备",
    26: "机械制造",
    27: "化纤",
    28: "农药化肥",
    29: "电气设备",
    30: "摩托车",
    31: "综合类",
    32: "仓储物流",
    33: "船舶",
    34: "医药",
    35: "元器件",
    36: "矿物制品",
    37: "酿酒",
    38: "造纸",
    39: "环保",
    40: "陶瓷",
    41: "服装家纺",
    42: "供气供热",
    43: "新能源",
    44: "互联网",
    45: "工程机械",
    46: "广告包装",
    47: "塑料",
    48: "文教休闲",
    49: "航空",
    50: "日用化工",
    51: "软件服务",
    52: "通用机械",
    53: "专用机械",
    54: "运输设备",
    55: "水务",
    56: "水力发电",
}

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

def main():
    from mootdx.quotes import Quotes
    client = Quotes.factory(market='std')

    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()
    cur.execute("SELECT code, name FROM stocks_basic ORDER BY code")
    stocks = [(r[0], r[1]) for r in cur.fetchall()]
    print(f"共 {len(stocks)} 只股票")

    updated = 0
    industry_stats = {}

    for i, (code, name) in enumerate(stocks):
        try:
            fin = client.finance(code)
            if fin is None or len(fin) == 0:
                continue
            row = fin.iloc[-1]
            ind_code = int(row.get('industry', 0) or 0)
            province = int(row.get('province', 0) or 0)

            if ind_code > 0:
                ind_name = TDX_INDUSTRY_MAP.get(ind_code, f"行业{ind_code}")
                industry_stats[ind_name] = industry_stats.get(ind_name, 0) + 1

                cur.execute("""
                    UPDATE stocks_basic SET industry = %s, updated_at = NOW()
                    WHERE code = %s
                """, (ind_name, code))
                updated += 1
        except Exception as e:
            pass

        if (i+1) % 100 == 0:
            print(f"  {i+1}/{len(stocks)} | 更新: {updated}")

    conn.commit()
    cur.close()
    conn.close()

    print(f"\n✅ 行业更新完成: {updated}/{len(stocks)}")
    print("\n📊 行业分布:")
    for ind, cnt in sorted(industry_stats.items(), key=lambda x: -x[1]):
        bar = "█" * max(1, cnt // 2)
        print(f"  {ind:　<8s} {cnt:>3d} {bar}")

if __name__ == "__main__":
    main()
