import { Briefcase, ArrowUp, ArrowDown } from 'lucide-react';
import { Table } from '@arco-design/web-react';

const mock = [
  { id:1, code:'600519', name:'贵州茅台', cost:1850, price:1876, qty:100, pnl:2600, pnlPct:1.4 },
  { id:2, code:'000858', name:'五粮液', cost:152, price:148.5, qty:500, pnl:-1750, pnlPct:-2.3 },
  { id:3, code:'300750', name:'宁德时代', cost:210, price:218.3, qty:200, pnl:1660, pnlPct:3.95 },
];

export default function HoldingsPage() {
  const totalPnl = mock.reduce((s, i) => s + i.pnl, 0);
  const totalValue = mock.reduce((s, i) => s + i.price * i.qty, 0);

  return (
    <div>
      <div className="page-header"><h2><Briefcase size={20} style={{marginRight:4}} />持仓动态跟踪</h2><span className="muted">实时盈亏 · 止盈止损距离</span></div>

      <div className="stat-grid mb16">
        <div className="stat-card"><div className="stat-label">总市值</div><div className="stat-value">{totalValue.toLocaleString()}<span style={{fontSize:14,color:'var(--color-text-2)'}}>元</span></div></div>
        <div className="stat-card"><div className="stat-label">总盈亏</div><div className={`stat-value ${totalPnl>=0?'up':'down'}`}>{totalPnl>=0?'+':''}{totalPnl.toLocaleString()}<span style={{fontSize:14}}>元</span></div></div>
        <div className="stat-card"><div className="stat-label">持仓数</div><div className="stat-value">{mock.length}<span style={{fontSize:14,color:'var(--color-text-2)'}}>只</span></div></div>
        <div className="stat-card"><div className="stat-label">盈亏比</div><div className="stat-value up">2/1</div></div>
      </div>

      <div className="card">
        <Table columns={[
          { title: '代码', dataIndex: 'code', render: (v:string) => <span className="num" style={{fontWeight:600}}>{v}</span> },
          { title: '名称', dataIndex: 'name' },
          { title: '成本', dataIndex: 'cost', render: (v:number) => <span className="num">¥{v.toFixed(2)}</span> },
          { title: '现价', dataIndex: 'price', render: (v:number) => <span className="num" style={{fontWeight:600}}>¥{v.toFixed(2)}</span> },
          { title: '持仓', dataIndex: 'qty', render: (v:number) => `${v}股` },
          { title: '盈亏', dataIndex: 'pnl', render: (v:number) => <span className={v>=0?'up':'down'}>{v>=0?'+':''}¥{v.toFixed(0)}</span> },
          { title: '盈亏%', dataIndex: 'pnlPct', render: (v:number) => <span className={`tag ${v>=0?'tag-red':'tag-green'}`}>{v>=0?<ArrowUp size={10}/>:<ArrowDown size={10}/>}{Math.abs(v).toFixed(1)}%</span> },
        ]} data={mock} rowKey="id" pagination={false} />
      </div>
    </div>
  );
}
