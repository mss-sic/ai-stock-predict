#!/usr/bin/env python3
"""股票基础信息采集 - 从东财获取PE/PB/市值"""
import argparse, json, sys, time, requests

def fetch_basic(code):
    """获取股票基础信息"""
    prefix = "sh" if code.startswith(("6", "9")) else "sz"
    secid = f"1.{code}" if code.startswith(("6", "9")) else f"0.{code}"
    
    # 东财个股概要
    url = "https://push2.eastmoney.com/api/qt/stock/get"
    params = {
        "secid": f"{'1' if code.startswith(('6','9')) else '0'}.{code}",
        "fields": "f57,f58,f43,f44,f45,f46,f47,f48,f49,f50,f51,f52,f55,f116,f117,f162,f167,f168,f169,f170,f171"
    }
    headers = {
        "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
        "Referer": "https://quote.eastmoney.com/"
    }
    try:
        resp = requests.get(url, params=params, headers=headers, timeout=15)
        data = resp.json().get("data", {})
        time.sleep(1.0)
        return {
            "code": code,
            "name": data.get("f58", ""),
            "price": data.get("f43", 0) / 100 if data.get("f43") else 0,
            "pe": data.get("f162", 0) / 100 if data.get("f162") else 0,
            "pb": data.get("f167", 0) / 100 if data.get("f167") else 0,
            "marketCap": data.get("f116", 0),
            "circulatingMarketCap": data.get("f117", 0),
            "turnoverRate": data.get("f168", 0) / 100 if data.get("f168") else 0,
        }
    except Exception as e:
        return {"error": str(e)}

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--code", required=True)
    args = parser.parse_args()

    result = fetch_basic(args.code)
    print(json.dumps(result, ensure_ascii=False))

if __name__ == "__main__":
    main()
