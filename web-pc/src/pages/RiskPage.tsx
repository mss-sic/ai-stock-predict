import { Table } from '@arco-design/web-react';
import { ShieldAlert, AlertTriangle, Info } from 'lucide-react';

const mockAlerts = [
  { id:1, code:'000001', level:'high', type:'连榜过热', desc:'连续上榜 15 个交易日，短线情绪过热', date:'2026-06-03' },
  { id:2, code:'002456', level:'medium', type:'技术破位', desc:'跌破 20 日均线支撑位', date:'2026-06-02' },
  { id:3, code:'300613', level:'high', type:'质押偏高', desc:'控股股东质押比例超 60%', date:'2026-06-01' },
  { id:4, code:'688170', level:'low', type:'流动性弱', desc:'日均成交额低于 5000 万', date:'2026-06-03' },
];

const iconMap: Record<string,any> = { high:AlertTriangle, medium:ShieldAlert, low:Info };
const colorMap: Record<string,string> = { high:'tag-red', medium:'tag-orange', low:'tag-green' };

export default function RiskPage() {
  return (
    <div>
      <div className="page-header"><h2><ShieldAlert size={20} style={{marginRight:4}} />风险预警</h2><span className="muted">基于公开数据与算法规则自动生成</span></div>
      <div className="stat-grid mb16">
        <div className="stat-card"><div className="stat-label">🔴 高风险</div><div className="stat-value" style={{color:'var(--red-6)'}}>2</div></div>
        <div className="stat-card"><div className="stat-label">🟠 中风险</div><div className="stat-value" style={{color:'var(--orange-6)'}}>1</div></div>
        <div className="stat-card"><div className="stat-label">🟢 低风险</div><div className="stat-value" style={{color:'var(--green-6)'}}>1</div></div>
        <div className="stat-card"><div className="stat-label">总计预警</div><div className="stat-value">4</div></div>
      </div>
      <div className="card">
        <Table columns={[
          { title: '代码', dataIndex: 'code', width:100, render:(v:string)=><span className="num" style={{fontWeight:600}}>{v}</span> },
          { title: '等级', dataIndex: 'level', width:80, render:(v:string)=>{const I=iconMap[v];return <span className={`tag ${colorMap[v]}`}><I size={11}/>{v==='high'?'高':v==='medium'?'中':'低'}</span>;} },
          { title: '类型', dataIndex: 'type', width:120 },
          { title: '说明', dataIndex: 'desc' },
          { title: '日期', dataIndex: 'date', width:110, render:(v:string)=><span className="muted">{v}</span> },
        ]} data={mockAlerts} rowKey="id" pagination={false} />
      </div>
    </div>
  );
}
