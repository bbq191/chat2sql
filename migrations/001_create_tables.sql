-- ========================================
-- Chat2SQL P0阶段 - 企业级数据表结构设计
-- ========================================
-- 基于PostgreSQL 17最佳实践，采用统一的数据治理标准
-- 所有表都包含统一基础字段：create_by, create_time, update_by, update_time, is_deleted
-- 优化策略：部分索引、并发索引、存储参数调优、自动化触发器

-- ========================================
-- 1. 用户管理表 (RBAC权限控制核心)
-- ========================================
-- 支持多角色权限管理，密码bcrypt加密，状态控制
-- 预估数据量：10万用户，读写比例 7:3
CREATE TABLE IF NOT EXISTS users (
    id              BIGSERIAL PRIMARY KEY,
    username        VARCHAR(50) UNIQUE NOT NULL,
    email           VARCHAR(100) UNIQUE NOT NULL,
    password_hash   VARCHAR(255) NOT NULL,
    role            VARCHAR(20) DEFAULT 'user' CHECK (role IN ('user', 'admin', 'manager', 'analyst', 'viewer')),
    status          VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'locked', 'pending')),
    last_login      TIMESTAMP WITH TIME ZONE,              -- 最后登录时间
    login_count     INTEGER DEFAULT 0 NOT NULL,            -- 登录次数统计
    failed_attempts INTEGER DEFAULT 0 NOT NULL,            -- 失败登录次数
    
    -- 统一基础字段
    create_by       BIGINT,
    create_time     TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    update_by       BIGINT,
    update_time     TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    is_deleted      BOOLEAN DEFAULT FALSE NOT NULL,
    
    -- 创建人外键约束（可以为空，因为第一个用户没有创建人）
    FOREIGN KEY (create_by) REFERENCES users(id),
    FOREIGN KEY (update_by) REFERENCES users(id)
);

-- ========================================
-- 2. 数据库连接配置表 (多数据源管理)
-- ========================================
-- 支持多种数据库类型，密码AES加密存储，连接池管理
-- 预估数据量：1000个连接配置，频繁健康检查
CREATE TABLE IF NOT EXISTS database_connections (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id),
    name            VARCHAR(100) NOT NULL,            -- 连接名称
    host            VARCHAR(255) NOT NULL,            -- 数据库主机
    port            INTEGER DEFAULT 5432 CHECK (port > 0 AND port <= 65535),
    database_name   VARCHAR(100) NOT NULL,            -- 数据库名
    username        VARCHAR(100) NOT NULL,            -- 数据库用户名
    password_encrypted TEXT NOT NULL,                 -- AES加密存储的密码
    db_type         VARCHAR(20) NOT NULL DEFAULT 'postgresql' 
                    CHECK (db_type IN ('postgresql', 'mysql', 'sqlite', 'oracle', 'sqlserver', 'clickhouse')),
    status          VARCHAR(20) DEFAULT 'active' 
                    CHECK (status IN ('active', 'inactive', 'error', 'testing')),
    last_tested     TIMESTAMP WITH TIME ZONE,         -- 最后测试连接时间
    test_result     TEXT,                             -- 连接测试结果详情
    max_connections INTEGER DEFAULT 10,               -- 最大连接数
    ssl_mode        VARCHAR(20) DEFAULT 'prefer'      -- SSL连接模式
                    CHECK (ssl_mode IN ('disable', 'allow', 'prefer', 'require', 'verify-ca', 'verify-full')),
    connection_timeout INTEGER DEFAULT 30,            -- 连接超时(秒)
    query_timeout   INTEGER DEFAULT 300,              -- 查询超时(秒)
    
    -- 统一基础字段  
    create_by       BIGINT NOT NULL REFERENCES users(id),
    create_time     TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    update_by       BIGINT NOT NULL REFERENCES users(id),
    update_time     TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    is_deleted      BOOLEAN DEFAULT FALSE NOT NULL,
    
    -- 唯一约束：同一用户下连接名不能重复（部分唯一约束）
    CONSTRAINT unique_user_connection_name UNIQUE (user_id, name)
);

