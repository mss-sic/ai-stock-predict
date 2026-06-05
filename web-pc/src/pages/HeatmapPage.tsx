import { useEffect, useState } from 'react';
import ReactECharts from 'echarts-for-react';
import { fetchHeatmap } from '../services/api';
import { Grid3X3 } from 'lucide-react';

export default function HeatmapPage() {
  const [option, setOption] = useState<any>(null);
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
      setOption({
        tooltip: { position: 'top' },
        grid: { left: 110, right: 40, top: 30, bottom: 60 },
        xAxis: { type: 'category', data: dates, axisLabel: { rotate: 45, fontSize: 11 } },
        yAxis: { type: 'category', data: codes, axisLabel: { fontSize: 11 } },
        visualMap: { min: 0, max: 100, calculable: true, orient: 'horizontal', left: 'center', bottom: 0, inRange: { color: ['#E8F3FF', '#165DFF', '#722ED1'] } },
        series: [{ type: 'heatmap', data, label: { show: false }, itemStyle: { borderRadius: 2 } }],
      });
    });
  }, []);
  return (
    <div>
      <div className="page-header"><h2><Grid3X3 size={20} style={{marginRight:4}} />上榜热力图</h2><span className="muted">20 交易日上榜矩阵，按评分着色</span></div>
      <div className="card"><div className="card-body">{option ? <ReactECharts option={option} style={{ height: 600 }} /> : <div className="muted" style={{textAlign:'center',padding:60}}>暂无热力图数据</div>}</div></div>
    </div>
  );
}
