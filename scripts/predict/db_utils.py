"""Shared PostgreSQL utilities for prediction scripts."""
import os
import psycopg2
import pandas as pd
import numpy as np

DSN = os.environ.get("POSTGRES_DSN", "host=localhost user=stock password=stock123 dbname=stock_predict port=5432 sslmode=disable")

def get_connection():
    return psycopg2.connect(DSN)

def fetch_kline(code, days=120):
    """Fetch OHLCV data for a stock, most recent `days` rows."""
    conn = get_connection()
    df = pd.read_sql_query("""
        SELECT trade_date, open, high, low, close, volume
        FROM stocks_daily_k
        WHERE code = %s
        ORDER BY trade_date ASC
    """, conn, params=(code,))
    conn.close()
    if df.empty:
        return None
    df['trade_date'] = pd.to_datetime(df['trade_date'])
    df.set_index('trade_date', inplace=True)
    # Take last N days
    return df.tail(days).copy()

def fetch_signal(code):
    """Fetch the algorithm signal value for a stock."""
    conn = get_connection()
    cur = conn.cursor()
    cur.execute("SELECT signal_value FROM stock_signals WHERE code = %s", (code,))
    row = cur.fetchone()
    conn.close()
    return float(row[0]) if row else 0.0
