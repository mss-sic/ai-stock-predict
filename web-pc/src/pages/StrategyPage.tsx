import { useState } from 'react';
import { Target, BarChart4 } from 'lucide-react';

export default function StrategyPage() {
  const [tab, setTab] = useState<'plan' | 'backtest'>('plan');
  return (
    <div>
      <div className="page-header"><h2><Target size={20} style={{marginRight:4}} />交易策略中心</h2><span className="muted">卖点计算 · 预期收益 · 回测验证</span></div>
      <div className="card mb16">
        <div className="card-header">
          <div className="seg">
            <button className={tab==='plan'?'active':''} onClick={()=>setTab('plan')}><Target size={13} style={{marginRight:4}} />交易计划</button>
            <button className={tab==='backtest'?'active':''} onClick={()=>setTab('backtest')}><BarChart4 size={13} style={{marginRight:4}} />策略回测</button>
          </div>
        </div>
        <div className="card-body">
          {tab === 'plan' ? (
            <div className="row gap16" style={{flexWrap:'wrap'}}>
              <div className="col gap8" style={{flex:1,minWidth:200}}>
                <label className="muted">止盈 %</label>
                <input type="number" defaultValue={15} style={{padding:'6px 10px',border:'1px solid var(--color-border-2)',borderRadius:4,width:'100%'}} />
              </div>
              <div className="col gap8" style={{flex:1,minWidth:200}}>
                <label className="muted">止损 %</label>
                <input type="number" defaultValue={-8} style={{padding:'6px 10px',border:'1px solid var(--color-border-2)',borderRadius:4,width:'100%'}} />
              </div>
              <div className="col gap8" style={{flex:1,minWidth:200}}>
                <label className="muted">最大持有日</label>
                <input type="number" defaultValue={20} style={{padding:'6px 10px',border:'1px solid var(--color-border-2)',borderRadius:4,width:'100%'}} />
              </div>
              <div className="col" style={{justifyContent:'flex-end'}}>
                <button className="chip active" style={{fontSize:13,padding:'8px 20px'}}>计算预期收益</button>
              </div>
            </div>
          ) : (
            <div className="muted" style={{textAlign:'center',padding:40}}>🚧 回测功能构建中，敬请期待</div>
          )}
        </div>
      </div>
      <div className="stat-grid">
        <div className="stat-card"><div className="stat-label">预期收益率</div><div className="stat-value">12.5<span style={{fontSize:14,color:'var(--color-text-2)'}}>%</span></div><div className="stat-sub up">+2.3% vs 基准</div></div>
        <div className="stat-card"><div className="stat-label">盈亏比</div><div className="stat-value">1.88</div></div>
        <div className="stat-card"><div className="stat-label">胜率预估</div><div className="stat-value">58<span style={{fontSize:14,color:'var(--color-text-2)'}}>%</span></div></div>
        <div className="stat-card"><div className="stat-label">建议仓位</div><div className="stat-value">15<span style={{fontSize:14,color:'var(--color-text-2)'}}>%</span></div></div>
      </div>
    </div>
  );
}
