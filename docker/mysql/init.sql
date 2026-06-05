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

-- 策略定义
CREATE TABLE IF NOT EXISTS strategies (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id    BIGINT NOT NULL DEFAULT 0,
    name       VARCHAR(100) NOT NULL,
    params     JSON NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 回测结果
CREATE TABLE IF NOT EXISTS backtest_results (
    id           BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id      BIGINT NOT NULL DEFAULT 0,
    strategy_id  BIGINT NOT NULL,
    stock_code   VARCHAR(10) NOT NULL,
    start_date   DATE NOT NULL,
    end_date     DATE NOT NULL,
    total_return NUMERIC(10,4) DEFAULT 0,
    sharpe_ratio NUMERIC(8,4) DEFAULT 0,
    max_drawdown NUMERIC(10,4) DEFAULT 0,
    win_rate     NUMERIC(8,4) DEFAULT 0,
    trade_count  INT DEFAULT 0,
    trades       JSON,
    equity_curve JSON,
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

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
