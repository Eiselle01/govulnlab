-- 0001_init.sql —— 业务主表
-- 约定：
--   1. 金额一律用「分」存 BIGINT，禁止 FLOAT/DOUBLE 存钱（0.1+0.2 的经典误差）；
--   2. 对外只暴露 order_no（业务号），不暴露自增 id，避免被顺序遍历；
--   3. 审计表 user_id 不建外键：登录失败等场景没有合法 user_id。

CREATE TABLE IF NOT EXISTS users (
    id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    username      VARCHAR(32)  NOT NULL,
    password_hash VARCHAR(100) NOT NULL COMMENT 'bcrypt 哈希，永不存明文',
    role          VARCHAR(16)  NOT NULL DEFAULT 'user' COMMENT 'user / admin',
    status        TINYINT      NOT NULL DEFAULT 1 COMMENT '1 正常 / 0 禁用',
    created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_username (username)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT = '用户';

CREATE TABLE IF NOT EXISTS orders (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    order_no    VARCHAR(40)  NOT NULL COMMENT '业务订单号，前端只认这个',
    user_id     BIGINT UNSIGNED NOT NULL,
    goods_name  VARCHAR(128) NOT NULL,
    amount_cent BIGINT       NOT NULL COMMENT '金额，单位分',
    status      TINYINT      NOT NULL DEFAULT 1 COMMENT '1 待支付 / 2 已支付 / 3 已取消',
    remark      VARCHAR(255) NOT NULL DEFAULT '',
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_order_no (order_no),
    KEY idx_user_created (user_id, created_at),
    CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT = '订单';

CREATE TABLE IF NOT EXISTS guestbook (
    id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id    BIGINT UNSIGNED NOT NULL,
    content    VARCHAR(1000) NOT NULL,
    created_at DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_user_created (user_id, created_at),
    CONSTRAINT fk_guestbook_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT = '留言板';

CREATE TABLE IF NOT EXISTS audit_log (
    id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id    BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0 表示未登录事件',
    action     VARCHAR(64)  NOT NULL COMMENT 'login / create_order / change_password ...',
    detail     VARCHAR(512) NOT NULL DEFAULT '',
    ip         VARCHAR(64)  NOT NULL DEFAULT '',
    ua         VARCHAR(255) NOT NULL DEFAULT '',
    request_id VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '与访问日志对账用',
    created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_user_time (user_id, created_at),
    KEY idx_action_time (action, created_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT = '审计日志';