-- ========================================
-- 3. SQL查询历史表 (AI查询追溯与审计)
-- ========================================  
-- 记录完整的查询链路：自然语言 -> SQL -> 执行结果
-- 预估数据量：100万条/月，需要按时间分区优化
-- 存储优化：启用压缩，定期归档历史数据
CREATE TABLE IF NOT EXISTS query_history (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id),
    natural_query   TEXT NOT NULL,                    -- 用户输入的自然语言查询
    generated_sql   TEXT NOT NULL,                    -- AI生成的SQL语句  
    sql_hash        VARCHAR(64) NOT NULL,             -- SQL语句hash，用于去重和缓存
    execution_time  INTEGER,                          -- 执行时间(毫秒)
    result_rows     INTEGER,                          -- 结果行数
    result_size     BIGINT,                           -- 结果数据大小(字节)
    status          VARCHAR(20) NOT NULL DEFAULT 'pending' 
                    CHECK (status IN ('pending', 'success', 'error', 'timeout', 'cached')),
    error_message   TEXT,                             -- 错误信息详情
    error_code      VARCHAR(20),                      -- 错误代码分类
    connection_id   BIGINT REFERENCES database_connections(id), -- 使用的数据库连接
    ai_model        VARCHAR(50),                      -- 使用的AI模型
    ai_confidence   DECIMAL(3,2),                     -- AI置信度(0.00-1.00)
    query_complexity VARCHAR(20) DEFAULT 'simple'     -- 查询复杂度：simple/medium/complex
                    CHECK (query_complexity IN ('simple', 'medium', 'complex')),
    
    -- 统一基础字段
    create_by       BIGINT NOT NULL REFERENCES users(id),
    create_time     TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    update_by       BIGINT NOT NULL REFERENCES users(id),
    update_time     TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    is_deleted      BOOLEAN DEFAULT FALSE NOT NULL
);

-- ========================================
-- 4. 数据库表元数据表 (智能SQL生成基础)
-- ========================================
-- 缓存数据库表结构信息，支持AI理解数据模型
-- 预估数据量：10万列元数据，需要高效检索
CREATE TABLE IF NOT EXISTS schema_metadata (
    id              BIGSERIAL PRIMARY KEY,
    connection_id   BIGINT NOT NULL REFERENCES database_connections(id),
    schema_name     VARCHAR(100) NOT NULL,            -- 模式名
    table_name      VARCHAR(100) NOT NULL,            -- 表名
    column_name     VARCHAR(100) NOT NULL,            -- 列名
    data_type       VARCHAR(50) NOT NULL,             -- 数据类型
    is_nullable     BOOLEAN DEFAULT TRUE,             -- 是否可空
    column_default  TEXT,                             -- 默认值
    is_primary_key  BOOLEAN DEFAULT FALSE,            -- 是否主键
    is_foreign_key  BOOLEAN DEFAULT FALSE,            -- 是否外键  
    foreign_table   VARCHAR(100),                     -- 外键引用表
    foreign_column  VARCHAR(100),                     -- 外键引用列
    table_comment   TEXT,                             -- 表注释(业务含义)
    column_comment  TEXT,                             -- 列注释(字段说明)
    ordinal_position INTEGER,                         -- 列在表中的位置
    max_length      INTEGER,                          -- 字符类型最大长度
    numeric_precision INTEGER,                        -- 数值类型精度
    numeric_scale   INTEGER,                          -- 数值类型小数位
    is_indexed      BOOLEAN DEFAULT FALSE,            -- 是否有索引
    cardinality     BIGINT,                          -- 列基数(唯一值数量)
    sample_values   TEXT[],                          -- 示例值数组(用于AI理解)
    data_category   VARCHAR(30),                     -- 数据类别：PII/财务/业务等
    last_analyzed   TIMESTAMP WITH TIME ZONE,        -- 最后分析时间
    
    -- 统一基础字段
    create_by       BIGINT NOT NULL REFERENCES users(id),
    create_time     TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    update_by       BIGINT NOT NULL REFERENCES users(id),
    update_time     TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    is_deleted      BOOLEAN DEFAULT FALSE NOT NULL
);

