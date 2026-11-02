-- 可选。默认关闭，不建这个表也能跑。
-- 程序在 WHALE_MYSQL_ENABLED=true 且连上库时也会自己 CREATE TABLE IF NOT EXISTS。
-- 先自己建好库：CREATE DATABASE whale_live CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS whale_interactions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  user_name VARCHAR(64) NOT NULL DEFAULT '',
  source VARCHAR(32) NOT NULL DEFAULT '',
  danmaku TEXT NOT NULL,
  reply TEXT NOT NULL,
  mock TINYINT(1) NOT NULL DEFAULT 0,
  degraded TINYINT(1) NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  KEY idx_whale_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
