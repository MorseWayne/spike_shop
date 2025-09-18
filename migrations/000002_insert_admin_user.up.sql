-- 插入默认管理员用户
-- 密码为 "admin123"，实际生产环境应使用更强密码
-- bcrypt hash for "admin123": $2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi

INSERT IGNORE INTO `users` (`username`, `email`, `password_hash`, `role`) VALUES 
('admin', 'admin@spike.local', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'admin');
