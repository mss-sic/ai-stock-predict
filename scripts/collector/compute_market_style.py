#!/usr/bin/env python3
"""
盘后市场风格计算脚本（pipeline 自动调用）
调用 API /api/v1/market/compute-style?date=TODAY 写入 market_style_daily。
"""
import os, sys, urllib.request, json
from datetime import datetime

API_BASE = os.environ.get("API_BASE", "http://localhost:8080/api/v1")

def main():
    date = sys.argv[1] if len(sys.argv) > 1 else datetime.now().strftime("%Y-%m-%d")
    url = f"{API_BASE}/market/compute-style?date={date}"

    try:
        req = urllib.request.Request(url)
        with urllib.request.urlopen(req, timeout=30) as resp:
            data = json.loads(resp.read().decode())
            if data.get("code") == 0:
                print(f"[market_style] OK: {date}")
                return 0
            else:
                print(f"[market_style] API error: {data.get('message','unknown')}", file=sys.stderr)
                return 1
    except Exception as e:
        print(f"[market_style] request failed: {e}", file=sys.stderr)
        return 1

if __name__ == "__main__":
    sys.exit(main())
