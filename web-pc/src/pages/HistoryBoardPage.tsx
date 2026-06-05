import { useState } from 'react';
import { DatePicker, Table } from '@arco-design/web-react';
import { CalendarDays, TrendingUp, TrendingDown } from 'lucide-react';
import { fetchHistoryBoard } from '../services/api';

export default function HistoryBoardPage() {
  const [date, setDate] = useState('');
  const [data, setData] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);

  const handleChange = (d: string) => {
    setDate(d);
    setLoading(true);
    fetchHistoryBoard(d).then((res: any) => setData(res.data || [])).finally(() => setLoading(false));
  };

  const columns = [
    { title: '#', dataIndex: 'rank', width: 50, render: (v: number) => <span className="muted">{v}</span> },
    { title: '代码', dataIndex: 'stockCode', width: 100, render: (v: string) => <span className="num" style={{fontWeight:600}}>{v}</span> },
    { title: '评分', dataIndex: 'score', width: 80, render: (v: number) => <span className="num" style={{fontWeight:600}}>{v?.toFixed(1)}</span> },
    { title: '信号', dataIndex: 'suggestion', width: 80, render: (v: string) => {
      const m: Record<string,string> = { buy: '买入', hold: '持有', sell: '卖出' };
      const c: Record<string,string> = { buy: 'tag-red', hold: 'tag-gray', sell: 'tag-green' };
      return <span className={`tag ${c[v]||'tag-gray'}`}>{m[v]||v}</span>;
    }},
    { title: '风险', dataIndex: 'riskLevel', width: 70, render: (v: string) => {
      const c: Record<string,string> = { high: 'tag-red', medium: 'tag-orange', low: 'tag-green' };
      return <span className={`tag ${c[v]||'tag-gray'}`}>{v==='high'?'高':v==='medium'?'中':'低'}</span>;
    }},
  ];

  return (
    <div>
      <div className="page-header">
        <div className="row gap8"><h2><CalendarDays size={20} style={{marginRight:4}} />历史榜单</h2><span className="muted">回看任意历史交易日的算法榜单</span></div>
      </div>
      <div className="card">
        <div className="card-body" style={{display:'flex',alignItems:'center',gap:16,marginBottom:12}}>
          <DatePicker onChange={(_,ds) => handleChange(ds.dateString?.toString()||'')} style={{width:180}} />
          {data.length > 0 && <span className="muted">共 <b>{data.length}</b> 只上榜股票</span>}
        </div>
        <Table columns={columns} data={data} loading={loading} rowKey="id" pagination={false} />
      </div>
    </div>
  );
}
