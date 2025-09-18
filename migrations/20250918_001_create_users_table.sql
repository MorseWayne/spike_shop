-- 用户表迁移
-- 支持基础注册/登录和简单 RBAC (admin/user)

CREATE TABLE IF NOT EXISTS `users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '用户ID',
  `username` varchar(64) NOT NULL COMMENT '用户名，唯一',
  `email` varchar(255) NOT NULL COMMENT '邮箱，唯一',
  `password_hash` varchar(255) NOT NULL COMMENT 'bcrypt 哈希后的密码',
  `role` enum('user', 'admin') NOT NULL DEFAULT 'user' COMMENT '用户角色',
  `is_active` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`),
  UNIQUE KEY `uk_email` (`email`),
  KEY `idx_role` (`role`),
  KEY `idx_is_active` (`is_active`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';

-- 插入默认管理员用户（密码为 "admin123"，实际生产环境应使用更强密码）
-- bcrypt hash for "admin123": $2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi
INSERT IGNORE INTO `users` (`username`, `email`, `password_hash`, `role`) VALUES 
('admin', 'admin@spike.local', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'admin');