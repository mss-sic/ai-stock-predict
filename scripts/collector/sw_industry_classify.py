#!/usr/bin/env python3
"""
申万行业分类填充 —  TDX→申万 L1 + TDX→东财 L2 + tushare L2 三重标记
=====================================================================
数据源:
  L1 (sw_l1):   TDX industry → 申万一级映射
  L2_dc (sw_l2_dc): TDX industry → 东财 BK04xx 映射 (手动对照表)
  L2 (sw_l2):   tushare stock_basic.industry (110 行业, 1次/小时限制)

用法: python3 sw_industry_classify.py

生产环境首次部署步骤:
  1. docker pull 新镜像 → docker compose up -d (迁移 v052 自动运行)
  2. docker exec <container> python3 scripts/collector/sw_industry_classify.py
  3. 触发市场风格批量重算: POST /api/v1/market/bulk-compute
"""
import os, sys, time, psycopg2

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

# ── TDX → 申万一级行业映射 ──
TDX_TO_SW = {
    "银行保险": "银行", "证券": "非银金融", "钢铁": "钢铁", "建材": "建筑材料",
    "石油化工": "石油石化", "化工": "基础化工", "汽车": "汽车",
    "交通运输": "交通运输", "医疗保健": "医药生物", "旅游": "社会服务",
    "房地产": "房地产", "商业连锁": "商贸零售", "外贸": "商贸零售",
    "食品饮料": "食品饮料", "纺织服饰": "纺织服饰", "电力": "公用事业",
    "农林牧渔": "农林牧渔", "传媒娱乐": "传媒", "化工新材料": "基础化工",
    "煤炭": "煤炭", "有色": "有色金属", "水泥": "建筑材料",
    "家用电器": "家用电器", "通信设备": "通信", "IT设备": "计算机",
    "机械制造": "机械设备", "化纤": "基础化工", "农药化肥": "基础化工",
    "电气设备": "电力设备", "摩托车": "汽车", "综合类": "综合",
    "仓储物流": "交通运输", "船舶": "国防军工", "医药": "医药生物",
    "元器件": "电子", "矿物制品": "有色金属", "酿酒": "食品饮料",
    "造纸": "轻工制造", "环保": "环保", "陶瓷": "建筑材料",
    "服装家纺": "纺织服饰", "供气供热": "公用事业", "新能源": "电力设备",
    "互联网": "传媒", "工程机械": "机械设备", "广告包装": "轻工制造",
    "塑料": "基础化工", "文教休闲": "社会服务", "航空": "国防军工",
    "日用化工": "美容护理", "软件服务": "计算机", "通用机械": "机械设备",
}

# ── TDX → 东财 BK04xx 行业映射 (手动对照, 语义近似的二级行业) ──
TDX_TO_DC = {
    "IT设备": "元件",
    "交通运输": "铁路公路",
    "仓储物流": "物流",
    "传媒娱乐": "传媒",
    "供气供热": "公用事业",
    "元器件": "元件",
    "农林牧渔": "农林牧渔",
    "农药化肥": "化学制药",
    "化工": "石油石化",
    "化工新材料": "塑料",
    "化纤": "化学纤维",
    "医疗保健": "化学制药",
    "医药": "化学制药",
    "商业连锁": "一般零售",
    "塑料": "塑料",
    "外贸": "贸易Ⅱ",
    "家用电器": "家用电器",
    "工程机械": "工程建设",
    "广告包装": "造纸印刷",
    "建材": "装修建材",
    "房地产": "房地产开发",
    "摩托车": "交运设备",
    "文教休闲": "旅游酒店",
    "新能源": "电网设备",
    "旅游": "旅游酒店",
    "日用化工": "化学制药",
    "有色": "有色金属",
    "服装家纺": "纺织服饰",
    "机械制造": "工程建设",
    "水泥": "水泥",
    "汽车": "汽车零部件",
    "煤炭": "煤炭",
    "环保": "公用事业",
    "电力": "电力",
    "电气设备": "电网设备",
    "石油化工": "石油石化",
    "矿物制品": "有色金属",
    "纺织服饰": "纺织服饰",
    "航空": "航空机场",
    "船舶": "航运港口",
    "软件服务": "互联网服务",
    "通信设备": "通信设备",
    "通用机械": "工程建设",
    "造纸": "造纸印刷",
    "酿酒": "食品饮料",
    "钢铁": "钢铁",
    "银行保险": "银行Ⅱ",
    "陶瓷": "装修建材",
    "食品饮料": "食品饮料",
}

