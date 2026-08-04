## 数据采集标准化架构（Data Pipeline Normalization Layer）

### 摘要

当前 51 个采集脚本各自为政：字段名不一致、单位不统一、ON CONFLICT 策略混乱、无数据溯源。方案核心：**数据源适配器模式 + 标准化归一层 + 优先级覆盖策略 + CSV 应急导入**。

### 1. 数据源统一模型

**新增 `stocks_daily_k` 字段**：

```sql
ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS data_source VARCHAR(20) DEFAULT 'tencent';
ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS source_priority INT DEFAULT 0;   -- 数值越大越优先
ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS data_quality VARCHAR(10) DEFAULT 'ok'; -- ok/warn/bad
```

**Go Model 同步补充**（当前 `StockDailyK` 缺少这三个字段）。

### 2. 数据源优先级体系

```
优先级 5: CSV 手动导入      (人工校验, 最高优先级, 永不覆盖)
优先级 4: Tushare 付费接口  (权威数据源, 开通后启用)
优先级 3: mootdx 通达信     (交易所直连, 稳定但慢)
优先级 2: Tencent 腾讯财经  (当前主力, 免费但不稳定)
优先级 1: Youzi 柚子API     (备用)
优先级 0: 默认/历史遗留     (最低, 可被任何来源覆盖)
```

**覆盖规则**：`ON CONFLICT DO UPDATE` 仅当 `EXCLUDED.source_priority >= stocks_daily_k.source_priority`

### 3. 数据源适配器模式

**新增 `scripts/collector/adapters/` 目录**：

```
adapters/
├── __init__.py
├── base_adapter.py      # 抽象基类：定义标准接口
├── tencent_adapter.py   # 腾讯财经 → 标准格式
├── tushare_adapter.py   # Tushare → 标准格式
├── mootdx_adapter.py    # 通达信 → 标准格式
├── youzi_adapter.py     # 柚子API → 标准格式
└── csv_adapter.py       # CSV 导入 → 标准格式
```

**适配器接口**：

```python
class BaseKlineAdapter(ABC):
    """所有K线数据源适配器的基类"""
    
    # 元数据
    SOURCE_NAME: str           # 'tencent' | 'tushare' | 'mootdx' | 'csv'
    SOURCE_PRIORITY: int       # 0-5
    SOURCE_URL: str            # API 地址
    
    # 单位声明（源数据格式）
    VOLUME_UNIT: str           # 'gu' | 'shou'  
    AMOUNT_UNIT: str           # 'yuan' | 'wan_yuan' | 'qian_yuan'
    PRICE_ADJUST: str          # 'qfq' | 'hfq' | 'none'
    
    @abstractmethod
    def fetch(self, code: str, start_date: str, end_date: str) -> list[dict]:
        """从数据源拉取原始数据"""
    
    @abstractmethod  
    def normalize(self, raw_row: dict) -> StandardKlineRow:
        """将原始数据转为标准格式"""
    
    def validate(self, row: StandardKlineRow) -> tuple[bool, str]:
        """校验标准化后的数据是否合理"""
```

**标准数据格式**：

```python
@dataclass
class StandardKlineRow:
    code: str
    trade_date: str       # YYYY-MM-DD
    open: float           # 元
    high: float           # 元
    low: float            # 元
    close: float          # 元
    pre_close: float      # 元
    change_pct: float     # %
    volume: int           # 股 (统一)
    amount: float         # 元 (统一)
    turnover_rate: float  # 原始比率
    adj_factor: float     # 复权因子（未知=1.0）
    data_source: str
    source_priority: int
```

**归一化示例（tencent_adapter）**：

```python
class TencentAdapter(BaseKlineAdapter):
    SOURCE_NAME = 'tencent'
    SOURCE_PRIORITY = 2
    VOLUME_UNIT = 'shou'   # 腾讯API默认返回手（主板是股需特殊处理）
    AMOUNT_UNIT = 'yuan'   # close × volume 计算
    PRICE_ADJUST = 'qfq'
    
    def normalize(self, raw: dict) -> StandardKlineRow:
        # 1. 板块检测：深市主板/创业板返回股，其他返回手
        is_gu_board = raw['code'].startswith(('000','001','002','003','300','301'))
        
        # 2. 单位统一 → 股
        vol = int(raw['volume']) if is_gu_board else int(raw['volume'] * 100)
        
        # 3. 成交额统一 → 元（优先用行情接口返回，fallback close×volume）
        amt = raw.get('amount_wan', 0) * 1e4
        if amt <= 0:
            amt = raw['close'] * vol
        
        return StandardKlineRow(
            code=raw['code'], trade_date=raw['trade_date'],
            open=raw['open'], high=raw['high'], low=raw['low'], close=raw['close'],
            pre_close=raw.get('pre_close', 0),
            change_pct=raw.get('change_pct', 0),
            volume=vol, amount=amt,
            turnover_rate=raw.get('turnover_rate', 0),
            adj_factor=raw.get('adj_factor', 1.0),
            data_source=self.SOURCE_NAME,
            source_priority=self.SOURCE_PRIORITY,
        )
```

