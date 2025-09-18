-- 删除管理员用户

DELETE FROM `users` WHERE `username` = 'admin' AND `email` = 'admin@spike.local';
