import { useEffect, useState } from 'react';
import { Table, Tag } from '@arco-design/web-react';
import { useNavigate, Link } from 'react-router-dom';
import { TrendingUp, TrendingDown, Minus, Star, Shield, AlertTriangle } from 'lucide-react';
import { fetchTodayBoard, fetchBoardHistory, authFetch } from '../services/api';
const SUGGEST_COLORS: Record<string,string>={'强烈买入':'#F53F3F','买入':'#F77234','增持':'#FF7D00','持有':'#86909C','减持':'#3491FA','卖出':'#00B42A','强烈卖出':'#009A29'};
const SUGGEST_BG: Record<string,string>={'强烈买入':'#FFECE8','买入':'#FFF3E8','增持':'#FFF7E8','持有':'#F2F3F5','减持':'#E8F3FF','卖出':'#E8FFEA','强烈卖出':'#DBF5DF'};
const RISK_COLORS: Record<string,string>={'高风险':'#F53F3F','中高风险':'#F77234','中风险':'#FF7D00','中低风险':'#3491FA','低风险':'#00B42A'};
const RISK_BG: Record<string,string>={'高风险':'#FFECE8','中高风险':'#FFF3E8','中风险':'#FFF7E8','中低风险':'#E8F3FF','低风险':'#E8FFEA'};


interface BoardItem {
  id: number; pickDate: string; stockCode: string; stockName: string;
  rank: number; score: number; riskLevel: string; suggestion: string;
  open: number; close: number; preClose: number; chgPct: number;
  pe: number; pb: number; industry: string; marketCap: number;
}

