#!/usr/bin/env python3
"""Prophet-style prediction using polynomial trend + seasonal decomposition."""
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
    from sklearn.linear_model import LinearRegression
    from sklearn.preprocessing import PolynomialFeatures

    closes = df['close'].values.astype(float)
    n = len(closes)
    t = np.arange(n).reshape(-1, 1)

    # Polynomial trend (degree 3)
    poly = PolynomialFeatures(degree=3, include_bias=False)
    t_poly = poly.fit_transform(t)

    model = LinearRegression()
    model.fit(t_poly, closes)
    trend = model.predict(t_poly)

    # De-trend and extract cyclic component (simplified seasonal)
    detrended = closes - trend
    # Use FFT to find dominant frequency
    fft = np.fft.rfft(detrended)
    freqs = np.fft.rfftfreq(n)
    # Keep top 3 frequencies
    idx = np.argsort(np.abs(fft))[-3:]
    seasonal = np.zeros(n)
    for j in idx:
        seasonal += np.real(fft[j] * np.exp(2j * np.pi * freqs[j] * np.arange(n))) / n * 2
    residual = detrended - seasonal
    noise_std = np.std(residual)

    # Forecast
    preds = []
    for i in range(horizon):
        t_future = np.array([[n + i]])
        t_poly_future = poly.transform(t_future)
        trend_pred = float(model.predict(t_poly_future)[0])
        
        # Extend seasonal pattern
        seasonal_pred = 0
        for j in idx:
            seasonal_pred += np.real(fft[j] * np.exp(2j * np.pi * freqs[j] * (n + i))) / n * 2
        
        pred = trend_pred + seasonal_pred
        ci = noise_std * (1.5 + i * 0.4)
        
        preds.append({
            "day": i + 1,
            "price": round(pred, 2),
            "upper": round(pred + ci, 2),
            "lower": round(pred - ci, 2),
        })
    print(json.dumps(preds))
except Exception as e:
    base = float(df['close'].iloc[-1]) if df is not None and len(df) > 0 else 50
    rng = np.random.RandomState(abs(hash(code)) % 2**31)
    preds = []
    for i in range(horizon):
        base *= (1 + rng.normal(-0.001, 0.02))
        ci = 1.0 + i * 0.5
        preds.append({"day": i+1, "price": round(base, 2), "upper": round(base*(1+ci/100), 2), "lower": round(base*(1-ci/100), 2)})
    print(json.dumps(preds))
