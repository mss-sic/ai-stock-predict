import { useEffect, useState } from 'react';
import { Table } from '@arco-design/web-react';
import { useNavigate } from 'react-router-dom';
import { TrendingUp, TrendingDown, AlertTriangle, Shield, ThumbsUp } from 'lucide-react';
import { fetchTodayBoard, fetchHistoryBoard, fetchStocks } from '../services/api';

interface Stock {
  id: number; stockCode: string; rank: number; score: number;
  signalTags: string[]; riskLevel: string; suggestion: string;
}
const riskColors: Record<string, string> = { high: 'tag-red', medium: 'tag-orange', low: 'tag-green' };
const riskIcons: Record<string, any> = { high: AlertTriangle, medium: Shield, low: ThumbsUp };
const sigLabels: Record<string, string> = { buy: '买入', hold: '持有', sell: '卖出' };
const sigColors: Record<string, string> = { buy: 'tag-red', hold: 'tag-gray', sell: 'tag-green' };

export default function BoardPage() {
  const [data, setData] = useState<Stock[]>([]);
  const [loading, setLoading] = useState(true);
  const [dataDate, setDataDate] = useState('');
  const [nameMap, setNameMap] = useState<Record<string, string>>({});
  const navigate = useNavigate();

  useEffect(() => {
    (async () => {
      // Load names first
      try {
        const nameRes: any = await fetchStocks({ pageSize: 600 });
        const map: Record<string, string> = {};
        (nameRes.data || []).forEach((s: any) => { map[s.code] = s.name; });
        setNameMap(map);
      } catch (_) {}

      // Load board data
      let boardData: Stock[] = [];
      let boardDate = '';
      try {
        const todayRes: any = await fetchTodayBoard();
        if (todayRes.data && todayRes.data.length > 0) {
          boardData = todayRes.data;
          boardDate = '今日';
        }
      } catch (_) {}

      if (boardData.length === 0) {
        const d = new Date();
        for (let i = 1; i <= 10; i++) {
          d.setDate(d.getDate() - 1);
          const ds = d.toISOString().slice(0, 10);
          try {
            const r: any = await fetchHistoryBoard(ds);
            if (r.data && r.data.length > 0) {
              boardData = r.data;
              boardDate = ds;
              break;
            }
          } catch (_) { continue; }
        }
      }
      setData(boardData);
      setDataDate(boardDate);
      setLoading(false);
    })();
  }, []);

  const columns = [
    { title: '#', dataIndex: 'rank', width: 48, align: 'center' as const,
      render: (v: number) => <span className="muted">{v}</span> },
    { title: '代码', dataIndex: 'stockCode', width: 96,
      render: (v: string) => <span className="num" style={{ fontWeight: 600, cursor: 'pointer', color: 'var(--arcoblue-6)' }} onClick={() => navigate(`/stock/${v}`)}>{v}</span> },
    { title: '名称', dataIndex: 'stockCode', width: 120,
      render: (v: string) => <span style={{ fontWeight: 500 }}>{nameMap[v] || v}</span> },
    { title: '标签', dataIndex: 'signalTags',
      render: (tags: string[]) => tags && tags.length > 0
        ? <div className="chips">{tags.map((t, i) => <span key={i} className="chip">{t}</span>)}</div>
        : <span className="muted">-</span> },
    { title: '评分', dataIndex: 'score', width: 72, align: 'right' as const,
      render: (v: number) => <span className="num" style={{ fontWeight: 600, fontSize: 14 }}>{v > 0 ? v.toFixed(1) : '-'}</span> },
    { title: '信号', dataIndex: 'suggestion', width: 84, align: 'center' as const, render: (v: string) => {
      const Icon = v === 'buy' ? TrendingUp : v === 'sell' ? TrendingDown : null;
      return <span className={`tag ${sigColors[v] || 'tag-gray'}`} style={{ fontSize: 12, padding: '3px 10px' }}>{Icon && <Icon size={12} />}{sigLabels[v] || v}</span>;
    }},
    { title: '风险', dataIndex: 'riskLevel', width: 72, align: 'center' as const, render: (v: string) => {
      const Icon = riskIcons[v] || Shield;
      return <span className={`tag ${riskColors[v] || 'tag-gray'}`} style={{ fontSize: 12, padding: '3px 10px' }}>{<Icon size={12} />}{v === 'high' ? '高' : v === 'medium' ? '中' : '低'}</span>;
    }},
    { title: '', dataIndex: 'stockCode', width: 100, align: 'center' as const,
      render: (v: string) => <button className="chip active" style={{ cursor: 'pointer' }} onClick={() => navigate(`/stock/${v}`)}>查看详情</button> },
  ];

  return (
    <div>
      <div className="page-header">
        <div className="row gap8">
          <h2>📊 算法精选榜单</h2>
          {dataDate && <span className="muted">{dataDate === '今日' ? '今日' : dataDate} · {data.length} 只上榜</span>}
        </div>
      </div>
      <div className="card" style={{ overflow: 'hidden' }}>
        <Table columns={columns} data={data} loading={loading} rowKey="id" pagination={false}
          scroll={{ x: 900 }}
          empty={() => <div style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)' }}>暂无榜单数据，请先导入 Excel 或触发数据采集</div>} />
      </div>
    </div>
  );
}
