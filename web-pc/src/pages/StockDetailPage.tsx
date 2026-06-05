import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { ArrowUp, ArrowDown } from 'lucide-react';
import { fetchStockDetail, fetchKLine } from '../services/api';
import KLineChart from '../components/KLineChart';

export default function StockDetailPage() {
  const { code } = useParams<{ code: string }>();
  const [stock, setStock] = useState<any>(null);
  const [klines, setKlines] = useState<any[]>([]);

  useEffect(() => {
    if (!code) return;
    fetchStockDetail(code).then((res: any) => setStock(res.data));
    fetchKLine(code).then((res: any) => setKlines(res.data || []));
  }, [code]);

  if (!stock) return <div style={{textAlign:'center',padding:60,color:'var(--color-text-3)'}}>加载中...</div>;

  const chg = 0; // Mock for now — real data would have daily change

  return (
    <div>
      <div className="page-header">
        <h2>{stock.name} <span className="muted" style={{fontSize:14,fontWeight:400}}>{stock.code}</span></h2>
        <span className={`tag ${chg >= 0 ? 'tag-red' : 'tag-green'}`}>{chg >= 0 ? <ArrowUp size={11} /> : <ArrowDown size={11} />}{Math.abs(chg).toFixed(2)}%</span>
      </div>

      <div className="stat-grid mb16">
        <div className="stat-card"><div className="stat-label">行业</div><div className="stat-value" style={{fontSize:16}}>{stock.industry || '-'}</div></div>
        <div className="stat-card"><div className="stat-label">总股本</div><div className="stat-value">{stock.totalShares ? (stock.totalShares / 1e8).toFixed(2) : '-'}<span style={{fontSize:14,color:'var(--color-text-2)'}}>亿</span></div></div>
        <div className="stat-card"><div className="stat-label">概念标签</div><div className="chips" style={{marginTop:4}}>{(stock.conceptTags || []).map((t: string, i: number) => <span key={i} className="chip">{t}</span>)}</div></div>
        <div className="stat-card"><div className="stat-label">上市日期</div><div className="stat-value" style={{fontSize:16}}>{stock.listedDate ? stock.listedDate.slice(0, 10) : '-'}</div></div>
      </div>

      <div className="card">
        <div className="card-header"><span className="card-title">📈 K线图</span></div>
        <div className="card-body"><KLineChart data={klines} /></div>
      </div>
    </div>
  );
}
