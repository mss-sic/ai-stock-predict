import { useState, useEffect } from 'react';
import { Table } from '@arco-design/web-react';
import { Briefcase, ArrowUp, ArrowDown } from 'lucide-react';

interface Holding {
  id: number; code: string; name: string; cost: number; qty: number;
  price: number; stopProfit: number; stopLoss: number;
  pnl: number; pnlPct: number; signal: string;
}

const initial: Holding[] = [
  { id: 1, code: '600519', name: '贵州茅台', cost: 1850, qty: 100, price: 1876, stopProfit: 1950, stopLoss: 1780, pnl: 2600, pnlPct: 1.4, signal: '持有' },
  { id: 2, code: '000858', name: '五 粮 液', cost: 152, qty: 500, price: 148.5, stopProfit: 162, stopLoss: 145, pnl: -1750, pnlPct: -2.3, signal: '持有' },
  { id: 3, code: '300750', name: '宁德时代', cost: 210, qty: 200, price: 218.3, stopProfit: 230, stopLoss: 200, pnl: 1660, pnlPct: 3.95, signal: '关注止盈' },
  { id: 4, code: '688981', name: '中芯国际', cost: 55.2, qty: 800, price: 58.1, stopProfit: 62, stopLoss: 52, pnl: 2320, pnlPct: 5.25, signal: '持有' },
];

export default function HoldingsPage() {
  const [data, setData] = useState<Holding[]>(initial);

  useEffect(() => {
    const timer = setInterval(() => {
      setData(prev => prev.map(h => {
        const jitter = (Math.random() - 0.48) * h.price * 0.005;
        const newPrice = +(h.price + jitter).toFixed(2);
        const newPnl = +((newPrice - h.cost) * h.qty).toFixed(0);
        const newPnlPct = +(((newPrice - h.cost) / h.cost) * 100).toFixed(2);
        return { ...h, price: newPrice, pnl: newPnl, pnlPct: newPnlPct };
      }));
    }, 2000);
    return () => clearInterval(timer);
  }, []);

  const totalValue = data.reduce((s, h) => s + h.price * h.qty, 0);
  const totalPnl = data.reduce((s, h) => s + h.pnl, 0);
  const upCount = data.filter(h => h.pnl >= 0).length;

  return (
    <div>
      <div className="page-header">
        <h2><Briefcase size={20} style={{ marginRight: 4 }} />持仓动态跟踪</h2>
        <span className="row gap8">
          <span className="live-dot" />
          <span className="muted">实时刷新中 · {new Date().toLocaleTimeString()}</span>
        </span>
      </div>

      <div className="stat-grid mb16">
        <div className="stat-card">
          <div className="stat-label">总市值</div>
          <div className="stat-value">{totalValue.toLocaleString()}<span style={{ fontSize: 14, color: 'var(--color-text-2)' }}> 元</span></div>
        </div>
        <div className="stat-card">
          <div className="stat-label">总盈亏</div>
          <div className={`stat-value ${totalPnl >= 0 ? 'up' : 'down'}`}>
            {totalPnl >= 0 ? '+' : ''}{totalPnl.toLocaleString()}<span style={{ fontSize: 14 }}> 元</span>
          </div>
          <div className={`stat-sub ${totalPnl >= 0 ? 'up' : 'down'}`}>
            {totalPnl >= 0 ? '+' : ''}{((totalPnl / (totalValue - totalPnl)) * 100).toFixed(2)}%
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-label">持仓数</div>
          <div className="stat-value">{data.length}<span style={{ fontSize: 14, color: 'var(--color-text-2)' }}> 只</span></div>
          <div className="stat-sub">
            <span className="up">{upCount}盈</span> / <span className="down">{data.length - upCount}亏</span>
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-label">仓位状态</div>
          <div className="stat-value" style={{ fontSize: 18, color: 'var(--green-6)' }}>正常运行</div>
        </div>
      </div>

      <div className="card">
        <div className="card-header">
          <span className="card-title">持仓明细</span>
          <span className="muted">行情模拟 · 2s 刷新</span>
        </div>
        <Table
          columns={[
            { title: '代码', dataIndex: 'code', width: 96, render: (v: string) => <span className="num" style={{ fontWeight: 600 }}>{v}</span> },
            { title: '名称', dataIndex: 'name', width: 96 },
            { title: '成本', dataIndex: 'cost', width: 80, render: (v: number) => <span className="num">{v.toFixed(2)}</span> },
            { title: '现价', dataIndex: 'price', width: 88, render: (v: number) => <span className="num" style={{ fontWeight: 600 }}>{v.toFixed(2)}</span> },
            { title: '持仓', dataIndex: 'qty', width: 64, render: (v: number) => `${v}股` },
            { title: '盈亏', dataIndex: 'pnl', width: 100, render: (v: number) => <span className={v >= 0 ? 'up' : 'down'}>{v >= 0 ? '+' : ''}¥{v.toLocaleString()}</span> },
            { title: '盈亏%', dataIndex: 'pnlPct', width: 80, render: (v: number) => <span className={`tag ${v >= 0 ? 'tag-red' : 'tag-green'}`} style={{ fontSize: 12 }}>{v >= 0 ? <ArrowUp size={10} /> : <ArrowDown size={10} />}{Math.abs(v).toFixed(1)}%</span> },
            { title: '止盈', dataIndex: 'stopProfit', width: 80, render: (v: number) => <span className="muted">¥{v.toFixed(2)}</span> },
            { title: '止损', dataIndex: 'stopLoss', width: 80, render: (v: number) => <span className="muted">¥{v.toFixed(2)}</span> },
            { title: '信号', dataIndex: 'signal', width: 88, render: (v: string) => {
              const color = v.includes('止盈') ? 'tag-orange' : v.includes('止损') ? 'tag-red' : 'tag-green';
              return <span className={`tag ${color}`}>{v}</span>;
            }},
          ]}
          data={data}
          rowKey="id"
          pagination={false}
          scroll={{ x: 960 }}
        />
      </div>
    </div>
  );
}