export default function BoardPage() {
  const [data, setData] = useState<BoardItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [boardDate, setBoardDate] = useState('');
  const navigate = useNavigate();

  useEffect(() => {
    (async () => {
      setLoading(true);
      try {
        const res: any = await fetchTodayBoard();
        if (res.data?.data?.length > 0) {
          setData(res.data.data);
          setBoardDate(res.date || '');
        } else {
          // Try recent dates
          const d = new Date();
          for (let i = 1; i <= 5; i++) {
            d.setDate(d.getDate() - 1);
            const ds = d.toISOString().slice(0, 10);
            try {
              const r: any = await (await authFetch(`/api/v1/board/history?date=${ds}`)).json();
              if (r.data?.length > 0) {
                setData(r.data);
                setBoardDate(ds);
                break;
              }
            } catch (_) {}
          }
        }
      } catch (_) {}
      setLoading(false);
    })();
  }, []);

  const fmtMoney = (v: number) => {
    if (!v || v === 0) return '-';
    if (v > 1e8) return (v / 1e8).toFixed(2) + '亿';
    if (v > 1e4) return (v / 1e4).toFixed(2) + '万';
    return v.toFixed(2);
  };

  const columns = [
    {
      title: '#', dataIndex: 'rank', width: 48, align: 'center' as const,
      sorter: (a: BoardItem, b: BoardItem) => a.rank - b.rank,
      render: (v: number) => {
        if (v === 1) return <span style={{ color: '#f53f3f', fontWeight: 700, fontSize: 16 }}>🥇</span>;
        if (v === 2) return <span style={{ color: '#ff7d00', fontWeight: 700, fontSize: 16 }}>🥈</span>;
        if (v === 3) return <span style={{ color: '#ffb400', fontWeight: 700, fontSize: 16 }}>🥉</span>;
        return <span style={{ color: '#86909c', fontWeight: 500 }}>{v}</span>;
      },
    },
    {
      title: '股票', dataIndex: 'stockCode', width: 160,
      render: (_: string, record: BoardItem) => (
        <div style={{ cursor: 'pointer' }} onClick={() => navigate(`/stock/${record.stockCode}`)}>
          <div style={{ fontWeight: 600, fontSize: 14, color: '#1d2129' }}>{record.stockName || record.stockCode}</div>
          <div style={{ fontSize: 11, color: '#86909c', marginTop: 1 }}>{record.stockCode}</div>
        </div>
      ),
    },
    {
      title: '行业', dataIndex: 'industry', width: 90,
      render: (v: string) => v ? <span style={{ fontSize: 12, color: '#4e5969' }}>{v}</span> : <span className="muted">-</span>,
    },
    {
      title: '收盘', dataIndex: 'close', width: 80, align: 'right' as const,
      sorter: (a: BoardItem, b: BoardItem) => (a.close || 0) - (b.close || 0),
      render: (v: number) => v > 0 ? <span style={{ fontWeight: 600, fontSize: 14 }}>{v.toFixed(2)}</span> : <span className="muted">-</span>,
    },
    {
      title: '涨跌', dataIndex: 'chgPct', width: 85, align: 'right' as const,
      sorter: (a: BoardItem, b: BoardItem) => (a.chgPct || 0) - (b.chgPct || 0),
      render: (v: number) => {
        if (v === undefined || v === null) return <span className="muted">-</span>;
        const color = v > 0 ? '#f53f3f' : v < 0 ? '#00b42a' : '#86909c';
        const icon = v > 0 ? <TrendingUp size={12} /> : v < 0 ? <TrendingDown size={12} /> : <Minus size={12} />;
        return (
          <span style={{ color, fontWeight: 600, fontSize: 13, display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 2 }}>
            {icon}{v > 0 ? '+' : ''}{v.toFixed(2)}%
          </span>
        );
      },
    },
    {
      title: '评分', dataIndex: 'score', width: 65, align: 'right' as const,
      sorter: (a: BoardItem, b: BoardItem) => (a.score || 0) - (b.score || 0),
      render: (v: number) => {
        if (v === undefined || v === null) return <span className="muted">-</span>;
        const color = v > 5 ? '#F53F3F' : v > 0 ? '#FF7D00' : v > -5 ? '#86909C' : '#00B42A';
        return <span style={{ fontWeight: 600, fontSize: 13, color }}>{v > 0 ? '+' : ''}{v.toFixed(1)}</span>;
      },
    },
    {
      title: 'PE', dataIndex: 'pe', width: 70, align: 'right' as const,
      sorter: (a: BoardItem, b: BoardItem) => (a.pe || 0) - (b.pe || 0),
      render: (v: number) => v > 0 ? <span style={{ fontWeight: 500, fontSize: 13 }}>{v.toFixed(1)}</span> : <span className="muted">-</span>,
    },
    {
      title: 'PB', dataIndex: 'pb', width: 70, align: 'right' as const,
      sorter: (a: BoardItem, b: BoardItem) => (a.pb || 0) - (b.pb || 0),
      render: (v: number) => v > 0 ? <span style={{ fontWeight: 500, fontSize: 13 }}>{v.toFixed(2)}</span> : <span className="muted">-</span>,
    },
    {
      title: '建议', dataIndex: 'suggestion', width: 85, align: 'center' as const,
      render: (v: string) => {
        if (!v) return <span style={{ color: '#C9CDD4', fontSize: 12 }}>—</span>;
        const color = SUGGEST_COLORS[v] || '#86909C';
        const bg = SUGGEST_BG[v] || '#F2F3F5';
        return (
          <span style={{
            display: 'inline-block', padding: '2px 10px', borderRadius: 4,
            background: bg, color, fontWeight: 600, fontSize: 12,
            border: '1px solid ' + color,
          }}>{v}</span>
        );
      },
    },
    {
      title: '风险', dataIndex: 'riskLevel', width: 85, align: 'center' as const,
      render: (v: string) => {
        if (!v) return <span style={{ color: '#C9CDD4', fontSize: 12 }}>—</span>;
        const color = RISK_COLORS[v] || '#86909C';
        const bg = RISK_BG[v] || '#F2F3F5';
        return (
          <span style={{
            display: 'inline-block', padding: '2px 10px', borderRadius: 4,
            background: bg, color, fontWeight: 600, fontSize: 12,
            border: '1px solid ' + color,
          }}>{v}</span>
        );
      },
    },
  ];

  // Count missing data fields
  const missingStats = {
    name: data.filter(s => !s.stockName).length,
    pe: data.filter(s => !s.pe || s.pe === 0).length,
    pb: data.filter(s => !s.pb || s.pb === 0).length,
    industry: data.filter(s => !s.industry).length,
  };

  return (
    <div>
      <div className="page-header">
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <h2 style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <Star size={22} color="#165dff" />
            <span>算法精选榜单</span>
          </h2>
          {boardDate && (
            <Tag color="blue" style={{ fontSize: 12 }}>
              {boardDate === new Date().toISOString().slice(0, 10) ? '今日' : boardDate}
            </Tag>
          )}
          <span className="muted" style={{ fontSize: 13 }}>共 {data.length} 只上榜股票</span>
        </div>
      </div>

      {/* Missing data warning */}
      {(missingStats.name > 0 || missingStats.pe > 0 || missingStats.industry > 0) && (
        <div style={{
          marginBottom: 16, padding: '10px 16px', background: '#fff7e8',
          border: '1px solid #ffb400', borderRadius: 6, fontSize: 13, color: '#6b5900',
          display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap',
        }}>
          <AlertTriangle size={14} color="#ffb400" />
          <span>部分数据缺失：</span>
          {missingStats.name > 0 && <span><b>{missingStats.name}</b> 只缺名称（需运行股票列表同步）</span>}
          {missingStats.pe > 0 && <span>· <b>{missingStats.pe}</b> 只缺 PE/PB（需运行指标采集）</span>}
          {missingStats.industry > 0 && <span>· <b>{missingStats.industry}</b> 只缺行业（需运行行业采集）</span>}
          <span style={{ marginLeft: 'auto', color: '#86909c', fontSize: 12 }}>
            去 <Link to="/data" style={{ color: 'var(--arcoblue-6)' }}>数据管理</Link> 执行采集补全
          </span>
        </div>
      )}

      <div className="card" style={{ overflow: 'hidden' }}>
        <Table
          columns={columns}
          data={data || []}
          loading={loading}
          rowKey="id"
          pagination={false}
          scroll={{ x: 900 }}
          stripe
          onRow={(record) => ({
            onClick: () => navigate(`/stock/${record.stockCode}`),
            style: { cursor: 'pointer' },
          })}
          empty={() => (
            <div style={{ padding: 64, textAlign: 'center' }}>
              <div style={{ fontSize: 40, marginBottom: 12 }}>📊</div>
              <div style={{ fontSize: 15, fontWeight: 600, color: '#1d2129', marginBottom: 6 }}>暂无榜单数据</div>
              <div style={{ fontSize: 13, color: '#86909c', lineHeight: 1.8 }}>
                请先通过 <Link to="/data" style={{ color: 'var(--arcoblue-6)' }}>数据管理</Link> 导入 Excel 文件<br/>
                或触发数据采集后生成算法榜单
              </div>
            </div>
          )}
        />
      </div>
    </div>
  );
}
