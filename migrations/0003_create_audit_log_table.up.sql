CREATE TABLE IF NOT EXISTS audit_log (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    action VARCHAR(255) NOT NULL COMMENT '操作类型：login / create_order / update_pwd',
    detail TEXT COMMENT '操作详情，比如订单号、描述信息',
    create_time DATETIME DEFAULT CURRENT_TIMESTAMP,
    ip_addr VARCHAR(64) COMMENT '操作IP，安全审计很有用',
)ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;;