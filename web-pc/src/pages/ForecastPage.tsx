import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import ReactECharts from 'echarts-for-react';
import { fetchForecast } from '../services/api';
import { TrendingUp } from 'lucide-react';

const HORIZONS = [5, 10, 20, 30];

export default function ForecastPage() {
  const { code } = useParams<{ code: string }>();
  const [horizon, setHorizon] = useState(10);
  const [data, setData] = useState<any[]>([]);

  useEffect(() => {
    if (!code) return;
    fetchForecast(code, horizon).then((res: any) => setData(res.data?.data || []));
  }, [code, horizon]);

  const option = {
    tooltip: { trigger: 'axis' },
    grid: { left: 50, right: 30, top: 20, bottom: 30 },
    xAxis: { type: 'category', data: data.map((d: any) => `第${d.day}日`) },
    yAxis: { type: 'value', splitLine: { lineStyle: { color: '#E5E6EB' } } },
    series: [
      { name: '预测', type: 'line', data: data.map((d: any) => d.price), smooth: true, areaStyle: { color: 'rgba(22,93,255,0.08)' }, lineStyle: { color: '#165DFF', width: 2 }, symbol: 'circle', symbolSize: 6 },
      { name: '上轨', type: 'line', data: data.map((d: any) => d.upper), lineStyle: { type: 'dashed', color: '#C9CDD4', width: 1 }, symbol: 'none' },
      { name: '下轨', type: 'line', data: data.map((d: any) => d.lower), lineStyle: { type: 'dashed', color: '#C9CDD4', width: 1 }, symbol: 'none' },
    ],
  };

  return (
    <div>
      <div className="page-header"><h2><TrendingUp size={20} style={{marginRight:4}} />走势预测</h2><span className="muted">{code} · 算法集成预测</span></div>
      <div className="card">
        <div className="card-header">
          <span className="card-title">价格路径预测</span>
          <div className="seg">{HORIZONS.map(h => <button key={h} className={horizon===h?'active':''} onClick={()=>setHorizon(h)}>{h}日</button>)}</div>
        </div>
        <div className="card-body"><ReactECharts option={option} style={{ height: 380 }} /></div>
      </div>
    </div>
  );
}
