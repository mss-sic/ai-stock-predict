import os
# Fix macOS Homebrew Python 3.12 expat incompatibility
os.environ["DYLD_LIBRARY_PATH"] = "/opt/homebrew/opt/expat/lib:/opt/homebrew/opt/openssl@3/lib:" + os.environ.get("DYLD_LIBRARY_PATH", "")

import sys, time, datetime, threading, io, requests, pandas as pd
from pathlib import Path
from dotenv import load_dotenv

load_dotenv(Path(__file__).parent / ".env")

os.environ.setdefault("TRADINGAGENTS_LLM_PROVIDER", "deepseek")
os.environ.setdefault("TRADINGAGENTS_DEEP_THINK_LLM", "deepseek-reasoner")
os.environ.setdefault("TRADINGAGENTS_QUICK_THINK_LLM", "deepseek-chat")
os.environ.setdefault("TRADINGAGENTS_OUTPUT_LANGUAGE", "Chinese")
os.environ.setdefault("TRADINGAGENTS_MAX_DEBATE_ROUNDS", "1")
os.environ.setdefault("TRADINGAGENTS_MAX_RISK_ROUNDS", "1")
AV_KEY = "91TUIENFY57E1N0C"

# ===== PATCH 1: Throttle Alpha Vantage API calls (free tier: 5/min) =====
import tradingagents.dataflows.alpha_vantage_common as avc
_orig_av = avc._make_api_request
_last_av = 0
_av_lock = threading.Lock()
def _throttled_av(fn, params):
    global _last_av
    with _av_lock:
        w = 13.0 - (time.time() - _last_av)
        if w > 0: time.sleep(w)
        r = _orig_av(fn, params)
        _last_av = time.time()
        return r
avc._make_api_request = _throttled_av

# ===== PATCH 2: Free-tier TIME_SERIES_DAILY instead of premium =====
import tradingagents.dataflows.alpha_vantage_stock as avs
_orig_get_stock = avs.get_stock
def _free_get_stock(symbol, start_date, end_date):
    from tradingagents.dataflows.alpha_vantage_common import _make_api_request, _filter_csv_by_date_range
    params = {"symbol": symbol, "outputsize": "compact", "datatype": "csv"}
    resp = _make_api_request("TIME_SERIES_DAILY", params)
    return _filter_csv_by_date_range(resp, start_date, end_date)
avs.get_stock = _free_get_stock

# ===== PATCH 3: Pre-seed yfinance cache with correct filename =====
from tradingagents.dataflows.config import get_config
from tradingagents.dataflows.stockstats_utils import normalize_symbol, safe_ticker_component

TICKER = sys.argv[1] if len(sys.argv) > 1 else "TSLA"
config = get_config()
canonical = normalize_symbol(TICKER)
safe = safe_ticker_component(canonical)
today = pd.Timestamp.today()
cache_file = os.path.join(config["data_cache_dir"],
    f"{safe}-YFin-data-{(today-pd.DateOffset(years=5)).strftime('%Y-%m-%d')}-{(today+pd.Timedelta(days=1)).strftime('%Y-%m-%d')}.csv")

if not os.path.exists(cache_file):
    os.makedirs(config["data_cache_dir"], exist_ok=True)
    r = requests.get("https://www.alphavantage.co/query", timeout=30, params={
        "function": "TIME_SERIES_DAILY", "symbol": TICKER,
        "outputsize": "compact", "datatype": "csv", "apikey": AV_KEY,
    })
    df = pd.read_csv(io.StringIO(r.text))
    df = df.rename(columns={"timestamp":"Date","open":"Open","high":"High","low":"Low","close":"Close","volume":"Volume"})
    df["Date"] = pd.to_datetime(df["Date"])
    df.to_csv(cache_file, index=False, encoding="utf-8")
    print(f"[cache] {len(df)} rows for {TICKER}")
else:
    print(f"[cache] using existing cache for {TICKER}")

# ===== PATCH 4: All data vendors -> Alpha Vantage =====
from tradingagents.default_config import DEFAULT_CONFIG
for k in DEFAULT_CONFIG["data_vendors"]:
    if k in ("macro_data", "prediction_markets"):
        continue
    if DEFAULT_CONFIG["data_vendors"][k] == "yfinance":
        DEFAULT_CONFIG["data_vendors"][k] = "alpha_vantage"

from tradingagents.graph.trading_graph import TradingAgentsGraph

ANALYSIS_DATE = sys.argv[2] if len(sys.argv) > 2 else datetime.datetime.now().strftime("%Y-%m-%d")
print(f"🚀 {TICKER} | {ANALYSIS_DATE} | DeepSeek | Alpha Vantage\n")

results_dir = Path(DEFAULT_CONFIG["results_dir"]) / TICKER / ANALYSIS_DATE
results_dir.mkdir(parents=True, exist_ok=True)

# All 4 analysts
analyst_keys = ["market", "social", "news", "fundamentals"]
print(f"🔧 Building graph ({len(analyst_keys)} analysts)...")
graph = TradingAgentsGraph(analyst_keys, config=DEFAULT_CONFIG, debug=True)

print("⏳ Running full analysis...\n")
start = time.time()

try:
    final_state = graph.propagate(TICKER, ANALYSIS_DATE, asset_type="stock")
    elapsed = time.time() - start

    # graph.propagate() returns a tuple (final_state_dict, ticker)
    if isinstance(final_state, tuple):
        final_state, _ = final_state

    print(f"\n{'='*60}")
    print(f"✅ Done in {elapsed:.0f}s")
    print(f"📁 Results: {results_dir}")

    for key in ["market_report", "sentiment_report", "news_report", "fundamentals_report"]:
        val = final_state.get(key, "")
        if val:
            print(f"\n{'='*60}\n  {key.upper()}\n{'='*60}")
            print(str(val)[:1500])
            print("...(truncated)")
except Exception as e:
    elapsed = time.time() - start
    print(f"\n❌ Error after {elapsed:.0f}s: {e}")
    import traceback
    traceback.print_exc()
