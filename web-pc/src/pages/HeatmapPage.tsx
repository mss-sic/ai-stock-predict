import { useEffect, useState } from 'react';
import { Card } from '@arco-design/web-react';
import ReactECharts from 'echarts-for-react';
import { fetchHeatmap } from '../services/api';

export default function HeatmapPage() {
  const [chartOption, setChartOption] = useState<any>(null);

  useEffect(() => {
    fetchHeatmap().then((res: any) => {
      const details = res.data || [];
      const dates = [...new Set(details.map((d: any) => d.pickDate?.slice(0, 10)))].sort() as string[];
      const codes = [...new Set(details.map((d: any) => d.stockCode))] as string[];
      const heatData: [number, number, number][] = [];
      details.forEach((d: any) => {
        const x = dates.indexOf(d.pickDate?.slice(0, 10));
        const y = codes.indexOf(d.stockCode);
        if (x >= 0 && y >= 0) heatData.push([x, y, d.score || 0]);
      });

      setChartOption({
        tooltip: { position: 'top' },
        grid: { left: 120, right: 40, top: 40, bottom: 80 },
        xAxis: { type: 'category', data: dates, axisLabel: { rotate: 45 } },
        yAxis: { type: 'category', data: codes },
        visualMap: { min: 0, max: 100, calculable: true, orient: 'horizontal', left: 'center', bottom: 0 },
        series: [{ type: 'heatmap', data: heatData, label: { show: false } }],
      });
    });
  }, []);

  return (
    <Card title="🔥 上榜热力图">
      {chartOption ? <ReactECharts option={chartOption} style={{ height: 600 }} /> : <div>加载中...</div>}
    </Card>
  );
}
