import ReactECharts from 'echarts-for-react';

interface Props { data: any[]; }

export default function KLineChart({ data }: Props) {
  if (!data || data.length === 0) return <div className="muted" style={{textAlign:'center',padding:60}}>暂无K线数据，请先触发数据采集</div>;

  const dates = data.map((d: any) => d.tradeDate?.slice(0, 10) || d.date || '');
  const ohlc = data.map((d: any) => [d.open, d.close, d.low, d.high]);
  const volumes = data.map((d: any) => d.volume || 0);
  const ma5 = calcMA(ohlc.map((d: any[]) => d[1]), 5);

  const option = {
    tooltip: { trigger: 'axis', axisPointer: { type: 'cross' } },
    grid: [
      { left: 60, right: 20, top: 20, height: '58%' },
      { left: 60, right: 20, top: '78%', height: '12%' },
    ],
    xAxis: [
      { type: 'category', data: dates, gridIndex: 0, axisLabel: { show: false } },
      { type: 'category', data: dates, gridIndex: 1, axisLabel: { fontSize: 10 } },
    ],
    yAxis: [
      { type: 'value', gridIndex: 0, splitLine: { lineStyle: { color: '#E5E6EB' } } },
      { type: 'value', gridIndex: 1, splitLine: { show: false } },
    ],
    series: [
      {
        type: 'candlestick', data: ohlc, xAxisIndex: 0, yAxisIndex: 0,
        itemStyle: { color: '#F53F3F', color0: '#00B42A', borderColor: '#F53F3F', borderColor0: '#00B42A' },
      },
      {
        type: 'line', data: ma5, xAxisIndex: 0, yAxisIndex: 0,
        lineStyle: { color: '#FF7D00', width: 1 }, symbol: 'none', name: 'MA5',
      },
      {
        type: 'bar', data: volumes, xAxisIndex: 1, yAxisIndex: 1,
        itemStyle: { color: (params: any) => {
          const idx = params.dataIndex;
          return idx > 0 && ohlc[idx][1] >= ohlc[idx][0] ? '#F53F3F40' : '#00B42A40';
        }},
      },
    ],
  };

  return <ReactECharts option={option} style={{ height: 420 }} />;
}

function calcMA(data: number[], period: number): (number | null)[] {
  const result: (number | null)[] = [];
  for (let i = 0; i < data.length; i++) {
    if (i < period - 1) { result.push(null); continue; }
    let sum = 0;
    for (let j = i - period + 1; j <= i; j++) sum += data[j];
    result.push(sum / period);
  }
  return result;
}