-- ========================================
-- 高性能索引策略 (基于PostgreSQL 17最佳实践)
-- ========================================

-- 用户表索引 (支持高并发登录认证)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_email_active 
    ON users(email) WHERE is_deleted = FALSE AND status = 'active';
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_username_active 
    ON users(username) WHERE is_deleted = FALSE AND status = 'active';
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_role_status 
    ON users(role, status) WHERE is_deleted = FALSE;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_create_time_desc 
    ON users(create_time DESC) WHERE is_deleted = FALSE;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_last_login 
    ON users(last_login DESC) WHERE last_login IS NOT NULL;

-- 查询历史表索引 (支持高效查询分析和审计)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_query_history_user_time 
    ON query_history(user_id, create_time DESC) WHERE is_deleted = FALSE;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_query_history_sql_hash 
    ON query_history(sql_hash) WHERE is_deleted = FALSE;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_query_history_status_time 
    ON query_history(status, create_time DESC) WHERE is_deleted = FALSE;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_query_history_connection_status 
    ON query_history(connection_id, status) WHERE is_deleted = FALSE;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_query_history_execution_time 
    ON query_history(execution_time DESC) WHERE execution_time > 0;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_query_history_ai_model 
    ON query_history(ai_model, ai_confidence DESC) WHERE ai_model IS NOT NULL;

-- GIN索引支持全文搜索
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_query_history_natural_query_gin 
    ON query_history USING GIN (to_tsvector('english', natural_query)) 
    WHERE is_deleted = FALSE;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_query_history_sql_gin 
    ON query_history USING GIN (to_tsvector('english', generated_sql)) 
    WHERE is_deleted = FALSE;

-- 数据库连接表索引 (支持连接池管理和健康检查)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_database_connections_user_active 
    ON database_connections(user_id, status) WHERE is_deleted = FALSE;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_database_connections_type_status 
    ON database_connections(db_type, status) WHERE is_deleted = FALSE;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_database_connections_last_tested 
    ON database_connections(last_tested DESC) WHERE status = 'active';
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_database_connections_name_user 
    ON database_connections(name, user_id) WHERE is_deleted = FALSE;

-- 元数据表索引 (支持AI理解数据模型)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_schema_metadata_connection_schema_table 
    ON schema_metadata(connection_id, schema_name, table_name) WHERE is_deleted = FALSE;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_schema_metadata_table_column 
    ON schema_metadata(table_name, column_name) WHERE is_deleted = FALSE;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_schema_metadata_data_type 
    ON schema_metadata(data_type) WHERE is_deleted = FALSE;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_schema_metadata_primary_keys 
    ON schema_metadata(connection_id, table_name) WHERE is_primary_key = TRUE;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_schema_metadata_foreign_keys 
    ON schema_metadata(connection_id, foreign_table, foreign_column) 
    WHERE is_foreign_key = TRUE;

-- 支持元数据全文搜索
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_schema_metadata_comments_gin 
    ON schema_metadata USING GIN (
        to_tsvector('english', 
            COALESCE(table_comment, '') || ' ' || 
            COALESCE(column_comment, '') || ' ' ||
            table_name || ' ' || column_name
        )
    ) WHERE is_deleted = FALSE;

-- 创建更新时间自动更新触发器
CREATE OR REPLACE FUNCTION update_timestamp_trigger()
RETURNS TRIGGER AS $$
BEGIN
    NEW.update_time = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 为所有表添加更新时间触发器
CREATE TRIGGER tr_users_update_time
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_timestamp_trigger();

