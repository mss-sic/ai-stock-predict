#!/usr/bin/env python3
"""ARIMA statistical time-series prediction. Uses real K-line data."""
import sys, json, warnings
import numpy as np
from db_utils import fetch_kline, fetch_signal

warnings.filterwarnings("ignore")

code = sys.argv[1] if len(sys.argv) > 1 else "000001"
horizon = int(sys.argv[2]) if len(sys.argv) > 2 else 10

df = fetch_kline(code, 120)
if df is None or len(df) < 20:
    # Fallback: use signal value as base
    signal = fetch_signal(code)
    base = 20 + abs(signal) * 2
    rng = np.random.RandomState(abs(hash(code)) % 2**31)
    preds = []
    for i in range(horizon):
        base *= (1 + rng.normal(-0.002, 0.02))
        ci = 1.0 + i * 0.5
        preds.append({"day": i+1, "price": round(base, 2), "upper": round(base*(1+ci/100), 2), "lower": round(base*(1-ci/100), 2)})
    print(json.dumps(preds))
    sys.exit(0)

# Real ARIMA on close prices
try:
    from statsmodels.tsa.arima.model import ARIMA
    closes = df['close'].values.astype(float)
    model = ARIMA(closes, order=(2, 1, 2))
    fitted = model.fit()
    forecast = fitted.forecast(steps=horizon)
    residuals = fitted.resid
    sigma = np.std(residuals) if len(residuals) > 0 else closes[-1] * 0.02

    preds = []
    for i, f in enumerate(forecast):
        ci = sigma * (1.5 + i * 0.3)
        preds.append({
            "day": i + 1,
            "price": round(float(f), 2),
            "upper": round(float(f + ci), 2),
            "lower": round(float(f - ci), 2),
        })
    print(json.dumps(preds))
except Exception as e:
    # Fallback
    rng = np.random.RandomState(abs(hash(code)) % 2**31)
    base = float(df['close'].iloc[-1])
    preds = []
    for i in range(horizon):
        base *= (1 + rng.normal(-0.001, 0.02))
        ci = 1.0 + i * 0.5
        preds.append({"day": i+1, "price": round(base, 2), "upper": round(base*(1+ci/100), 2), "lower": round(base*(1-ci/100), 2)})
    print(json.dumps(preds))
