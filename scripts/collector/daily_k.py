#!/usr/bin/env python3
"""日K线数据采集 - 从腾讯财经API获取"""
import argparse, json, sys, time, requests

def fetch_kline(code, days=365):
    """拉取日K线，优先用腾讯财经（不封IP）"""
    if code.startswith("92"):
        prefix = "nq"
    elif code.startswith(("6", "9")):
        prefix = "sh"
    elif code.startswith("8"):
        prefix = "nq"
    else:
        prefix = "sz"
    url = f"http://ifzq.gtimg.cn/appstock/app/fqkline/get"
    params = {
        "param": f"{prefix}{code},day,,,{days},qfq"
    }
    headers = {"User-Agent": "Mozilla/5.0"}
    try:
        resp = requests.get(url, params=params, headers=headers, timeout=30)
        data = resp.json()
        stock_data = data.get("data", {})
        klines = []
        for pfx in [prefix, "sh", "sz", "nq"]:
            sd = stock_data.get(f"{pfx}{code}", {})
            klines = sd.get("day", []) or sd.get("qfqday", [])
            if klines:
                break
        result = []
        for row in klines:
            if len(row) >= 6:
                result.append({
                    "date": row[0],
                    "open": float(row[1]),
                    "close": float(row[2]),
                    "high": float(row[3]),
                    "low": float(row[4]),
                    "volume": int(float(row[5])) if code.startswith("688") else int(float(row[5]) * 100),
                })
        return result
    except Exception as e:
        return {"error": str(e)}

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--code", required=True)
    parser.add_argument("--days", type=int, default=365)
    parser.add_argument("--output", default="json")
    args = parser.parse_args()

    result = fetch_kline(args.code, args.days)
    print(json.dumps(result, ensure_ascii=False))

if __name__ == "__main__":
    main()
