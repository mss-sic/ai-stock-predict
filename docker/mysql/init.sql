-- 用户
CREATE TABLE IF NOT EXISTS users (
    id            BIGINT AUTO_INCREMENT PRIMARY KEY,
    username      VARCHAR(50) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 自选股
CREATE TABLE IF NOT EXISTS watchlists (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id    BIGINT NOT NULL,
    stock_code VARCHAR(10) NOT NULL,
    added_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_user_stock (user_id, stock_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 策略定义 (完整字段由 GORM AutoMigrate 管理)
CREATE TABLE IF NOT EXISTS strategies (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id    BIGINT NOT NULL DEFAULT 0,
    name       VARCHAR(100) NOT NULL,
    params     JSON NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ──────────────────────────────────────────────
-- 回测系统
-- ──────────────────────────────────────────────

-- 回测任务 (状态机，追踪异步执行)
CREATE TABLE IF NOT EXISTS backtest_tasks (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id         BIGINT NOT NULL DEFAULT 0,
    strategy_id     BIGINT NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending/running/completed/failed/cancelled
    phase           VARCHAR(100) DEFAULT '',                  -- 当前阶段描述
    current_day     INT DEFAULT 0,                            -- 当前回测天数
    total_days      INT DEFAULT 0,                            -- 总交易日数
    error_msg       VARCHAR(500) DEFAULT '',
    progress_pct    DOUBLE DEFAULT 0,                         -- 0-100
    initial_capital DOUBLE DEFAULT 0,                         -- 初始资金
    final_equity    DOUBLE DEFAULT 0,                         -- 最终权益
    total_return    DOUBLE DEFAULT 0,                         -- 总收益率%
    current_positions TEXT,                                   -- 最新持仓快照 JSON
    params          TEXT,                                     -- 回测参数 JSON
    result_id       BIGINT,                                   -- FK → backtest_results.id
    started_at      TIMESTAMP NULL,
    completed_at    TIMESTAMP NULL,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_task_strategy (strategy_id),
    INDEX idx_task_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 每日持仓快照 (按天记录，可构建权益曲线)
CREATE TABLE IF NOT EXISTS backtest_daily_snapshots (
    id                BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id           BIGINT NOT NULL,
    strategy_id       BIGINT NOT NULL DEFAULT 0,
    user_id           BIGINT NOT NULL DEFAULT 0,
    date              DATE NOT NULL,                           -- YYYY-MM-DD
    day_index         INT DEFAULT 0,                           -- 第N个交易日 (1-based)
    cash              DOUBLE DEFAULT 0,                        -- 剩余现金
    total_equity      DOUBLE DEFAULT 0,                        -- 总权益 (现金+持仓市值)
    daily_return      DOUBLE DEFAULT 0,                        -- 日收益率%
    cumulative_return DOUBLE DEFAULT 0,                        -- 累计收益率%
    position_count    INT DEFAULT 0,                           -- 持仓数
    positions         JSON,                                    -- 持仓明细 [{code,name,qty,costPrice,marketVal,pnl,pnlPct}]
    max_drawdown      DOUBLE DEFAULT 0,                        -- 当前最大回撤%
    created_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_snap_task_date (task_id, date),
    INDEX idx_snap_user (user_id),
    INDEX idx_snap_strategy (strategy_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 执行日志 (每日策略决策/成交/系统事件)
CREATE TABLE IF NOT EXISTS backtest_execution_logs (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id     BIGINT NOT NULL,
    strategy_id BIGINT NOT NULL DEFAULT 0,
    user_id     BIGINT NOT NULL DEFAULT 0,
    date        VARCHAR(10) DEFAULT '',                        -- YYYY-MM-DD, 系统事件为空
    seq         INT DEFAULT 0,                                 -- 同日排序
    log_type    VARCHAR(20) NOT NULL,                          -- condition_eval/signal/trade/system/error
    level       VARCHAR(10) DEFAULT 'info',                    -- info/warn/error/debug
    stock_code  VARCHAR(20) DEFAULT '',                        -- 个股代码, 系统事件为空
    stock_name  VARCHAR(50) DEFAULT '',                        -- 个股名称
    message     VARCHAR(1000) DEFAULT '',                      -- 可读消息
    detail      JSON,                                          -- 结构化详情
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_exec_task_date (task_id, date),
    INDEX idx_exec_user (user_id),
    INDEX idx_exec_strategy (strategy_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 回测结果汇总 (一次回测完成后存一行)
CREATE TABLE IF NOT EXISTS backtest_results (
    id                BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id           BIGINT NOT NULL,                         -- FK → backtest_tasks.id
    user_id           BIGINT NOT NULL DEFAULT 0,
    strategy_id       BIGINT NOT NULL,
    stock_pool        VARCHAR(30) NOT NULL DEFAULT '',  -- pool key: all/watchlist_N/portfolio/codes
    stock_pool_params JSON,                                    -- 实际股票列表
    start_date        DATE NOT NULL,
    end_date          DATE NOT NULL,
    initial_capital   DOUBLE DEFAULT 0,
    final_equity      DOUBLE DEFAULT 0,
    total_return      DOUBLE DEFAULT 0,
    sharpe_ratio      DOUBLE DEFAULT 0,
    max_drawdown      DOUBLE DEFAULT 0,
    win_rate          DOUBLE DEFAULT 0,
    trade_count       INT DEFAULT 0,
    trades            JSON,                                    -- 交易明细
    equity_curve      JSON,                                    -- 权益曲线
    coverage          JSON,                                    -- 指标覆盖率
    created_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_result_task (task_id),
    INDEX idx_result_user (user_id),
    INDEX idx_result_strategy (strategy_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ──────────────────────────────────────────────
-- 未来功能预留
-- ──────────────────────────────────────────────

-- 策略实盘运行 (每日自动执行)
CREATE TABLE IF NOT EXISTS strategy_runs (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id         BIGINT NOT NULL DEFAULT 0,
    strategy_id     BIGINT NOT NULL,
    name            VARCHAR(100) DEFAULT '',                   -- 运行标签
    status          VARCHAR(20) DEFAULT 'active',             -- active/paused/stopped/archived
    stock_pool      VARCHAR(500) DEFAULT '',                   -- 股票池标识
    start_date      VARCHAR(10) DEFAULT '',
    end_date        VARCHAR(10) DEFAULT '',
    initial_capital DOUBLE DEFAULT 0,
    current_equity  DOUBLE DEFAULT 0,
    total_return    DOUBLE DEFAULT 0,
    sharpe_ratio    DOUBLE DEFAULT 0,
    max_drawdown    DOUBLE DEFAULT 0,
    win_rate        DOUBLE DEFAULT 0,
    trade_count     INT DEFAULT 0,
    last_run_date   VARCHAR(10) DEFAULT '',
    last_error      VARCHAR(500) DEFAULT '',
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_run_strategy (strategy_id),
    INDEX idx_run_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 策略PK对比
CREATE TABLE IF NOT EXISTS strategy_comparisons (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id     BIGINT NOT NULL DEFAULT 0,
    name        VARCHAR(100) DEFAULT '',
    description VARCHAR(500) DEFAULT '',
    run_ids     JSON,                                          -- 参与对比的 result/run ID 列表
    start_date  VARCHAR(10) DEFAULT '',
    end_date    VARCHAR(10) DEFAULT '',
    benchmark   VARCHAR(20) DEFAULT '',                        -- 基准指数
    metrics     JSON,                                          -- 聚合对比指标
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_comp_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ──────────────────────────────────────────────
-- 其他
-- ──────────────────────────────────────────────

-- 持仓
CREATE TABLE IF NOT EXISTS holdings (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id     BIGINT NOT NULL DEFAULT 0,
    stock_code  VARCHAR(10) NOT NULL,
    cost_price  NUMERIC(12,4) NOT NULL,
    quantity    INT NOT NULL DEFAULT 0,
    strategy_id BIGINT DEFAULT 0,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 风险预警
CREATE TABLE IF NOT EXISTS risk_alerts (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    stock_code  VARCHAR(10) NOT NULL,
    level       VARCHAR(10) NOT NULL DEFAULT 'low',
    type        VARCHAR(50) NOT NULL,
    description TEXT,
    hit_date    DATE NOT NULL,
    ignored     TINYINT DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 导入日志
CREATE TABLE IF NOT EXISTS import_logs (
    id            BIGINT AUTO_INCREMENT PRIMARY KEY,
    file_name     VARCHAR(255) NOT NULL,
    rows_imported INT DEFAULT 0,
    status        VARCHAR(20) NOT NULL DEFAULT 'pending',
    error_msg     TEXT,
    imported_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
