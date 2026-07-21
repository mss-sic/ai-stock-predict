#!/bin/bash
# 全量指标测试脚本
set -e

BASE_URL="${API_BASE_URL:-http://localhost:8080}"

echo "🔑 登录..."
TOKEN=$(curl -s -X POST "$BASE_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['accessToken'])")
AUTH="Authorization: Bearer $TOKEN"

STOCK="${1:-000001}"
DATE="${2:-$(date +%Y-%m-%d)}"
echo "  股票: $STOCK  日期: $DATE"

# Get all indicators
INDICATORS=$(curl -s "$BASE_URL/api/v1/strategies/indicators" -H "$AUTH" | python3 -c "
import sys, json
data = json.load(sys.stdin).get('data', [])
for ind in data:
    print(ind['key'])
")

TOTAL=$(echo "$INDICATORS" | wc -l | tr -d ' ')
PASS=0
NODATA=0
ZERO=0
FAIL_KEYS=""

echo ""
echo "╔══════════════════════════════════════════════════════════════════════╗"
printf "║  Stock: %-6s  Date: %-10s  Total: %-3s indicators      ║\n" "$STOCK" "$DATE" "$TOTAL"
echo "╠══════════════════════════════════════════════════════════════════════╣"

for IND in $INDICATORS; do
  RESULT=$(curl -s -X POST "$BASE_URL/api/v1/strategies/test-indicator" \
    -H "$AUTH" -H "Content-Type: application/json" \
    -d "{\"stockCode\":\"$STOCK\",\"date\":\"$DATE\",\"indicator\":\"$IND\",\"operator\":\"gte\",\"value\":0}" 2>/dev/null)
  
  HAS_DATA=$(echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print('true' if d.get('data',{}).get('hasData') else 'false')" 2>/dev/null)
  VALUE=$(echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); v=d.get('data',{}).get('computedValue','N/A'); print(v)" 2>/dev/null)
  
  if [ "$HAS_DATA" = "true" ]; then
    if [ "$VALUE" = "0" ] || [ "$VALUE" = "0.0" ]; then
      ZERO=$((ZERO + 1))
      printf "  ⚡ %-30s → %s (zero)\n" "$IND" "$VALUE"
    else
      PASS=$((PASS + 1))
      printf "  ✅ %-30s → %s\n" "$IND" "$VALUE"
    fi
  else
    NODATA=$((NODATA + 1))
    FAIL_KEYS="$FAIL_KEYS $IND"
    printf "  ❌ %-30s → NO DATA\n" "$IND"
  fi
done

echo "╠══════════════════════════════════════════════════════════════════════╣"
printf "║  ✅ Pass: %-3d  ⚡ Zero: %-3d  ❌ NoData: %-3d                   ║\n" $PASS $ZERO $NODATA
echo "╚══════════════════════════════════════════════════════════════════════╝"

if [ $NODATA -gt 0 ]; then
  echo ""
  echo "⚠️  无数据指标:"
  for k in $FAIL_KEYS; do echo "    - $k"; done
fi
