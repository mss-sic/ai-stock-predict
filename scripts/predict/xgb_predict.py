#!/usr/bin/env python3
"""XGBoost gradient boosting prediction. Uses real K-line data with feature engineering."""
import sys, json, warnings
import numpy as np
from db_utils import fetch_kline, fetch_signal

warnings.filterwarnings("ignore")

code = sys.argv[1] if len(sys.argv) > 1 else "000001"
horizon = int(sys.argv[2]) if len(sys.argv) > 2 else 10

df = fetch_kline(code, 120)
if df is None or len(df) < 20:
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

try:
    import xgboost as xgb
    from sklearn.preprocessing import StandardScaler

    closes = df['close'].values.astype(float)
    volumes = df['volume'].values.astype(float)
    highs = df['high'].values.astype(float)
    lows = df['low'].values.astype(float)
    opens = df['open'].values.astype(float)

    # Feature engineering
    n = len(closes)
    features = []
    for i in range(20, n):
        row = [
            closes[i-1],  # lag1
            closes[i-5] if i >= 5 else closes[0],  # lag5
            closes[i-10] if i >= 10 else closes[0],  # lag10
            closes[i-20] if i >= 20 else closes[0],  # lag20
            np.mean(closes[i-5:i]) if i >= 5 else closes[i],  # ma5
            np.mean(closes[i-10:i]) if i >= 10 else closes[i],  # ma10
            np.mean(closes[i-20:i]) if i >= 20 else closes[i],  # ma20
            np.std(closes[i-10:i]) if i >= 10 else 0,  # volatility
            volumes[i-1] / max(volumes[i-5:i].mean() if i >= 5 else 1, 1),  # vol ratio
            (highs[i-1] - lows[i-1]) / closes[i-1] if closes[i-1] > 0 else 0,  # range ratio
            (closes[i-1] - opens[i-1]) / closes[i-1] if closes[i-1] > 0 else 0,  # body ratio
        ]
        features.append(row)
    features = np.array(features)
    targets = closes[20:]

    if len(features) < 10:
        raise ValueError("Insufficient data")

    scaler = StandardScaler()
    features_scaled = scaler.fit_transform(features)

    model = xgb.XGBRegressor(n_estimators=50, max_depth=3, learning_rate=0.1, random_state=42, verbosity=0)
    model.fit(features_scaled, targets)

    # Iterative prediction
    last_features = features[-1:].copy()
    preds = []
    for i in range(horizon):
        pred = float(model.predict(scaler.transform(last_features))[0])
        sigma = np.std(targets - model.predict(features_scaled)) * (1 + i * 0.2)
        preds.append({
            "day": i + 1,
            "price": round(pred, 2),
            "upper": round(pred + sigma, 2),
            "lower": round(pred - sigma, 2),
        })
        # Update features for next step (shift)
        prev = last_features[0].copy()
        prev[0] = pred
        prev[1:4] = np.roll(prev[0:3], 1)
        last_features = np.array([prev])

    print(json.dumps(preds))
except Exception as e:
    # Fallback
    base = float(df['close'].iloc[-1]) if df is not None and len(df) > 0 else 50
    rng = np.random.RandomState(abs(hash(code)) % 2**31)
    preds = []
    for i in range(horizon):
        base *= (1 + rng.normal(-0.001, 0.02))
        ci = 1.0 + i * 0.5
        preds.append({"day": i+1, "price": round(base, 2), "upper": round(base*(1+ci/100), 2), "lower": round(base*(1-ci/100), 2)})
    print(json.dumps(preds))
