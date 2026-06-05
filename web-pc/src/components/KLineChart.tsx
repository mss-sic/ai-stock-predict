import ReactECharts from 'echarts-for-react';

interface KLineProps { data: any[]; }

export default function KLineChart({ data }: KLineProps) {
  if (!data || data.length === 0) return <div style={{ height: 400, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#999' }}>暂无K线数据</div>;

  const dates = data.map((d: any) => d.tradeDate?.slice(0, 10) || d.date || '');
  const ohlc = data.map((d: any) => [d.open, d.close, d.low, d.high]);
  const volumes = data.map((d: any) => d.volume || 0);

  const option = {
    tooltip: { trigger: 'axis', axisPointer: { type: 'cross' } },
    grid: [
      { left: '10%', right: '8%', top: '10%', height: '60%' },
      { left: '10%', right: '8%', top: '75%', height: '15%' },
    ],
    xAxis: [
      { type: 'category', data: dates, gridIndex: 0 },
      { type: 'category', data: dates, gridIndex: 1, axisLabel: { show: false } },
    ],
    yAxis: [
      { type: 'value', gridIndex: 0 },
      { type: 'value', gridIndex: 1 },
    ],
    series: [
      { type: 'candlestick', data: ohlc, xAxisIndex: 0, yAxisIndex: 0,
        itemStyle: { color: '#ef5350', color0: '#26a69a', borderColor: '#ef5350', borderColor0: '#26a69a' } },
      { type: 'bar', data: volumes, xAxisIndex: 1, yAxisIndex: 1 },
    ],
  };

  return <ReactECharts option={option} style={{ height: 400 }} />;
}