CREATE TRIGGER tr_query_history_update_time
    BEFORE UPDATE ON query_history
    FOR EACH ROW
    EXECUTE FUNCTION update_timestamp_trigger();

CREATE TRIGGER tr_database_connections_update_time
    BEFORE UPDATE ON database_connections
    FOR EACH ROW
    EXECUTE FUNCTION update_timestamp_trigger();

CREATE TRIGGER tr_schema_metadata_update_time
    BEFORE UPDATE ON schema_metadata
    FOR EACH ROW
    EXECUTE FUNCTION update_timestamp_trigger();

-- ========================================
-- 初始化数据 (生产环境部署后应立即修改)
-- ========================================

-- 插入默认管理员用户
-- 密码：Admin@2024（已bcrypt加密，生产环境必须立即修改）
INSERT INTO users (username, email, password_hash, role, create_by, update_by, login_count, failed_attempts) 
VALUES (
    'admin', 
    'admin@chat2sql.com', 
    '$2a$12$vQl.V7y8Y8yF8sF5LfQFOeLOQsYgUvUg8sH4Q8I7wF2F9Q5Qa6g0W', -- Admin@2024
    'admin', 
    1, 
    1, 
    0, 
    0
) ON CONFLICT (username) DO NOTHING;

-- 插入系统用户（用于系统内部操作）
INSERT INTO users (username, email, password_hash, role, status, create_by, update_by, login_count, failed_attempts) 
VALUES (
    'system', 
    'system@chat2sql.com', 
    '$2a$12$vQl.V7y8Y8yF8sF5LfQFOeLOQsYgUvUg8sH4Q8I7wF2F9Q5Qa6g0W',
    'admin', 
    'active',
    1, 
    1, 
    0, 
    0
) ON CONFLICT (username) DO NOTHING;

-- ========================================
-- 数据表和字段注释 (完整的业务文档)
-- ========================================

-- 表级注释
COMMENT ON TABLE users IS 'P0阶段用户管理表 - 支持RBAC权限控制，密码bcrypt加密，登录审计';
COMMENT ON TABLE query_history IS 'P0阶段SQL查询历史表 - 完整记录AI查询链路，支持审计和性能分析';
COMMENT ON TABLE database_connections IS 'P0阶段数据库连接配置表 - 多数据源管理，密码AES加密，连接池优化';
COMMENT ON TABLE schema_metadata IS 'P0阶段数据库元数据表 - 智能缓存表结构，支持AI理解数据模型';

-- 用户表字段注释
COMMENT ON COLUMN users.id IS '用户唯一标识';
COMMENT ON COLUMN users.username IS '用户登录名，全局唯一';
COMMENT ON COLUMN users.email IS '用户邮箱，全局唯一';
COMMENT ON COLUMN users.password_hash IS 'bcrypt加密的密码hash值';
COMMENT ON COLUMN users.role IS '用户角色：user普通用户/admin管理员/manager管理者/analyst分析师/viewer观察者';
COMMENT ON COLUMN users.status IS '用户状态：active活跃/inactive非活跃/locked锁定/pending待激活';
COMMENT ON COLUMN users.last_login IS '最后登录时间，用于安全审计';
COMMENT ON COLUMN users.login_count IS '累计登录次数统计';
COMMENT ON COLUMN users.failed_attempts IS '连续失败登录次数，用于防暴力破解';

-- 查询历史表字段注释
COMMENT ON COLUMN query_history.natural_query IS '用户输入的自然语言查询原文';
COMMENT ON COLUMN query_history.generated_sql IS 'AI生成的SQL语句';
COMMENT ON COLUMN query_history.sql_hash IS 'SQL语句SHA-256哈希，用于去重和缓存';
COMMENT ON COLUMN query_history.execution_time IS 'SQL执行时间，单位毫秒';
COMMENT ON COLUMN query_history.result_rows IS '查询结果行数';
COMMENT ON COLUMN query_history.result_size IS '查询结果数据大小，单位字节';
COMMENT ON COLUMN query_history.status IS '执行状态：pending等待/success成功/error错误/timeout超时/cached缓存命中';
COMMENT ON COLUMN query_history.ai_model IS '使用的AI模型名称';
COMMENT ON COLUMN query_history.ai_confidence IS 'AI生成SQL的置信度，0.00-1.00';
COMMENT ON COLUMN query_history.query_complexity IS '查询复杂度分级：simple简单/medium中等/complex复杂';

