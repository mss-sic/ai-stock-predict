import React, { useState } from 'react';
import { DollarSign, Repeat, TrendingUp, TrendingDown, Minus } from 'lucide-react';

interface StockFundFlowTabProps {
  code: string;
  fundFlow: any[];
  safeKlines: any[];
  refreshingPhase: string;
  refreshLogs: string[];
  handleRefreshStockData: (phase: string) => void;
}

const StockFundFlowTab: React.FC<StockFundFlowTabProps> = ({ code, fundFlow, safeKlines, refreshingPhase, refreshLogs, handleRefreshStockData }) => {
  const [activePeriod, setActivePeriod] = useState(20);

  const flow = [...fundFlow].reverse().slice(-activePeriod);

  const fmtVol = (v: number) => {
    if (!v) return '—';
    const abs = Math.abs(v);
    if (abs >= 1e8) return (v / 1e8).toFixed(2) + '亿';
    if (abs >= 1e4) return (v / 1e4).toFixed(0) + '万';
    return v.toFixed(0);
  };

  const latest = flow.length > 0 ? flow[flow.length - 1] : null;
  const netFlow = latest?.netFlow || 0;
  const netRatio = latest?.netFlowRatio || 0;
  const flowDir = netFlow > 0 ? 'inflow' : netFlow < 0 ? 'outflow' : 'neutral';

  // Compute simple stats from displayed flow
  const totalNet = flow.reduce((s: number, d: any) => s + (d.netFlow || 0), 0);
  const avgRatio = flow.length > 0 ? flow.reduce((s: number, d: any) => s + (d.netFlowRatio || 0), 0) / flow.length : 0;
  const maxNet = flow.length > 0 ? Math.max(...flow.map((d: any) => Math.abs(d.netFlow || 0))) : 1;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* 今日资金快照 */}
      <div className="card">
        <div className="card-header">
          <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <DollarSign size={16} color="var(--color-primary)" />
            <span style={{ fontWeight: 600, fontSize: 14 }}>资金流向（内外盘）</span>
            <span style={{ fontSize: 11, color: 'var(--color-text-3)', marginLeft: 4 }}>
              {flowDir === 'inflow' ? '· 主动买入占优' : flowDir === 'outflow' ? '· 主动卖出占优' : '· 买卖均衡'}
            </span>
          </span>
          <button onClick={() => handleRefreshStockData('fund_flow')} disabled={refreshingPhase !== ''}
            style={{ padding: '4px 10px', border: '1px solid var(--color-border-2)', borderRadius: 6, background: 'var(--color-bg-1)', cursor: 'pointer', fontSize: 12, display: 'flex', alignItems: 'center', gap: 4 }}>
            <Repeat size={12} className={refreshingPhase === 'fund_flow' ? 'spin' : ''} />
            {refreshingPhase === 'fund_flow' ? '更新中...' : '更新'}
          </button>
        </div>
        <div className="card-body" style={{ padding: '16px 20px' }}>
          {!latest ? (
            <div style={{ padding: 30, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>
              暂无资金流数据，需等待当日K线采集完成后自动生成
            </div>
          ) : (
            <div style={{ display: 'flex', gap: 24, alignItems: 'center', flexWrap: 'wrap' }}>
              {/* Net flow indicator */}
              <div style={{
                width: 80, height: 80, borderRadius: '50%',
                background: flowDir === 'inflow' ? 'linear-gradient(135deg, #F53F3F20, #F53F3F40)' : flowDir === 'outflow' ? 'linear-gradient(135deg, #00B42A20, #00B42A40)' : 'var(--color-fill-2)',
                display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0,
              }}>
                <div style={{ textAlign: 'center' }}>
                  {flowDir === 'inflow' ? <TrendingUp size={24} color="#F53F3F" /> :
                   flowDir === 'outflow' ? <TrendingDown size={24} color="#00B42A" /> :
                   <Minus size={24} color="var(--color-text-3)" />}
                  <div style={{ fontSize: 11, fontWeight: 600, color: flowDir === 'inflow' ? '#F53F3F' : flowDir === 'outflow' ? '#00B42A' : 'var(--color-text-2)', marginTop: 2 }}>
                    {netRatio > 0 ? '+' : ''}{netRatio.toFixed(1)}%
                  </div>
                </div>
              </div>
              {/* Stats */}
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '8px 24px' }}>
                <div><span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>外盘（主动买）</span>
                  <div style={{ fontSize: 18, fontWeight: 700, color: '#F53F3F' }}>{fmtVol(latest?.buyVol || 0)}</div></div>
                <div><span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>内盘（主动卖）</span>
                  <div style={{ fontSize: 18, fontWeight: 700, color: '#00B42A' }}>{fmtVol(latest?.sellVol || 0)}</div></div>
                <div><span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>净流向</span>
                  <div style={{ fontSize: 18, fontWeight: 700, color: netFlow > 0 ? '#F53F3F' : netFlow < 0 ? '#00B42A' : 'var(--color-text-2)' }}>
                    {netFlow > 0 ? '+' : ''}{fmtVol(netFlow)}</div></div>
                <div><span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>净流比</span>
                  <div style={{ fontSize: 18, fontWeight: 700, color: netRatio > 0 ? '#F53F3F' : netRatio < 0 ? '#00B42A' : 'var(--color-text-2)' }}>
                    {netRatio > 0 ? '+' : ''}{netRatio.toFixed(1)}%</div></div>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* 趋势图（简易柱状） */}
      {flow.length > 0 && (
        <div className="card">
          <div className="card-header">
            <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <span style={{ fontWeight: 600, fontSize: 13 }}>净流向趋势</span>
            </span>
            <div style={{ display: 'flex', gap: 4 }}>
              {[5, 10, 20, 30].map(n => (
                <button key={n} onClick={() => setActivePeriod(n)}
                  style={{
                    padding: '2px 8px', border: '1px solid var(--color-border-2)', borderRadius: 4,
                    background: activePeriod === n ? 'var(--color-primary)' : 'var(--color-bg-1)',
                    color: activePeriod === n ? '#fff' : 'var(--color-text-2)', fontSize: 11, cursor: 'pointer'
                  }}>{n}日</button>
              ))}
            </div>
          </div>
          <div className="card-body" style={{ padding: '12px 20px' }}>
            <div style={{ display: 'flex', alignItems: 'flex-end', gap: 2, height: 120, paddingTop: 20 }}>
              {flow.map((d: any, i: number) => {
                const h = maxNet > 0 ? Math.abs(d.netFlow || 0) / maxNet * 80 : 0;
                return (
                  <div key={i} style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 2 }}>
                    <div style={{
                      width: '70%', height: Math.max(h, 2),
                      background: (d.netFlow || 0) >= 0 ? '#F53F3F' : '#00B42A',
                      borderRadius: '2px 2px 0 0', opacity: 0.8
                    }} title={`${d.tradeDate}: ${fmtVol(d.netFlow)} (${d.netFlowRatio?.toFixed(1)}%)`} />
                    {flow.length <= 20 && (
                      <span style={{ fontSize: 9, color: 'var(--color-text-3)', transform: 'rotate(-45deg)', marginTop: 4, whiteSpace: 'nowrap' }}>
                        {(d.tradeDate || '').slice(5)}
                      </span>
                    )}
                  </div>
                );
              })}
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 8, fontSize: 11, color: 'var(--color-text-3)' }}>
              <span>近{activePeriod}日累计: {totalNet > 0 ? '+' : ''}{fmtVol(totalNet)} | 均比: {avgRatio > 0 ? '+' : ''}{avgRatio.toFixed(1)}%</span>
            </div>
          </div>
        </div>
      )}

      {/* 明细表 */}
      {flow.length > 0 && (
        <div className="card">
          <div className="card-header">
            <span style={{ fontWeight: 600, fontSize: 13 }}>资金流明细</span>
          </div>
          <div className="card-body" style={{ padding: '8px 16px' }}>
            <table style={{ width: '100%', fontSize: 12, borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ borderBottom: '2px solid var(--color-border-2)', color: 'var(--color-text-2)' }}>
                  <th style={{ padding: '6px 8px', textAlign: 'left' }}>日期</th>
                  <th style={{ padding: '6px 8px', textAlign: 'right' }}>外盘</th>
                  <th style={{ padding: '6px 8px', textAlign: 'right' }}>内盘</th>
                  <th style={{ padding: '6px 8px', textAlign: 'right' }}>净流向</th>
                  <th style={{ padding: '6px 8px', textAlign: 'right' }}>净流比</th>
                </tr>
              </thead>
              <tbody>
                {flow.map((d: any, i: number) => (
                  <tr key={i} style={{ borderBottom: '1px solid var(--color-border-1)' }}>
                    <td style={{ padding: '5px 8px' }}>{d.tradeDate}</td>
                    <td style={{ padding: '5px 8px', textAlign: 'right', fontFamily: 'monospace', color: '#F53F3F' }}>{fmtVol(d.buyVol || 0)}</td>
                    <td style={{ padding: '5px 8px', textAlign: 'right', fontFamily: 'monospace', color: '#00B42A' }}>{fmtVol(d.sellVol || 0)}</td>
                    <td style={{ padding: '5px 8px', textAlign: 'right', fontFamily: 'monospace', color: (d.netFlow || 0) >= 0 ? '#F53F3F' : '#00B42A', fontWeight: 500 }}>
                      {(d.netFlow || 0) > 0 ? '+' : ''}{fmtVol(d.netFlow || 0)}</td>
                    <td style={{ padding: '5px 8px', textAlign: 'right', fontFamily: 'monospace', color: (d.netFlowRatio || 0) >= 0 ? '#F53F3F' : '#00B42A' }}>
                      {(d.netFlowRatio || 0) > 0 ? '+' : ''}{(d.netFlowRatio || 0).toFixed(1)}%</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
};

export default StockFundFlowTab;