### 4. 统一写入引擎

**新增 `scripts/collector/kline_writer.py`**（替代各脚本分散的 UPSERT）：

```python
UPSERT_SQL = """
    INSERT INTO stocks_daily_k (
        code, trade_date, open, high, low, close, pre_close, change_pct,
        volume, amount, turnover_rate, adj_factor, data_source, source_priority
    ) VALUES %s
    ON CONFLICT (code, trade_date) DO UPDATE SET
        -- 仅当新数据优先级 >= 旧数据时覆盖
        open   = CASE WHEN EXCLUDED.source_priority >= stocks_daily_k.source_priority 
                      THEN EXCLUDED.open   ELSE stocks_daily_k.open END,
        high   = CASE WHEN EXCLUDED.source_priority >= stocks_daily_k.source_priority 
                      THEN EXCLUDED.high   ELSE stocks_daily_k.high END,
        -- ... 所有字段同样逻辑
        data_source     = CASE WHEN EXCLUDED.source_priority >= stocks_daily_k.source_priority 
                               THEN EXCLUDED.data_source ELSE stocks_daily_k.data_source END,
        source_priority = CASE WHEN EXCLUDED.source_priority >= stocks_daily_k.source_priority 
                               THEN EXCLUDED.source_priority ELSE stocks_daily_k.source_priority END
"""
```

### 5. CSV 导入功能

**新增 `scripts/collector/csv_import.py`**：

```bash
# 用法
python3 csv_import.py --file /path/to/kline.csv --source manual --priority 5

# CSV 格式要求（兼容 tushare / 东方财富 / 同花顺导出）
# code,date,open,high,low,close,volume,amount
# 自动检测表头映射：ts_code→code, trade_date→date, pct_chg→change_pct
```

- 自动检测 CSV 格式（tushare / 东财 / 同花顺多种表头）
- 批量校验（涨跌幅>20% 告警，成交量=0 标记）
- 比数据库已有数据更高优先级（priority=5）
- 前端上传入口（后续添加到数据管理页面）

### 6. 数据源切换配置

**新增 `scripts/collector/config.yaml`**（或环境变量）：

```yaml
data_sources:
  kline:
    primary: tencent           # 主数据源
    fallback:                  # 降级链（按顺序尝试）
      - mootdx
      - tushare_free
    paid_sources:              # 付费源（需开通后启用）
      - tushare_pro
    
  indicator:
    primary: tencent           # PE/PB/市值 从腾讯实时行情取
    fallback:
      - tushare_daily_basic
  
  # 采集时自动尝试：primary → fallback[0] → fallback[1]
  # 全部失败则标记该股票该日为 data_quality='bad'
```

### 7. 数据校验层

每个适配器的 `validate()` 方法：

```python
VALIDATION_RULES = [
    ("价格范围", lambda r: 0.01 < r.close < 10000, "价格异常"),
    ("涨跌幅", lambda r: -21 < r.change_pct < 21, "超过涨跌停限制"),
    ("成交量", lambda r: r.volume >= 0, "成交量为负"),
    ("OHLC关系", lambda r: r.low <= min(r.open, r.close) and r.high >= max(r.open, r.close), "OHLC逻辑不符"),
    ("昨收连续性", lambda r: abs(r.close / r.pre_close - 1) < 0.11 if r.pre_close > 0 else True, "与前日收盘差异>11%"),
]
```

### 8. 实施顺序

1. **v099 迁移** — `stocks_daily_k` 增加 `data_source` / `source_priority` / `data_quality` 三个字段 + Go Model 同步
2. **`base_adapter.py`** — 抽象基类 + `StandardKlineRow` 数据类
3. **`kline_writer.py`** — 优先级感知的统一写入引擎
4. **`tencent_adapter.py`** — 改造 `batch_collect.py` 的腾讯适配
5. **`tushare_adapter.py`** — 改造 `tushare_kline.py`，修复 DO NOTHING 为优先级写入
6. **`csv_adapter.py` + `csv_import.py`** — CSV 导入能力
7. **历史数据回填** — 对已有数据补充 `data_source='tencent'`, `source_priority=2`
8. **`config.yaml`** — 数据源切换配置
9. **前端入口** — 数据管理页面增加 CSV 上传 + 数据源切换开关

### 假设与约束

- 现有历史数据的 `data_source` 统一回填为 `'tencent'`，`source_priority=2`
- Tushare 付费接口开通后只需实现 `tushare_pro_adapter.py`，无需改动写入引擎
- 优先级覆盖是单向的（高优覆盖低优），不存在降级回退
- CSV 导入不自动触发，需手动操作并校验
