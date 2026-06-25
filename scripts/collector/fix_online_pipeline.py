#!/usr/bin/env python3
"""
线上市场情绪 + 市场风格数据修复脚本

按依赖顺序执行:
  1. precompute_aggs.py --last 60   (市场日聚合)
  2. compute_sentiment.py --last 60 (市场情绪)
  3. 调用 BulkCompute API 回填 market_style_daily 全量历史

用法 (在 Docker 容器内):
  docker exec ai-stock-predict-app-1 python3 scripts/collector/fix_online_pipeline.py

或直接:
  cd /opt/ai-stock-predict && python3 scripts/collector/fix_online_pipeline.py
"""
import os, sys, time, subprocess, json, urllib.request

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
API_BASE = os.environ.get("API_BASE", "http://localhost:8080/api/v1")
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))

def run_step(name, script, *args):
    print(f"\n{'='*60}")
    print(f"  STEP: {name}")
    print(f"{'='*60}")
    path = os.path.join(SCRIPT_DIR, script)
    cmd = ["python3", path] + list(args)
    print(f"  CMD: {' '.join(cmd)}")
    t0 = time.time()
    result = subprocess.run(cmd, env={**os.environ, "PYTHONUNBUFFERED": "1"})
    elapsed = time.time() - t0
    if result.returncode == 0:
        print(f"  ✅ {name} 完成 ({elapsed:.1f}s)")
    else:
        print(f"  ❌ {name} 失败 (exit={result.returncode}, {elapsed:.1f}s)")
        return False
    return True

def call_bulk_compute():
    """调用 BulkCompute API 回填全量市场风格"""
    print(f"\n{'='*60}")
    print(f"  STEP: 市场风格全量回填 (BulkCompute API)")
    print(f"{'='*60}")
    
    # Login
    login_url = f"{API_BASE}/auth/login"
    login_data = json.dumps({"username": "admin", "password": "admin123"}).encode()
    login_req = urllib.request.Request(login_url, data=login_data,
        headers={'Content-Type': 'application/json'}, method='POST')
    try:
        login_resp = urllib.request.urlopen(login_req, timeout=10)
        token = json.loads(login_resp.read()).get('data', {}).get('accessToken', '')
        if not token:
            print("  ❌ 登录失败，无法获取 token")
            return False
    except Exception as e:
        print(f"  ❌ 登录失败: {e}")
        return False
    
    # Bulk compute
    t0 = time.time()
    url = f"{API_BASE}/market/bulk-compute"
    req = urllib.request.Request(url, method='POST')
    req.add_header('Authorization', f'Bearer {token}')
    req.add_header('Content-Type', 'application/json')
    try:
        resp = urllib.request.urlopen(req, timeout=600)  # 10 min timeout for bulk
        body = json.loads(resp.read())
        elapsed = time.time() - t0
        if body.get('code') == 0:
            result = body.get('data', {})
            print(f"  ✅ 市场风格回填完成: {result.get('computed', 0)} 天 ({elapsed:.1f}s)")
            return True
        else:
            print(f"  ❌ API 返回错误: {body.get('message', 'unknown')}")
            return False
    except Exception as e:
        print(f"  ❌ API 调用失败: {e}")
        return False

def main():
    print("=" * 60)
    print("  线上市场数据修复脚本")
    print(f"  DB: {PG_DSN.split(' ')[0]}")
    print(f"  API: {API_BASE}")
    print("=" * 60)
    
    # Step 1: precompute_aggs (market_daily_agg)
    if not run_step("市场日聚合", "precompute_aggs.py", "--last", "60"):
        print("\n⚠️  日聚合失败，后续步骤可能缺少数据")
    
    # Step 2: compute_sentiment (market_sentiment)  
    if not run_step("市场情绪计算", "compute_sentiment.py", "--last", "60"):
        print("\n⚠️  情绪计算失败")
    
    # Step 3: bulk market style backfill
    call_bulk_compute()
    
    print(f"\n{'='*60}")
    print("  修复完成")
    print(f"{'='*60}")

if __name__ == '__main__':
    main()
