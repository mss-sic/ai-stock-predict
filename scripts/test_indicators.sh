#!/bin/bash
# 全量指标测试脚本（纯 Python 实现，容器内无 curl 也可用）
set -e

BASE_URL="${API_BASE_URL:-http://localhost:8080}"
STOCK="${1:-000001}"
DATE="${2:-$(date +%Y-%m-%d)}"

python3 << PYEOF
import urllib.request, json, sys

BASE = "${BASE_URL}"
STOCK = "${STOCK}"
DATE = "${DATE}"
TOKEN = ""

def req(method, path, body=None):
    url = BASE + path
    data = json.dumps(body).encode() if body else None
    r = urllib.request.Request(url, data=data, method=method)
    r.add_header("Content-Type", "application/json")
    if TOKEN:
        r.add_header("Authorization", "Bearer " + TOKEN)
    try:
        with urllib.request.urlopen(r, timeout=10) as resp:
            return json.loads(resp.read())
    except Exception as e:
        return {"error": str(e)}

# Login
print("🔑 登录...")
resp = req("POST", "/api/v1/auth/login", {"username": "admin", "password": "admin123"})
if not resp or resp.get("code") != 0:
    print("❌ 登录失败:", resp.get("error", ""))
    sys.exit(1)
TOKEN = resp["data"]["accessToken"]

# Get all indicators
print("📋 获取指标列表...")
resp = req("GET", "/api/v1/strategies/indicators")
indicators = resp.get("data", []) if resp else []
total = len(indicators)
print(f"  共 {total} 个指标")

pass_cnt = 0
zero_cnt = 0
nodata_cnt = 0
nodata_keys = []

print()
print("╔══════════════════════════════════════════════════════════════════════╗")
print(f"║  Stock: {STOCK:<6s}  Date: {DATE:<10s}  Total: {total:<3d} indicators      ║")
print("╠══════════════════════════════════════════════════════════════════════╣")

for ind in indicators:
    key = ind["key"]
    body = {
        "stockCode": STOCK,
        "date": DATE,
        "indicator": key,
        "operator": "gte",
        "value": 0
    }
    resp = req("POST", "/api/v1/strategies/test-indicator", body)
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

print("╠══════════════════════════════════════════════════════════════════════╣")
print(f"║  ✅ Pass: {pass_cnt:<3d}  ⚡ Zero: {zero_cnt:<3d}  ❌ NoData: {nodata_cnt:<3d}                   ║")
print("╚══════════════════════════════════════════════════════════════════════╝")

if nodata_keys:
    print()
    print("⚠️  无数据指标:")
    for k in nodata_keys:
        print(f"    - {k}")
PYEOF
