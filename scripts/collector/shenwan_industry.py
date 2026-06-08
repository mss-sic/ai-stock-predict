#!/usr/bin/env python3
"""
申万行业分类填充 — 基于 TDX 行业映射 + 东财 HTTP 交叉验证
存储到 concept_tags JSONB 数组: ["sw:食品饮料", "dc:酿酒行业"]
"""
import os, psycopg2, json, requests, time
os.environ['NO_PROXY'] = '*'

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

# TDX 行业 → 申万一级行业 映射 (56 → 28)
TDX_TO_SW = {
    "银行保险": "银行",
    "证券": "非银金融",
    "钢铁": "钢铁",
    "建材": "建筑材料",
    "石油化工": "石油石化",
    "化工": "基础化工",
    "汽车": "汽车",
    "交通运输": "交通运输",
    "医疗保健": "医药生物",
    "旅游": "社会服务",
    "房地产": "房地产",
    "商业连锁": "商贸零售",
    "外贸": "商贸零售",
    "食品饮料": "食品饮料",
    "纺织服饰": "纺织服饰",
    "电力": "公用事业",
    "农林牧渔": "农林牧渔",
    "传媒娱乐": "传媒",
    "化工新材料": "基础化工",
    "煤炭": "煤炭",
    "有色": "有色金属",
    "水泥": "建筑材料",
    "家用电器": "家用电器",
    "通信设备": "通信",
    "IT设备": "计算机",
    "机械制造": "机械设备",
    "化纤": "基础化工",
    "农药化肥": "基础化工",
    "电气设备": "电力设备",
    "摩托车": "汽车",
    "综合类": "综合",
    "仓储物流": "交通运输",
    "船舶": "国防军工",
    "医药": "医药生物",
    "元器件": "电子",
    "矿物制品": "有色金属",
    "酿酒": "食品饮料",
    "造纸": "轻工制造",
    "环保": "环保",
    "陶瓷": "建筑材料",
    "服装家纺": "纺织服饰",
    "供气供热": "公用事业",
    "新能源": "电力设备",
    "互联网": "传媒",
    "工程机械": "机械设备",
    "广告包装": "轻工制造",
    "塑料": "基础化工",
    "文教休闲": "社会服务",
    "航空": "国防军工",
    "日用化工": "美容护理",
    "软件服务": "计算机",
    "通用机械": "机械设备",
    "专用机械": "机械设备",
    "运输设备": "交通运输",
    "水务": "公用事业",
    "水力发电": "公用事业",
}

def fetch_dc_industry(code):
    """从东财 HTTP 获取上市行业 (sshy)"""
    prefix = "SH" if code.startswith(("6", "9")) else "SZ"
    try:
        r = requests.get(
            f"http://emweb.securities.eastmoney.com/PC_HSF10/CompanySurvey/CompanySurveyAjax?code={prefix}{code}",
            headers={"User-Agent": "Mozilla/5.0"},
            timeout=5,
        )
        data = r.json()
        return data.get("jbzl", {}).get("sshy", "")
    except:
        return ""

def main():
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()
    cur.execute("SELECT code, name, industry, concept_tags FROM stocks_basic ORDER BY code")
    stocks = [(r[0], r[1], r[2] or '', r[3] or []) for r in cur.fetchall()]
    print(f"共 {len(stocks)} 只股票")

    updated = 0
    sw_stats = {}
    dc_stats = {}

    for i, (code, name, tdx_ind, existing_tags) in enumerate(stocks):
        tags = list(existing_tags) if isinstance(existing_tags, list) else []

        # Map TDX → 申万
        sw_ind = TDX_TO_SW.get(tdx_ind, tdx_ind if tdx_ind else "未知")
        sw_tag = f"sw:{sw_ind}"
        if sw_tag not in tags:
            tags.append(sw_tag)
        sw_stats[sw_ind] = sw_stats.get(sw_ind, 0) + 1

        # Fetch 东财 industry via HTTP
        dc_ind = fetch_dc_industry(code)
        if dc_ind and dc_ind != code:
            dc_tag = f"dc:{dc_ind}"
            if dc_tag not in tags and dc_ind != "—":
                tags.append(dc_tag)
            dc_stats[dc_ind] = dc_stats.get(dc_ind, 0) + 1

        # Clean up: remove any old sw:/dc: tags to avoid duplicates
        final_tags = []
        seen = set()
        for t in tags:
            # For sw: and dc: prefixes, keep only the latest
            if t.startswith("sw:") or t.startswith("dc:"):
                prefix = t[:3]
                base = t
                key = prefix + base.split(":")[1] if ":" in base else base
            else:
                key = t
            if key not in seen:
                seen.add(key)
                final_tags.append(t)

        cur.execute("""
            UPDATE stocks_basic SET concept_tags = %s::jsonb, updated_at = NOW()
            WHERE code = %s
        """, (json.dumps(final_tags, ensure_ascii=False), code))
        updated += 1

        if (i+1) % 100 == 0:
            print(f"  {i+1}/{len(stocks)}")

    conn.commit()
    cur.close()
    conn.close()

    print(f"\n✅ 更新完成: {updated}/{len(stocks)}")

    print(f"\n📊 申万行业分布 (28个):")
    for ind, cnt in sorted(sw_stats.items(), key=lambda x: -x[1]):
        bar = "█" * max(1, cnt // 3)
        print(f"  {ind:　<6s} {cnt:>3d} {bar}")

    if dc_stats:
        print(f"\n📊 东财行业分布 ({len(dc_stats)}个):")
        for ind, cnt in sorted(dc_stats.items(), key=lambda x: -x[1])[:15]:
            print(f"  {ind:　<8s} {cnt:>3d}")

if __name__ == "__main__":
    main()
