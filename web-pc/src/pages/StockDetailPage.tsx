import { useEffect, useState, useMemo } from 'react';
import { useParams } from 'react-router-dom';
import { ArrowUp, ArrowDown, TrendingUp, TrendingDown } from 'lucide-react';
import { fetchStockDetail, fetchKLine, fetchIndicator } from '../services/api';
import KLineChart from '../components/KLineChart';

export default function StockDetailPage() {
  const { code } = useParams<{ code: string }>();
  const [stock, setStock] = useState<any>(null);
  const [klines, setKlines] = useState<any[]>([]);
  const [indicator, setIndicator] = useState<any>(null);
  const [view, setView] = useState<'kline' | 'intraday'>('kline');

  useEffect(() => {
    if (!code) return;
    fetchStockDetail(code).then((r: any) => setStock(r.data));
    fetchKLine(code).then((r: any) => setKlines(r.data || []));
    fetchIndicator(code).then((r: any) => setIndicator(r.data)).catch(() => {});
  }, [code]);

  const priceStats = useMemo(() => {
    if (!klines.length) return null;
    const latest = klines[klines.length - 1];
    const prev = klines.length > 1 ? klines[klines.length - 2] : latest;
    const chg = latest.close - prev.close;
    const chgPct = prev.close ? (chg / prev.close) * 100 : 0;
    const high = Math.max(...klines.slice(-20).map((k: any) => k.high));
    const low = Math.min(...klines.slice(-20).map((k: any) => k.low));
    const avgVol = klines.slice(-20).reduce((s: number, k: any) => s + (k.volume || 0), 0) / 20;
    return { price: latest.close, chg, chgPct, high, low, avgVol, prevClose: prev.close, open: latest.open };
  }, [klines]);

  if (!stock) return <div style={{ textAlign: 'center', padding: 60, color: 'var(--color-text-3)' }}>加载中...</div>;

  const isUp = (priceStats?.chg ?? 0) >= 0;

  return (
    <div>
      {/* Header with big price */}
      <div className="page-header">
        <div>
          <h2>{stock.name} <span className="muted" style={{ fontSize: 14, fontWeight: 400 }}>{stock.code}</span></h2>
          <span className="muted">{stock.industry || '—'}</span>
        </div>
      </div>

      {priceStats && (
        <div className="card mb16">
          <div className="card-body">
            <div className="price-hero">
              <span className={`price-num ${isUp ? 'up' : 'down'}`}>¥{priceStats.price.toFixed(2)}</span>
              <span>
                <span className={`price-chg ${isUp ? 'up' : 'down'}`}>
                  {isUp ? <ArrowUp size={18} style={{ verticalAlign: 'middle', marginRight: 2 }} /> : <ArrowDown size={18} style={{ verticalAlign: 'middle', marginRight: 2 }} />}
                  {isUp ? '+' : ''}{priceStats.chg.toFixed(2)}
                </span>
                <span className={`price-chg ${isUp ? 'up' : 'down'}`} style={{ marginLeft: 8 }}>
                  ({isUp ? '+' : ''}{priceStats.chgPct.toFixed(2)}%)
                </span>
              </span>
            </div>
            <div className="row gap16 mt16">
              <span className="muted">昨收 <b>{priceStats.prevClose.toFixed(2)}</b></span>
              <span className="muted">开盘 <b>{priceStats.open.toFixed(2)}</b></span>
              <span className="muted">最高 <b className="up">{priceStats.high.toFixed(2)}</b></span>
              <span className="muted">最低 <b className="down">{priceStats.low.toFixed(2)}</b></span>
              <span className="muted">均量 <b>{(priceStats.avgVol / 10000).toFixed(0)}万</b></span>
            </div>
            {indicator && (
              <div className="row gap16 mt12">
                <span className="muted">市盈率 <b>{indicator.pe > 0 ? indicator.pe.toFixed(2) : '—'}</b></span>
                <span className="muted">市净率 <b>{indicator.pb > 0 ? indicator.pb.toFixed(2) : '—'}</b></span>
                <span className="muted">总市值 <b>{indicator.totalMarketCap > 0 ? (indicator.totalMarketCap / 1e8).toFixed(0) + '亿' : '—'}</b></span>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Chart */}
      <div className="card mb16">
        <div className="card-header">
          <div className="seg">
            <button className={view === 'kline' ? 'active' : ''} onClick={() => setView('kline')}>
              <TrendingUp size={13} style={{ marginRight: 4 }} />K线图
            </button>
            <button className={view === 'intraday' ? 'active' : ''} onClick={() => setView('intraday')}>
              <TrendingDown size={13} style={{ marginRight: 4 }} />分时图
            </button>
          </div>
        </div>
        <div className="card-body">
          {view === 'kline' ? (
            <KLineChart data={klines} />
          ) : (
            <div className="muted" style={{ textAlign: 'center', padding: 60 }}>分时图数据需额外采集，功能完善中</div>
          )}
        </div>
      </div>

      {/* Stats grid */}
      <div className="stat-grid">
        <div className="stat-card">
          <div className="stat-label">行业</div>
          <div className="stat-value" style={{ fontSize: 15 }}>{stock.industry || '—'}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">20日最高</div>
          <div className="stat-value up">{priceStats?.high.toFixed(2) || '—'}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">20日最低</div>
          <div className="stat-value down">{priceStats?.low.toFixed(2) || '—'}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">总股本</div>
          <div className="stat-value">{stock.totalShares ? (stock.totalShares / 1e8).toFixed(2) + '亿' : '—'}</div>
        </div>
      </div>
    </div>
  );
}
