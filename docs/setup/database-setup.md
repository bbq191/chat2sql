# 启动 PostgreSQL 服务

sudo systemctl start postgresql
sudo systemctl enable postgresql # 开机自启

# 检查服务状态

sudo systemctl status postgresql

# 切换到 postgres 用户

sudo -u postgres psql

# 在 PostgreSQL 命令行中执行：

CREATE DATABASE chat2sql_test;
CREATE USER postgres WITH PASSWORD 'password';
ALTER USER postgres PASSWORD 'password';
GRANT ALL PRIVILEGES ON DATABASE chat2sql_test TO postgres;
ALTER USER postgres CREATEDB; # 允许创建数据库
\q # 退出

# 应用数据库迁移

sudo -u postgres psql -d chat2sql_test -f migrations/001_create_tables.sql

# 测试连接是否正常

psql -h localhost -p 5432 -U postgres -d chat2sql_test
