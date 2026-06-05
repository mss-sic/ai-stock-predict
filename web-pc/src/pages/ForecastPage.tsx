import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { Card, Button } from '@arco-design/web-react';
import ReactECharts from 'echarts-for-react';
import { fetchForecast } from '../services/api';

const horizons = [5, 10, 20, 30];

export default function ForecastPage() {
  const { code } = useParams<{ code: string }>();
  const [horizon, setHorizon] = useState(10);
  const [data, setData] = useState<any[]>([]);

  useEffect(() => {
    if (!code) return;
    fetchForecast(code, horizon).then((res: any) => setData(res.data || []));
  }, [code, horizon]);

  const option = {
    tooltip: { trigger: 'axis' },
    xAxis: { type: 'category', data: data.map((d: any) => `第${d.day}日`) },
    yAxis: { type: 'value' },
    series: [
      { name: '预测价格', type: 'line', data: data.map((d: any) => d.price), smooth: true,
        areaStyle: { color: 'rgba(22,93,255,0.1)' }, lineStyle: { color: '#165DFF' } },
      { name: '上轨', type: 'line', data: data.map((d: any) => d.upper), lineStyle: { type: 'dashed', color: '#ccc' }, symbol: 'none' },
      { name: '下轨', type: 'line', data: data.map((d: any) => d.lower), lineStyle: { type: 'dashed', color: '#ccc' }, symbol: 'none' },
    ],
  };

  return (
    <Card title={`🔮 走势预测 - ${code}`}>
      <div style={{ marginBottom: 16 }}>
        {horizons.map((h) => (
          <Button key={h} type={horizon === h ? 'primary' : 'default'} onClick={() => setHorizon(h)} style={{ marginRight: 8 }}>
            {h}日
          </Button>
        ))}
      </div>
      <ReactECharts option={option} style={{ height: 400 }} />
    </Card>
  );
}
