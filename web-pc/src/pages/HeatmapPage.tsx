import { useEffect, useState } from 'react';
import ReactECharts from 'echarts-for-react';
import { fetchHeatmap } from '../services/api';
import { Grid3X3, Calendar } from 'lucide-react';

export default function HeatmapPage() {
  const [option, setOption] = useState<any>(null);
  const [view, setView] = useState<'matrix' | 'calendar'>('matrix');

  useEffect(() => {
    fetchHeatmap().then((res: any) => {
      const details = res.data || [];
      const dates = [...new Set(details.map((d: any) => d.pickDate?.slice(0, 10)))] as string[];
      const codes = [...new Set(details.map((d: any) => d.stockCode))] as string[];
      const data: [number, number, number][] = [];
      details.forEach((d: any) => {
        const x = dates.indexOf(d.pickDate?.slice(0, 10));
        const y = codes.indexOf(d.stockCode);
        if (x >= 0 && y >= 0) data.push([x, y, d.score || 0]);
      });

      if (view === 'matrix') {
        setOption({
          tooltip: { position: 'top', formatter: (p: any) => `${codes[p.data[1]]} · ${dates[p.data[0]]}<br/>评分: ${p.data[2]}` },
          grid: { left: 110, right: 40, top: 30, bottom: 60 },
          xAxis: { type: 'category', data: dates, axisLabel: { rotate: 45, fontSize: 11 }, position: 'top' },
          yAxis: { type: 'category', data: codes, axisLabel: { fontSize: 11 } },
          visualMap: { min: 0, max: 100, calculable: true, orient: 'horizontal', left: 'center', bottom: 0, inRange: { color: ['#E8F3FF', '#4080FF', '#722ED1','#F53F3F'] } },
          series: [{ type: 'heatmap', data, label: { show: false }, itemStyle: { borderRadius: 2 } }],
        });
      }
    });
  }, [view]);

  return (
    <div>
      <div className="page-header">
        <h2><Grid3X3 size={20} style={{ marginRight: 4 }} />上榜热力图</h2>
        <div className="seg">
          <button className={view === 'matrix' ? 'active' : ''} onClick={() => setView('matrix')}>
            <Grid3X3 size={13} style={{ marginRight: 4 }} />矩阵
          </button>
          <button className={view === 'calendar' ? 'active' : ''} onClick={() => setView('calendar')}>
            <Calendar size={13} style={{ marginRight: 4 }} />日历
          </button>
        </div>
      </div>
      <div className="card">
        <div className="card-body">
          {option ? (
            <ReactECharts option={option} style={{ height: view === 'matrix' ? 600 : 400 }} />
          ) : (
            <div className="muted" style={{ textAlign: 'center', padding: 60 }}>暂无热力图数据，请先导入 Excel</div>
          )}
        </div>
      </div>
    </div>
  );
}