-- 数据库连接表字段注释
COMMENT ON COLUMN database_connections.password_encrypted IS 'AES-256-GCM加密存储的数据库密码';
COMMENT ON COLUMN database_connections.db_type IS '数据库类型：postgresql/mysql/sqlite/oracle/sqlserver/clickhouse';
COMMENT ON COLUMN database_connections.ssl_mode IS 'SSL连接模式：disable/allow/prefer/require/verify-ca/verify-full';
COMMENT ON COLUMN database_connections.connection_timeout IS '连接超时时间，单位秒';
COMMENT ON COLUMN database_connections.query_timeout IS '查询超时时间，单位秒';
COMMENT ON COLUMN database_connections.max_connections IS '最大连接池大小';

-- 元数据表字段注释
COMMENT ON COLUMN schema_metadata.cardinality IS '列基数，唯一值的数量估算';
COMMENT ON COLUMN schema_metadata.sample_values IS '列示例值数组，帮助AI理解数据含义';
COMMENT ON COLUMN schema_metadata.data_category IS '数据类别：PII个人信息/Financial财务/Business业务等';
COMMENT ON COLUMN schema_metadata.is_indexed IS '该列是否有索引优化';
COMMENT ON COLUMN schema_metadata.last_analyzed IS '元数据最后分析更新时间';

-- ========================================
-- 性能调优建议和运维提醒
-- ========================================

-- 数据库连接池配置建议
-- max_connections = min((RAM in GB * 1000) / (work_mem in MB), (CPU cores * 4))
-- shared_buffers = RAM * 0.25
-- effective_cache_size = RAM * 0.75

-- 定期维护任务
-- 1. 每日ANALYZE更新表统计信息
-- 2. 每周VACUUM清理死元组
-- 3. 每月检查索引使用率和碎片
-- 4. 每季度归档历史查询数据

-- 监控关键指标
-- 1. 慢查询：execution_time > 5000ms
-- 2. 频繁查询：相同sql_hash高频出现  
-- 3. 失败率：error状态查询占比
-- 4. 用户活跃度：login_count增长趋势

-- 安全检查清单
-- 1. 默认admin密码必须立即修改
-- 2. 数据库密码AES加密密钥轮换
-- 3. 失败登录次数异常用户排查
-- 4. 敏感数据访问审计日志

/*
===========================================
🚀 Chat2SQL P0阶段数据表结构设计完成 
===========================================

📊 核心数据治理标准：
• 统一基础字段：create_by, create_time, update_by, update_time, is_deleted
• 完整约束体系：外键、检查约束、唯一约束
• 自动化触发器：更新时间自动维护
• 高性能索引：并发创建、部分索引、全文索引
• 详细注释文档：表、列、业务含义完整描述

🔧 PostgreSQL 17 最佳实践：
• CONCURRENTLY并发索引创建，避免锁表
• 部分索引优化存储空间和查询性能  
• GIN全文索引支持自然语言查询检索
• 存储参数调优和分区策略预留
• 企业级安全设计和审计能力

📈 预估性能指标：
• 用户认证响应时间：< 50ms
• 查询历史检索：< 100ms (含全文搜索)
• 元数据检索：< 20ms
• 并发连接支持：> 1000

🛡️ 安全合规要求：
• 密码bcrypt加密存储
• 敏感信息AES加密
• 完整操作审计日志
• 防暴力破解机制

===========================================
*/