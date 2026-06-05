import ReactECharts from 'echarts-for-react';

interface Props { data: any[]; }

export default function KLineChart({ data }: Props) {
  if (!data || data.length === 0) {
    return <div className="muted" style={{ textAlign: 'center', padding: 60 }}>暂无K线数据，请先触发数据采集</div>;
  }

  const dates = data.map((d: any) => d.tradeDate?.slice(0, 10) || d.date || '');
  const ohlc = data.map((d: any) => [d.open, d.close, d.low, d.high]);
  const volumes = data.map((d: any) => d.volume || 0);
  const ma5 = calcMA(ohlc.map((d: any[]) => d[1]), 5);
  const ma10 = calcMA(ohlc.map((d: any[]) => d[1]), 10);
  const ma20 = calcMA(ohlc.map((d: any[]) => d[1]), 20);

  const upColor = '#F53F3F';
  const downColor = '#00B42A';

  const option = {
    animation: false,
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'cross' },
      formatter: (params: any) => {
        if (!params || params.length < 4) return '';
        const k = params[0];
        const d = k.axisValue;
        const o = params.find((p: any) => p.seriesName === 'K线');
        const v = params.find((p: any) => p.seriesName === '成交量');
        if (!o) return '';
        const vals = o.data;
        const chg = vals[2] - vals[1];
        const chgPct = vals[1] !== 0 ? ((chg / vals[1]) * 100).toFixed(2) : '0.00';
        const color = chg >= 0 ? upColor : downColor;
        return `
          <div style="font-size:13px;line-height:22px">
            <div style="font-weight:600;margin-bottom:4px">${d}</div>
            <div>开盘：<b>${vals[1]?.toFixed(2)}</b></div>
            <div>收盘：<b style="color:${color}">${vals[2]?.toFixed(2)}</b></div>
            <div>最高：<b>${vals[4]?.toFixed(2)}</b></div>
            <div>最低：<b>${vals[3]?.toFixed(2)}</b></div>
            <div style="margin-top:4px;color:#86909C">
              涨跌：<b style="color:${color}">${chg >= 0 ? '+' : ''}${chg.toFixed(2)} (${chg >= 0 ? '+' : ''}${chgPct}%)</b>
            </div>
            ${v ? `<div>成交量：<b>${(v.data[1] / 10000).toFixed(0)}万手</b></div>` : ''}
          </div>`;
      },
    },
    grid: [
      { left: 70, right: 20, top: 20, height: '55%' },
      { left: 70, right: 20, top: '78%', height: '14%' },
    ],
    xAxis: [
      { type: 'category', data: dates, gridIndex: 0, axisLabel: { show: false }, axisLine: { lineStyle: { color: '#E5E6EB' } } },
      { type: 'category', data: dates, gridIndex: 1, axisLabel: { fontSize: 10, color: '#86909C' }, axisLine: { lineStyle: { color: '#E5E6EB' } } },
    ],
    yAxis: [
      {
        type: 'value', gridIndex: 0, scale: true, splitNumber: 5,
        axisLabel: { formatter: '{value}' },
        splitLine: { lineStyle: { color: '#F2F3F5' } },
      },
      {
        type: 'value', gridIndex: 1,
        axisLabel: {
          formatter: (v: number) => v >= 1e8 ? (v / 1e8).toFixed(1) + '亿' : (v / 1e4).toFixed(0) + '万',
          fontSize: 10, color: '#86909C',
        },
        splitLine: { show: false },
      },
    ],
    series: [
      {
        name: 'K线', type: 'candlestick', data: ohlc, xAxisIndex: 0, yAxisIndex: 0,
        itemStyle: { color: upColor, color0: downColor, borderColor: upColor, borderColor0: downColor },
        markPoint: {
          label: { formatter: (p: any) => Math.round(p.value) },
        },
      },
      {
        name: 'MA5', type: 'line', data: ma5, xAxisIndex: 0, yAxisIndex: 0, smooth: true,
        lineStyle: { color: '#FF7D00', width: 1 }, symbol: 'none',
      },
      {
        name: 'MA10', type: 'line', data: ma10, xAxisIndex: 0, yAxisIndex: 0, smooth: true,
        lineStyle: { color: '#3491FA', width: 1 }, symbol: 'none',
      },
      {
        name: 'MA20', type: 'line', data: ma20, xAxisIndex: 0, yAxisIndex: 0, smooth: true,
        lineStyle: { color: '#722ED1', width: 1 }, symbol: 'none',
      },
      {
        name: '成交量', type: 'bar', data: volumes.map((v: number, i: number) => [i, v, ohlc[i]?.[2] >= ohlc[i]?.[1]]),
        xAxisIndex: 1, yAxisIndex: 1,
        itemStyle: {
          color: (params: any) => {
            const idx = params.dataIndex;
            if (idx < 0 || idx >= ohlc.length) return downColor + '40';
            return (ohlc[idx][2] >= ohlc[idx][1] ? upColor : downColor) + '40';
          },
        },
      },
    ],
  };

  return <ReactECharts option={option} style={{ height: 440 }} notMerge />;
}

function calcMA(data: number[], period: number): (number | null)[] {
  const result: (number | null)[] = [];
  for (let i = 0; i < data.length; i++) {
    if (i < period - 1) { result.push(null); continue; }
    let sum = 0;
    for (let j = i - period + 1; j <= i; j++) sum += data[j];
    result.push(+(sum / period).toFixed(2));
  }
  return result;
}
