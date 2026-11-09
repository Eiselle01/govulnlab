-- 0002_lab.sql —— 靶场专用表
-- 与业务表完全隔离：靶场里的注入、越权、脏数据不会污染真实业务数据。
-- 生产/演示环境把 lab.enabled=false 即可整体关闭（路由 404）。

CREATE TABLE IF NOT EXISTS lab_users (
    id       BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(32) NOT NULL,
    password VARCHAR(64) NOT NULL COMMENT '靶场故意用明文口令，用于演示 SQL 注入',
    role     VARCHAR(16) NOT NULL DEFAULT 'user',
    UNIQUE KEY uk_lab_username (username)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT = '靶场用户（明文口令，禁止用于真实业务）';

CREATE TABLE IF NOT EXISTS lab_orders (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id     BIGINT UNSIGNED NOT NULL,
    order_no    VARCHAR(40)  NOT NULL,
    goods_name  VARCHAR(128) NOT NULL,
    amount_cent BIGINT       NOT NULL,
    status      TINYINT      NOT NULL DEFAULT 1,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_lab_order_no (order_no),
    KEY idx_lab_user (user_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT = '靶场订单（用于越权演示）';

CREATE TABLE IF NOT EXISTS lab_notes (
    id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    author     VARCHAR(64)  NOT NULL,
    content    TEXT         NOT NULL,
    created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT = '靶场留言（用于存储型 XSS 演示）';

CREATE TABLE IF NOT EXISTS lab_profiles (
    user_id    BIGINT UNSIGNED PRIMARY KEY,
    nickname   VARCHAR(64) NOT NULL DEFAULT '',
    updated_at DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT = '靶场资料（用于 CSRF 演示）';
