import { useEffect, useState } from 'react';
import { Table } from '@arco-design/web-react';
import { useNavigate } from 'react-router-dom';
import { TrendingUp, TrendingDown, AlertTriangle, Shield, ThumbsUp } from 'lucide-react';
import { fetchTodayBoard } from '../services/api';

interface Stock {
  id: number; stockCode: string; rank: number; score: number;
  signalTags: string[]; riskLevel: string; suggestion: string;
}

const riskColors: Record<string, string> = { high: 'tag-red', medium: 'tag-orange', low: 'tag-green' };
const riskIcons: Record<string, any> = { high: AlertTriangle, medium: Shield, low: ThumbsUp };

export default function BoardPage() {
  const [data, setData] = useState<Stock[]>([]);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();

  useEffect(() => {
    fetchTodayBoard().then((res: any) => setData(res.data || [])).finally(() => setLoading(false));
  }, []);

  const columns = [
    { title: '排名', dataIndex: 'rank', width: 60, sorter: (a: any, b: any) => a.rank - b.rank,
      render: (v: number) => <span className="muted">#{v}</span> },
    {
      title: '代码', dataIndex: 'stockCode', width: 110, render: (v: string) => (
        <span className="num" style={{ fontWeight: 600, cursor: 'pointer', color: 'var(--arcoblue-6)' }}
          onClick={() => navigate(`/stock/${v}`)}>{v}</span>
      ),
    },
    {
      title: '标签', dataIndex: 'signalTags', render: (tags: string[]) => (
        <div className="chips">{tags?.map((t, i) => <span key={i} className="chip">{t}</span>)}</div>
      ),
    },
    {
      title: '评分', dataIndex: 'score', width: 80,
      render: (v: number) => <span className="num" style={{ fontWeight: 600 }}>{v?.toFixed(1)}</span>,
    },
    {
      title: '信号', dataIndex: 'suggestion', width: 90,
      render: (v: string) => {
        const cfg: Record<string, { color: string; icon: any; label: string }> = {
          buy: { color: 'tag-red', icon: TrendingUp, label: '买入' },
          hold: { color: 'tag-gray', icon: null, label: '持有' },
          sell: { color: 'tag-green', icon: TrendingDown, label: '卖出' },
        };
        const c = cfg[v] || cfg.hold;
        const Icon = c.icon;
        return <span className={`tag ${c.color}`}>{Icon && <Icon size={11} />}{c.label}</span>;
      },
    },
    {
      title: '风险', dataIndex: 'riskLevel', width: 80,
      render: (v: string) => {
        const Icon = riskIcons[v] || Shield;
        return <span className={`tag ${riskColors[v] || 'tag-gray'}`}><Icon size={11} />{v === 'high' ? '高' : v === 'medium' ? '中' : '低'}</span>;
      },
    },
    {
      title: '操作', width: 80,
      render: (_: any, r: Stock) => (
        <button className="chip active" onClick={() => navigate(`/stock/${r.stockCode}`)}>详情 →</button>
      ),
    },
  ];

  return (
    <div>
      <div className="page-header">
        <div className="row gap8">
          <h2>📊 算法精选榜单</h2>
          <span className="muted">AI 量化模型每日精选 50 只优质标的</span>
        </div>
        <div className="seg">
          <button className="active">今日</button>
          <button>本周</button>
          <button>本月</button>
        </div>
      </div>
      <div className="card">
        <Table columns={columns} data={data} loading={loading} rowKey="id" pagination={false}
          empty={() => <div style={{ padding: 40, textAlign: 'center', color: 'var(--color-text-3)' }}>暂无榜单数据，请先导入 Excel 或触发数据采集</div>} />
      </div>
    </div>
  );
}
