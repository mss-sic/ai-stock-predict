#!/bin/bash
# 全量指标测试脚本 — 纯 Python，支持传 token 或自动登录
# Usage:
#   ./test_indicators.sh 300632 2026-07-21                    # 自动登录 (admin/admin123)
#   TOKEN=xxx ./test_indicators.sh 300632 2026-07-21         # 用指定 token
set -e

BASE_URL="${API_BASE_URL:-http://localhost:8080}"
STOCK="${1:-000001}"
DATE="${2:-$(date +%Y-%m-%d)}"
API_TOKEN="${TOKEN:-}"

python3 << PYEOF
import urllib.request, json, sys, os

BASE = "${BASE_URL}"
STOCK = "${STOCK}"
DATE = "${DATE}"
TOKEN = os.environ.get("TOKEN", "")

def req(method, path, body=None):
    url = BASE + path
    data = json.dumps(body).encode() if body else None
    r = urllib.request.Request(url, data=data, method=method)
    r.add_header("Content-Type", "application/json")
    if TOKEN:
        r.add_header("Authorization", "Bearer " + TOKEN)
    try:
        with urllib.request.urlopen(r, timeout=15) as resp:
            return json.loads(resp.read())
    except Exception as e:
        return {"error": str(e)}

# Login if no token provided
if not TOKEN:
    import subprocess
    # Try reading from docker-compose env or common passwords
    for pw in ["admin123", "admin", "stock123", ""]:
        resp = req("POST", "/api/v1/auth/login", {"username": "admin", "password": pw})
        if resp and resp.get("code") == 0:
            TOKEN = resp["data"]["accessToken"]
            break
    if not TOKEN:
        print("❌ 自动登录失败，请手动传入 TOKEN 环境变量")
        print("   TOKEN=xxx bash test_indicators.sh 300632 2026-07-21")
        sys.exit(1)

# Get all indicators
resp = req("GET", "/api/v1/strategies/indicators")
indicators = resp.get("data", []) if resp else []
total = len(indicators)

pass_cnt = 0
zero_cnt = 0
nodata_cnt = 0
nodata_keys = []

print(f"🔍 {STOCK}  {DATE}  {total}个指标")
print()

for ind in indicators:
    key = ind["key"]
    resp = req("POST", "/api/v1/strategies/test-indicator", {
        "stockCode": STOCK, "date": DATE,
        "indicator": key, "operator": "gte", "value": 0
    })
    data = resp.get("data", {}) if resp else {}
    has = data.get("hasData", False)
    val = data.get("computedValue", "N/A")

    if has:
        if val == 0 or val == 0.0:
            zero_cnt += 1
            print(f"  ⚡ {key:<30s} → {val} (zero)")
        else:
            pass_cnt += 1
            print(f"  ✅ {key:<30s} → {val}")
    else:
        nodata_cnt += 1
        nodata_keys.append(key)
        print(f"  ❌ {key:<30s} → NO DATA")

print()
print(f"  ✅ Pass: {pass_cnt}  ⚡ Zero: {zero_cnt}  ❌ NoData: {nodata_cnt}")

if nodata_keys:
    print()
    print("⚠️  无数据指标:")
    for k in nodata_keys:
        print(f"    - {k}")
PYEOF