def main():
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    cur.execute("SELECT code, name, industry FROM stocks_basic ORDER BY code")
    stocks = cur.fetchall()
    total = len(stocks)
    print(f"[sw_classify] 总股票数: {total}")

    # Phase 1: L1 (TDX → 申万)
    print("[sw_classify] Phase 1: TDX → 申万 L1 ...")
    l1_count = 0
    batch = []
    for code, name, tdx in stocks:
        sw = TDX_TO_SW.get(tdx or '', '')
        if sw:
            batch.append((sw, code))
            l1_count += 1

    B = 500
    for i in range(0, len(batch), B):
        chunk = batch[i:i+B]
        for sw, code in chunk:
            cur.execute("UPDATE stocks_basic SET sw_l1 = %s, updated_at = NOW() WHERE code = %s", (sw, code))
        conn.commit()
    print(f"  L1: {l1_count}/{total} ({100*l1_count//total}%)")

    # Phase 2: L2_dc (TDX → BK04xx)
    print("[sw_classify] Phase 2: TDX → 东财 BK04xx L2 ...")
    l2dc_count = 0
    batch2 = []
    for code, name, tdx in stocks:
        dc = TDX_TO_DC.get(tdx or '', '')
        if dc:
            batch2.append((dc, code))
            l2dc_count += 1

    for i in range(0, len(batch2), B):
        chunk = batch2[i:i+B]
        for dc, code in chunk:
            cur.execute("UPDATE stocks_basic SET sw_l2_dc = %s, updated_at = NOW() WHERE code = %s", (dc, code))
        conn.commit()
    print(f"  L2_dc: {l2dc_count}/{total} ({100*l2dc_count//total}%)")

    # Phase 3: L2 (tushare) — only if rate limit allows
    print("[sw_classify] Phase 3: tushare L2 (尝试中)...")
    try:
        import tushare as ts
        TUSHARE_TOKEN = os.environ.get("TUSHARE_TOKEN", "12fa5227e5ff02a42299aad5da4797830b52f30da5ce009358fe66cb")
        ts.set_token(TUSHARE_TOKEN)
        pro = ts.pro_api()
        df = pro.stock_basic(exchange='', list_status='L', fields='ts_code,name,industry')
        ts_map = {}
        for _, row in df.iterrows():
            code = row['ts_code'].split('.')[0]
            ts_map[code] = row.get('industry', '')
        l2_count = 0
        for code in ts_map:
            if ts_map[code]:
                cur.execute("UPDATE stocks_basic SET sw_l2 = %s WHERE code = %s AND sw_l2 = ''", (ts_map[code], code))
                l2_count += 1
        conn.commit()
        print(f"  L2 (tushare): {l2_count}/{total} ({100*l2_count//total}%)")
    except Exception as e:
        print(f"  L2 (tushare): 跳过 ({e})")

    cur.close()
    conn.close()

    # Final report
    conn2 = psycopg2.connect(PG_DSN)
    cur2 = conn2.cursor()
    cur2.execute("""
        SELECT
            COUNT(*) FILTER (WHERE sw_l1 != '') as l1,
            COUNT(*) FILTER (WHERE sw_l2 != '') as l2,
            COUNT(*) FILTER (WHERE sw_l2_dc != '') as l2dc,
            COUNT(*) as total
        FROM stocks_basic
    """)
    r = cur2.fetchone()
    print(f"\n[sw_classify] ✅ 完成!")
    print(f"  sw_l1 (申万一级): {r[0]}/{r[3]} ({100*r[0]//r[3]}%)")
    print(f"  sw_l2 (tushare):  {r[1]}/{r[3]} ({100*r[1]//r[3]}%)")
    print(f"  sw_l2_dc (东财):  {r[2]}/{r[3]} ({100*r[2]//r[3]}%)")
    cur2.close()
    conn2.close()

if __name__ == "__main__":
    main()
