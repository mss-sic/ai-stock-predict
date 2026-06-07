#!/usr/bin/env python3
"""
一键回填所有历史数据
执行顺序: 财务 → 股东 → PE/PB指标
"""
import os, sys, time, subprocess

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))

def run_step(name, script, extra_args=None):
    print(f"\n{'='*60}")
    print(f"  📦 Step: {name}")
    print(f"{'='*60}")
    start = time.time()
    args = [sys.executable, os.path.join(SCRIPT_DIR, script)]
    if extra_args:
        args.extend(extra_args)
    result = subprocess.run(args)
    elapsed = time.time() - start
    status = "✅" if result.returncode == 0 else "❌"
    print(f"\n{status} {name}: {elapsed:.0f}s (exit {result.returncode})")
    return result.returncode == 0

def main():
    print("""
╔══════════════════════════════════════════╗
║    📊 全量历史数据回填                    ║
║    财务 → 股东 → PE/PB指标               ║
╚══════════════════════════════════════════╝
    """)
    
    # Parse args
    start_date = sys.argv[1] if len(sys.argv) > 1 else '2024-01-01'
    end_date = sys.argv[2] if len(sys.argv) > 2 else None
    
    # Step 1: Financial data (runs first, needed for PE/PB computation)
    if run_step("财务数据全量回填", "backfill_financial.py"):
        print("  ✅ 财报数据就绪")
    else:
        print("  ⚠️ 财报回填有错误，但继续执行")
    
    # Step 2: Shareholder data
    run_step("股东数据全量回填", "backfill_shareholder.py")
    
    # Step 3: PE/PB indicator computation
    extra = [start_date]
    if end_date:
        extra.append(end_date)
    run_step("PE/PB指标计算", "backfill_indicator.py", extra)
    
    print(f"\n{'='*60}")
    print(f"  🎉 全量回填完成!")
    print(f"{'='*60}")

if __name__ == "__main__":
    main()
