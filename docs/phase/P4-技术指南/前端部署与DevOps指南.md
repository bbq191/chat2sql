# 🚀 前端部署与DevOps指南

## 🎯 部署架构概述

Chat2SQL前端采用现代化的容器化部署方案，支持自动化CI/CD、性能监控和弹性扩缩容。本指南详细介绍P4阶段的生产部署和DevOps最佳实践。

### ✨ 部署架构特点

| 组件 | 技术选型 | 核心优势 | 性能指标 |
|------|---------|---------|---------|
| **构建系统** | Vite 5.x + TypeScript | 极速构建，HMR支持 | 构建时间<30s |
| **容器化** | Docker + Multi-stage | 轻量镜像，安全隔离 | 镜像大小<50MB |
| **Web服务器** | Nginx + Gzip | 高性能静态文件服务 | 响应时间<50ms |
| **CDN加速** | 全球CDN分发 | 就近访问，缓存优化 | TTFB<200ms |

### 🎁 DevOps价值

- **自动化部署**：代码提交到生产环境全流程自动化
- **零停机更新**：蓝绿部署确保服务连续性  
- **性能监控**：实时监控前端性能和用户体验
- **安全可靠**：多层安全防护和备份恢复机制

---

## 🏗️ 生产构建优化

### 📦 Vite构建配置

```typescript
// vite.config.ts - 生产优化配置
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { resolve } from 'path';
import { visualizer } from 'rollup-plugin-visualizer';
import { compression } from 'vite-plugin-compression';

export default defineConfig({
  plugins: [
    react({
      // React 18.3并发特性支持
      babel: {
        plugins: [
          ['@babel/plugin-transform-react-jsx', { runtime: 'automatic' }]
        ]
      }
    }),
    
    // Gzip压缩
    compression({
      algorithm: 'gzip',
      ext: '.gz'
    }),
    
    // Brotli压缩
    compression({
      algorithm: 'brotliCompress',
      ext: '.br'
    }),
    
    // 构建分析
    visualizer({
      filename: 'dist/bundle-analysis.html',
      open: true,
      gzipSize: true
    })
  ],
  
  // 构建优化
  build: {
    target: 'es2020',
    outDir: 'dist',
    assetsDir: 'assets',
    
    // 代码分割策略
    rollupOptions: {
      output: {
        // 手动分包
        manualChunks: {
          // React核心库
          'react-vendor': ['react', 'react-dom'],
          
          // Ant Design组件库
          'antd-vendor': ['antd', '@ant-design/icons'],
          
          // 图表库
          'chart-vendor': ['echarts', 'echarts-for-react'],
          
          // 工具库
          'utils-vendor': ['lodash-es', 'dayjs', 'axios']
        },
        
        // 文件命名
        chunkFileNames: 'assets/js/[name]-[hash].js',
        entryFileNames: 'assets/js/[name]-[hash].js',
        assetFileNames: 'assets/[ext]/[name]-[hash].[ext]'
      }
    },
    
    // 压缩配置
    minify: 'terser',
    terserOptions: {
      compress: {
        drop_console: true,    // 移除console
        drop_debugger: true,   // 移除debugger
        pure_funcs: ['console.log'] // 移除特定函数调用
      },
      mangle: {
        safari10: true
      }
    },
    
    // 文件大小警告阈值
    chunkSizeWarningLimit: 500,
    
    // 资源内联阈值
    assetsInlineLimit: 4096
  },
  
  // 预览服务器配置
  preview: {
    port: 3000,
    strictPort: true,
    host: true
  },
  
  // 路径解析
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
      '@components': resolve(__dirname, 'src/components'),
      '@pages': resolve(__dirname, 'src/pages'),
      '@utils': resolve(__dirname, 'src/utils'),
      '@hooks': resolve(__dirname, 'src/hooks'),
      '@types': resolve(__dirname, 'src/types')
    }
  },
  
  // 环境变量
  define: {
    __APP_VERSION__: JSON.stringify(process.env.npm_package_version),
    __BUILD_TIME__: JSON.stringify(new Date().toISOString())
  }
});
```

### 🎯 资源优化策略

```typescript
// 资源优化配置
// src/utils/performance.ts

// 图片懒加载Hook
export function useLazyImage() {
  const [imageSrc, setImageSrc] = useState<string | null>(null);
  const [imageRef, isIntersecting] = useIntersectionObserver({
    threshold: 0.1,
    rootMargin: '50px'
  });
  
  useEffect(() => {
    if (isIntersecting && !imageSrc) {
      // 动态导入图片
      import('@/assets/images/placeholder.webp')
        .then(module => setImageSrc(module.default));
    }
  }, [isIntersecting, imageSrc]);
  
  return { imageRef, imageSrc };
}

// 代码分割工具
export const createAsyncComponent = <T extends ComponentType<any>>(
  importFunc: () => Promise<{ default: T }>,
  fallback?: ComponentType
) => {
  const LazyComponent = lazy(importFunc);
  
  return (props: ComponentProps<T>) => (
    <Suspense fallback={fallback ? <fallback /> : <div>Loading...</div>}>
      <LazyComponent {...props} />
    </Suspense>
  );
};

// 使用示例
export const QueryHistory = createAsyncComponent(
  () => import('@/pages/QueryHistory'),
  () => <div>加载查询历史...</div>
);

// 预加载关键资源
export function preloadCriticalResources() {
  // 预加载关键CSS
  const link = document.createElement('link');
  link.rel = 'preload';
  link.as = 'style';
  link.href = '/assets/css/critical.css';
  document.head.appendChild(link);
  
  // 预加载关键字体
  const fontLink = document.createElement('link');
  fontLink.rel = 'preload';
  fontLink.as = 'font';
  fontLink.type = 'font/woff2';
  fontLink.href = '/assets/fonts/roboto-v30-latin-regular.woff2';
  fontLink.crossOrigin = 'anonymous';
  document.head.appendChild(fontLink);
  
  // 预连接到API域名
  const preconnect = document.createElement('link');
  preconnect.rel = 'preconnect';
  preconnect.href = 'https://api.chat2sql.com';
  document.head.appendChild(preconnect);
}

// 资源提示
export function addResourceHints() {
  // DNS预解析
  const dnsPrefetch = document.createElement('link');
  dnsPrefetch.rel = 'dns-prefetch';
  dnsPrefetch.href = '//cdn.jsdelivr.net';
  document.head.appendChild(dnsPrefetch);
  
  // 预获取下一页资源
  const prefetch = document.createElement('link');
  prefetch.rel = 'prefetch';
  prefetch.href = '/assets/js/query-history-[hash].js';
  document.head.appendChild(prefetch);
}
```

### 📊 构建性能分析

```javascript
// 构建性能分析脚本
// scripts/build-analysis.js

const fs = require('fs');
const path = require('path');
const { exec } = require('child_process');

class BuildAnalyzer {
  constructor() {
    this.buildStartTime = Date.now();
    this.distPath = path.resolve(__dirname, '../dist');
  }
  
  async analyzeBuild() {
    console.log('🔍 开始构建分析...');
    
    const results = {
      buildTime: Date.now() - this.buildStartTime,
      bundleSize: this.calculateBundleSize(),
      chunkAnalysis: this.analyzeChunks(),
      assetAnalysis: this.analyzeAssets(),
      recommendations: []
    };
    
    this.generateRecommendations(results);
    this.generateReport(results);
    
    return results;
  }
  
  calculateBundleSize() {
    const totalSize = this.getFolderSize(this.distPath);
    const gzipSize = this.getGzipSize();
    
    return {
      total: this.formatSize(totalSize),
      gzipped: this.formatSize(gzipSize),
      compression: ((totalSize - gzipSize) / totalSize * 100).toFixed(1) + '%'
    };
  }
  
  analyzeChunks() {
    const jsDir = path.join(this.distPath, 'assets/js');
    if (!fs.existsSync(jsDir)) return [];
    
    const chunks = fs.readdirSync(jsDir)
      .filter(file => file.endsWith('.js'))
      .map(file => {
        const filePath = path.join(jsDir, file);
        const size = fs.statSync(filePath).size;
        
        return {
          name: file,
          size: this.formatSize(size),
          type: this.getChunkType(file)
        };
      })
      .sort((a, b) => this.parseSize(b.size) - this.parseSize(a.size));
    
    return chunks;
  }
  
  analyzeAssets() {
    const assetsDir = path.join(this.distPath, 'assets');
    const analysis = {
      images: [],
      fonts: [],
      css: []
    };
    
    if (fs.existsSync(assetsDir)) {
      this.walkDirectory(assetsDir, (filePath, stats) => {
        const ext = path.extname(filePath);
        const size = this.formatSize(stats.size);
        const name = path.basename(filePath);
        
        if (['.png', '.jpg', '.jpeg', '.svg', '.webp'].includes(ext)) {
          analysis.images.push({ name, size });
        } else if (['.woff', '.woff2', '.ttf'].includes(ext)) {
          analysis.fonts.push({ name, size });
        } else if (ext === '.css') {
          analysis.css.push({ name, size });
        }
      });
    }
    
    return analysis;
  }
  
  generateRecommendations(results) {
    const recommendations = [];
    
    // 检查大文件
    results.chunkAnalysis.forEach(chunk => {
      if (this.parseSize(chunk.size) > 500 * 1024) { // 500KB
        recommendations.push({
          type: 'warning',
          message: `${chunk.name} 文件过大 (${chunk.size})，建议进一步分包`
        });
      }
    });
    
    // 检查图片优化
    results.assetAnalysis.images.forEach(image => {
      if (this.parseSize(image.size) > 100 * 1024) { // 100KB
        recommendations.push({
          type: 'info',
          message: `${image.name} 图片较大 (${image.size})，建议使用WebP格式或图片压缩`
        });
      }
    });
    
    // 检查构建时间
    if (results.buildTime > 60000) { // 1分钟
      recommendations.push({
        type: 'warning',
        message: `构建时间过长 (${(results.buildTime / 1000).toFixed(1)}s)，建议优化构建配置`
      });
    }
    
    results.recommendations = recommendations;
  }
  
  generateReport(results) {
    const report = `
# 📊 构建分析报告

## 构建概览
- **构建时间**: ${(results.buildTime / 1000).toFixed(1)}s
- **总包大小**: ${results.bundleSize.total}
- **Gzip后大小**: ${results.bundleSize.gzipped}
- **压缩率**: ${results.bundleSize.compression}

## 代码分包分析
${results.chunkAnalysis.map(chunk => 
  `- **${chunk.name}** (${chunk.type}): ${chunk.size}`
).join('\n')}

## 资源分析
### 图片资源
${results.assetAnalysis.images.map(img => 
  `- ${img.name}: ${img.size}`
).join('\n')}

### CSS文件
${results.assetAnalysis.css.map(css => 
  `- ${css.name}: ${css.size}`
).join('\n')}

## 优化建议
${results.recommendations.map(rec => 
  `- ${rec.type === 'warning' ? '⚠️' : 'ℹ️'} ${rec.message}`
).join('\n')}

---
*生成时间: ${new Date().toLocaleString()}*
    `;
    
    fs.writeFileSync(path.join(this.distPath, 'build-report.md'), report);
    console.log('📋 构建报告已生成: dist/build-report.md');
  }
  
  // 工具方法
  getFolderSize(dirPath) {
    let totalSize = 0;
    this.walkDirectory(dirPath, (filePath, stats) => {
      totalSize += stats.size;
    });
    return totalSize;
  }
  
  walkDirectory(dirPath, callback) {
    const items = fs.readdirSync(dirPath);
    
    items.forEach(item => {
      const itemPath = path.join(dirPath, item);
      const stats = fs.statSync(itemPath);
      
      if (stats.isDirectory()) {
        this.walkDirectory(itemPath, callback);
      } else {
        callback(itemPath, stats);
      }
    });
  }
  
  formatSize(bytes) {
    const sizes = ['B', 'KB', 'MB', 'GB'];
    if (bytes === 0) return '0B';
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return Math.round(bytes / Math.pow(1024, i) * 100) / 100 + sizes[i];
  }
  
  parseSize(sizeStr) {
    const match = sizeStr.match(/^([\d.]+)([KMGT]?B)$/);
    if (!match) return 0;
    
    const value = parseFloat(match[1]);
    const unit = match[2];
    
    const multipliers = { 'B': 1, 'KB': 1024, 'MB': 1024**2, 'GB': 1024**3 };
    return value * (multipliers[unit] || 1);
  }
  
  getChunkType(filename) {
    if (filename.includes('vendor')) return 'vendor';
    if (filename.includes('runtime')) return 'runtime';
    if (filename.includes('main')) return 'main';
    return 'chunk';
  }
  
  getGzipSize() {
    // 简化实现，实际应该计算gzip文件大小
    return this.getFolderSize(this.distPath) * 0.3; // 假设压缩率30%
  }
}

// 执行分析
if (require.main === module) {
  const analyzer = new BuildAnalyzer();
  analyzer.analyzeBuild().then(results => {
    console.log('✅ 构建分析完成');
  }).catch(console.error);
}

module.exports = BuildAnalyzer;
```

---

## 🐳 Docker容器化部署

### 📦 多阶段构建Dockerfile

```dockerfile
# Dockerfile - 多阶段构建优化
FROM node:18-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装pnpm
RUN npm install -g pnpm

# 复制package文件
COPY package.json pnpm-lock.yaml ./

# 安装依赖
RUN pnpm install --frozen-lockfile

# 复制源代码
COPY . .

# 构建应用
RUN pnpm build

# 分析构建结果
RUN node scripts/build-analysis.js

# ======================================
# 生产阶段
FROM nginx:alpine AS production

# 安装必要的工具
RUN apk add --no-cache \
    curl \
    jq \
    bash

# 创建nginx用户和目录
RUN addgroup -g 1001 -S nginx && \
    adduser -S -D -H -u 1001 -h /var/cache/nginx -s /sbin/nologin -G nginx -g nginx nginx

# 复制nginx配置
COPY docker/nginx.conf /etc/nginx/nginx.conf
COPY docker/default.conf /etc/nginx/conf.d/default.conf

# 复制构建产物
COPY --from=builder /app/dist /usr/share/nginx/html

# 复制启动脚本
COPY docker/docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:80/health || exit 1

# 暴露端口
EXPOSE 80

# 启动命令
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]

# 添加标签
LABEL maintainer="chat2sql-team@example.com"
LABEL version="1.0.0"
LABEL description="Chat2SQL Frontend Application"
```

### 🔧 Docker Compose配置

```yaml
# docker-compose.yml
version: '3.8'

services:
  # 前端应用
  frontend:
    build:
      context: .
      dockerfile: Dockerfile
      target: production
    container_name: chat2sql-frontend
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./docker/ssl:/etc/nginx/ssl:ro
      - ./docker/logs:/var/log/nginx
      - ./docker/cache:/var/cache/nginx
    environment:
      - NODE_ENV=production
      - API_BASE_URL=https://api.chat2sql.com
      - ENABLE_GZIP=true
      - ENABLE_BROTLI=true
    networks:
      - chat2sql-network
    depends_on:
      - backend
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:80/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.frontend.rule=Host(`chat2sql.com`)"
      - "traefik.http.routers.frontend.tls=true"
      - "traefik.http.routers.frontend.tls.certresolver=letsencrypt"

  # 后端API服务
  backend:
    image: chat2sql-backend:latest
    container_name: chat2sql-backend
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - GO_ENV=production
      - DATABASE_URL=postgresql://user:pass@db:5432/chat2sql
      - REDIS_URL=redis://redis:6379
    networks:
      - chat2sql-network
    depends_on:
      - database
      - redis

  # 数据库
  database:
    image: postgres:17-alpine
    container_name: chat2sql-db
    restart: unless-stopped
    environment:
      - POSTGRES_DB=chat2sql
      - POSTGRES_USER=chat2sql_user
      - POSTGRES_PASSWORD_FILE=/run/secrets/db_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./docker/init.sql:/docker-entrypoint-initdb.d/init.sql
    networks:
      - chat2sql-network
    secrets:
      - db_password

  # Redis缓存
  redis:
    image: redis:7-alpine
    container_name: chat2sql-redis
    restart: unless-stopped
    volumes:
      - redis_data:/data
      - ./docker/redis.conf:/etc/redis/redis.conf
    command: redis-server /etc/redis/redis.conf
    networks:
      - chat2sql-network

  # 监控服务
  prometheus:
    image: prom/prometheus:latest
    container_name: chat2sql-prometheus
    restart: unless-stopped
    ports:
      - "9090:9090"
    volumes:
      - ./docker/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    networks:
      - chat2sql-network

  grafana:
    image: grafana/grafana:latest
    container_name: chat2sql-grafana
    restart: unless-stopped
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD_FILE=/run/secrets/grafana_password
    volumes:
      - grafana_data:/var/lib/grafana
      - ./docker/grafana/dashboards:/etc/grafana/provisioning/dashboards
      - ./docker/grafana/datasources:/etc/grafana/provisioning/datasources
    networks:
      - chat2sql-network
    secrets:
      - grafana_password

volumes:
  postgres_data:
  redis_data:
  prometheus_data:
  grafana_data:

networks:
  chat2sql-network:
    driver: bridge

secrets:
  db_password:
    file: ./secrets/db_password.txt
  grafana_password:
    file: ./secrets/grafana_password.txt
```

### 🚀 启动脚本

```bash
#!/bin/bash
# docker/docker-entrypoint.sh

set -e

# 等待后端服务启动
wait_for_service() {
    local host=$1
    local port=$2
    local timeout=${3:-30}
    
    echo "等待 $host:$port 服务启动..."
    
    for i in $(seq 1 $timeout); do
        if nc -z $host $port; then
            echo "$host:$port 服务已启动"
            return 0
        fi
        echo "等待中... ($i/$timeout)"
        sleep 1
    done
    
    echo "超时: $host:$port 服务未启动"
    return 1
}

# 环境变量处理
setup_environment() {
    echo "设置环境变量..."
    
    # API地址配置
    if [ -n "$API_BASE_URL" ]; then
        sed -i "s|API_BASE_URL_PLACEHOLDER|$API_BASE_URL|g" /usr/share/nginx/html/assets/js/*.js
    fi
    
    # 启用压缩
    if [ "$ENABLE_GZIP" = "true" ]; then
        sed -i 's/# gzip on;/gzip on;/' /etc/nginx/nginx.conf
    fi
    
    if [ "$ENABLE_BROTLI" = "true" ]; then
        sed -i 's/# brotli on;/brotli on;/' /etc/nginx/nginx.conf
    fi
}

# 健康检查端点
setup_health_check() {
    cat > /usr/share/nginx/html/health <<EOF
{
  "status": "healthy",
  "timestamp": "$(date -Iseconds)",
  "version": "${APP_VERSION:-unknown}",
  "build_time": "${BUILD_TIME:-unknown}"
}
EOF
}

# Nginx配置验证
validate_nginx_config() {
    echo "验证Nginx配置..."
    nginx -t
}

# 主启动流程
main() {
    echo "🚀 启动Chat2SQL前端应用..."
    
    # 等待后端服务
    if [ -n "$BACKEND_HOST" ] && [ -n "$BACKEND_PORT" ]; then
        wait_for_service "$BACKEND_HOST" "$BACKEND_PORT"
    fi
    
    # 设置环境
    setup_environment
    
    # 健康检查
    setup_health_check
    
    # 验证配置
    validate_nginx_config
    
    echo "✅ 启动完成，执行: $@"
    exec "$@"
}

# 执行主流程
main "$@"
```

---

## 🌐 Nginx配置优化

### ⚡ 高性能Nginx配置

```nginx
# docker/nginx.conf
user nginx;
worker_processes auto;
error_log /var/log/nginx/error.log warn;
pid /var/run/nginx.pid;

# 性能优化
worker_rlimit_nofile 65535;

events {
    worker_connections 4096;
    use epoll;
    multi_accept on;
}

http {
    # 基础配置
    include /etc/nginx/mime.types;
    default_type application/octet-stream;
    
    # 日志格式
    log_format main '$remote_addr - $remote_user [$time_local] "$request" '
                    '$status $body_bytes_sent "$http_referer" '
                    '"$http_user_agent" "$http_x_forwarded_for" '
                    'rt=$request_time uct="$upstream_connect_time" '
                    'uht="$upstream_header_time" urt="$upstream_response_time"';
    
    access_log /var/log/nginx/access.log main;
    
    # 性能优化
    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;
    keepalive_requests 1000;
    types_hash_max_size 2048;
    client_max_body_size 20M;
    
    # 缓冲区优化
    client_body_buffer_size 128k;
    client_header_buffer_size 3m;
    large_client_header_buffers 4 256k;
    output_buffers 1 32k;
    postpone_output 1460;
    
    # Gzip压缩
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_comp_level 6;
    gzip_types
        text/plain
        text/css
        text/xml
        text/javascript
        application/javascript
        application/xml+rss
        application/json
        application/atom+xml
        image/svg+xml;
    
    # Brotli压缩 (需要模块支持)
    # brotli on;
    # brotli_comp_level 4;
    # brotli_types
    #     text/plain
    #     text/css
    #     application/json
    #     application/javascript
    #     text/xml
    #     application/xml
    #     application/xml+rss
    #     text/javascript;
    
    # 安全头
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header Referrer-Policy "no-referrer-when-downgrade" always;
    add_header Content-Security-Policy "default-src 'self' http: https: data: blob: 'unsafe-inline'" always;
    
    # 隐藏Nginx版本
    server_tokens off;
    
    # 限流配置
    limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;
    limit_req_zone $binary_remote_addr zone=static:10m rate=30r/s;
    
    # 包含站点配置
    include /etc/nginx/conf.d/*.conf;
}
```

### 🏗️ 站点配置

```nginx
# docker/default.conf
# 缓存配置
map $sent_http_content_type $expires {
    default                    off;
    text/html                  epoch;
    text/css                   max;
    application/javascript     max;
    ~image/                    1M;
    ~font/                     1M;
    application/pdf            1M;
}

# 上游后端服务
upstream backend {
    server backend:8080;
    keepalive 32;
}

# HTTP服务器 (重定向到HTTPS)
server {
    listen 80;
    server_name chat2sql.com www.chat2sql.com;
    
    # 健康检查端点
    location /health {
        access_log off;
        return 200 "healthy\n";
        add_header Content-Type text/plain;
    }
    
    # Let's Encrypt验证
    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }
    
    # 重定向到HTTPS
    location / {
        return 301 https://$server_name$request_uri;
    }
}

# HTTPS服务器
server {
    listen 443 ssl http2;
    server_name chat2sql.com www.chat2sql.com;
    
    # SSL配置
    ssl_certificate /etc/nginx/ssl/fullchain.pem;
    ssl_certificate_key /etc/nginx/ssl/privkey.pem;
    ssl_session_timeout 1d;
    ssl_session_cache shared:MozTLS:10m;
    ssl_session_tickets off;
    
    # 现代SSL配置
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers off;
    
    # HSTS
    add_header Strict-Transport-Security "max-age=63072000" always;
    
    # 网站根目录
    root /usr/share/nginx/html;
    index index.html;
    
    # 缓存配置
    expires $expires;
    
    # 错误页面
    error_page 404 /404.html;
    error_page 500 502 503 504 /50x.html;
    
    # 健康检查
    location /health {
        access_log off;
        try_files /health =404;
    }
    
    # API代理
    location /api/ {
        limit_req zone=api burst=20 nodelay;
        
        proxy_pass http://backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
        
        # 超时配置
        proxy_connect_timeout 5s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
        
        # 缓冲配置
        proxy_buffering on;
        proxy_buffer_size 128k;
        proxy_buffers 4 256k;
        proxy_busy_buffers_size 256k;
    }
    
    # WebSocket代理
    location /ws/ {
        proxy_pass http://backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket特定配置
        proxy_read_timeout 86400;
    }
    
    # 静态资源缓存
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        limit_req zone=static burst=50 nodelay;
        
        expires 1y;
        add_header Cache-Control "public, immutable";
        add_header Vary "Accept-Encoding";
        
        # 跨域配置
        add_header Access-Control-Allow-Origin "*";
        add_header Access-Control-Allow-Methods "GET, OPTIONS";
        add_header Access-Control-Allow-Headers "DNT,User-Agent,X-Requested-With,If-Modified-Since,Cache-Control,Content-Type,Range";
        
        # 压缩
        gzip_static on;
        brotli_static on;
    }
    
    # HTML文件不缓存
    location ~* \.html$ {
        expires -1;
        add_header Cache-Control "no-cache, no-store, must-revalidate";
        add_header Pragma "no-cache";
    }
    
    # Service Worker
    location /sw.js {
        expires -1;
        add_header Cache-Control "no-cache, no-store, must-revalidate";
        add_header Pragma "no-cache";
    }
    
    # 前端路由支持
    location / {
        try_files $uri $uri/ /index.html;
    }
    
    # 禁止访问隐藏文件
    location ~ /\. {
        deny all;
        access_log off;
        log_not_found off;
    }
    
    # 禁止访问备份文件
    location ~ ~$ {
        deny all;
        access_log off;
        log_not_found off;
    }
}
```

---

## 🔄 CI/CD流水线

### 🛠️ GitHub Actions配置

```yaml
# .github/workflows/deploy.yml
name: 🚀 Deploy Frontend

on:
  push:
    branches: [main, develop]
    paths:
      - 'frontend/**'
      - '.github/workflows/deploy.yml'
  pull_request:
    branches: [main]
    paths:
      - 'frontend/**'

env:
  NODE_VERSION: '18'
  REGISTRY: ghcr.io
  IMAGE_NAME: chat2sql/frontend

jobs:
  # 代码质量检查
  quality-checks:
    name: 🔍 Quality Checks
    runs-on: ubuntu-latest
    steps:
      - name: Checkout代码
        uses: actions/checkout@v4
        
      - name: 设置Node.js
        uses: actions/setup-node@v4
        with:
          node-version: ${{ env.NODE_VERSION }}
          cache: 'pnpm'
          
      - name: 安装pnpm
        run: npm install -g pnpm
        
      - name: 安装依赖
        run: pnpm install --frozen-lockfile
        
      - name: 类型检查
        run: pnpm type-check
        
      - name: 代码检查
        run: pnpm lint
        
      - name: 代码格式检查
        run: pnpm format:check
        
      - name: 单元测试
        run: pnpm test:unit --coverage
        
      - name: 上传测试覆盖率
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage/lcov.info

  # 构建和测试
  build-and-test:
    name: 🏗️ Build & Test
    runs-on: ubuntu-latest
    needs: quality-checks
    strategy:
      matrix:
        node-version: [18, 20]
    steps:
      - name: Checkout代码
        uses: actions/checkout@v4
        
      - name: 设置Node.js ${{ matrix.node-version }}
        uses: actions/setup-node@v4
        with:
          node-version: ${{ matrix.node-version }}
          cache: 'pnpm'
          
      - name: 安装pnpm
        run: npm install -g pnpm
        
      - name: 安装依赖
        run: pnpm install --frozen-lockfile
        
      - name: 构建应用
        run: pnpm build
        env:
          VITE_API_BASE_URL: ${{ secrets.API_BASE_URL }}
          VITE_APP_VERSION: ${{ github.sha }}
          
      - name: E2E测试
        run: pnpm test:e2e
        
      - name: 性能测试
        run: pnpm test:performance
        
      - name: 可访问性测试
        run: pnpm test:a11y
        
      - name: 上传构建产物
        uses: actions/upload-artifact@v3
        with:
          name: build-artifacts-node-${{ matrix.node-version }}
          path: dist/
          retention-days: 7

  # 安全扫描
  security-scan:
    name: 🔒 Security Scan
    runs-on: ubuntu-latest
    needs: quality-checks
    steps:
      - name: Checkout代码
        uses: actions/checkout@v4
        
      - name: 运行Trivy漏洞扫描
        uses: aquasecurity/trivy-action@master
        with:
          scan-type: 'fs'
          scan-ref: '.'
          format: 'sarif'
          output: 'trivy-results.sarif'
          
      - name: 上传扫描结果
        uses: github/codeql-action/upload-sarif@v2
        with:
          sarif_file: 'trivy-results.sarif'
          
      - name: 依赖漏洞检查
        run: |
          npm audit --audit-level high
          pnpm audit --audit-level high

  # Docker镜像构建
  build-image:
    name: 🐳 Build Docker Image
    runs-on: ubuntu-latest
    needs: [build-and-test, security-scan]
    if: github.event_name == 'push'
    outputs:
      image: ${{ steps.image.outputs.image }}
      digest: ${{ steps.build.outputs.digest }}
    steps:
      - name: Checkout代码
        uses: actions/checkout@v4
        
      - name: 设置Docker Buildx
        uses: docker/setup-buildx-action@v3
        
      - name: 登录Container Registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
          
      - name: 提取元数据
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=ref,event=branch
            type=ref,event=pr
            type=sha
            type=raw,value=latest,enable={{is_default_branch}}
            
      - name: 构建并推送镜像
        id: build
        uses: docker/build-push-action@v5
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
          build-args: |
            BUILD_DATE=${{ github.event.head_commit.timestamp }}
            VCS_REF=${{ github.sha }}
            VERSION=${{ steps.meta.outputs.version }}
            
      - name: 输出镜像信息
        id: image
        run: |
          echo "image=${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ steps.meta.outputs.version }}" >> $GITHUB_OUTPUT

  # 部署到Staging环境
  deploy-staging:
    name: 🚀 Deploy to Staging
    runs-on: ubuntu-latest
    needs: build-image
    if: github.ref == 'refs/heads/develop'
    environment:
      name: staging
      url: https://staging.chat2sql.com
    steps:
      - name: 部署到Staging
        uses: ./.github/actions/deploy
        with:
          environment: staging
          image: ${{ needs.build-image.outputs.image }}
          kube-config: ${{ secrets.KUBE_CONFIG_STAGING }}
          
      - name: 运行冒烟测试
        run: |
          curl -f https://staging.chat2sql.com/health
          curl -f https://staging.chat2sql.com/api/health

  # 部署到Production环境
  deploy-production:
    name: 🚀 Deploy to Production
    runs-on: ubuntu-latest
    needs: [build-image, deploy-staging]
    if: github.ref == 'refs/heads/main'
    environment:
      name: production
      url: https://chat2sql.com
    steps:
      - name: 部署到Production
        uses: ./.github/actions/deploy
        with:
          environment: production
          image: ${{ needs.build-image.outputs.image }}
          kube-config: ${{ secrets.KUBE_CONFIG_PRODUCTION }}
          
      - name: 运行生产环境测试
        run: |
          curl -f https://chat2sql.com/health
          curl -f https://chat2sql.com/api/health
          
      - name: 通知部署成功
        uses: 8398a7/action-slack@v3
        with:
          status: success
          channel: '#deployments'
          text: '🎉 Frontend deployed to production successfully!'
        env:
          SLACK_WEBHOOK_URL: ${{ secrets.SLACK_WEBHOOK }}

  # 性能监控
  performance-monitoring:
    name: 📊 Performance Monitoring
    runs-on: ubuntu-latest
    needs: deploy-production
    if: github.ref == 'refs/heads/main'
    steps:
      - name: Lighthouse CI
        uses: treosh/lighthouse-ci-action@v10
        with:
          urls: |
            https://chat2sql.com
            https://chat2sql.com/query
          configPath: './lighthouserc.json'
          uploadArtifacts: true
          temporaryPublicStorage: true
          
      - name: 性能基准测试
        run: |
          npx @web/test-runner-performance-plugin \
            --url https://chat2sql.com \
            --budget ./performance-budget.json
```

### 🔧 自定义部署Action

```yaml
# .github/actions/deploy/action.yml
name: 'Deploy to Kubernetes'
description: '部署应用到Kubernetes集群'

inputs:
  environment:
    description: '部署环境 (staging/production)'
    required: true
  image:
    description: 'Docker镜像地址'
    required: true
  kube-config:
    description: 'Kubernetes配置'
    required: true

runs:
  using: 'composite'
  steps:
    - name: 设置kubectl
      uses: azure/setup-kubectl@v3
      with:
        version: 'v1.28.0'
        
    - name: 配置Kubernetes
      shell: bash
      run: |
        echo "${{ inputs.kube-config }}" | base64 -d > kubeconfig
        export KUBECONFIG=kubeconfig
        
    - name: 部署应用
      shell: bash
      run: |
        export KUBECONFIG=kubeconfig
        
        # 更新镜像
        kubectl set image deployment/frontend frontend=${{ inputs.image }} -n chat2sql-${{ inputs.environment }}
        
        # 等待部署完成
        kubectl rollout status deployment/frontend -n chat2sql-${{ inputs.environment }} --timeout=300s
        
        # 验证部署
        kubectl get pods -n chat2sql-${{ inputs.environment }} -l app=frontend
        
    - name: 验证健康状态
      shell: bash
      run: |
        if [ "${{ inputs.environment }}" = "staging" ]; then
          URL="https://staging.chat2sql.com"
        else
          URL="https://chat2sql.com"
        fi
        
        # 等待服务就绪
        for i in {1..30}; do
          if curl -f "$URL/health"; then
            echo "✅ 健康检查通过"
            break
          fi
          echo "等待服务就绪... ($i/30)"
          sleep 10
        done
        
    - name: 清理
      shell: bash
      if: always()
      run: rm -f kubeconfig
```

---

## 📊 性能监控与分析

### 🔍 Web Vitals监控

```typescript
// src/utils/performance-monitoring.ts

// Web Vitals监控
import { getCLS, getFID, getFCP, getLCP, getTTFB } from 'web-vitals';

interface PerformanceMetric {
  name: string;
  value: number;
  rating: 'good' | 'needs-improvement' | 'poor';
  timestamp: number;
  url: string;
  userAgent: string;
}

class PerformanceMonitor {
  private metrics: PerformanceMetric[] = [];
  private reportingEndpoint = '/api/performance';
  
  constructor() {
    this.initializeWebVitals();
    this.initializeCustomMetrics();
    this.startReporting();
  }
  
  private initializeWebVitals() {
    // 最大内容绘制
    getLCP((metric) => {
      this.recordMetric({
        name: 'LCP',
        value: metric.value,
        rating: metric.rating,
        timestamp: Date.now(),
        url: window.location.href,
        userAgent: navigator.userAgent
      });
    });
    
    // 首次输入延迟
    getFID((metric) => {
      this.recordMetric({
        name: 'FID',
        value: metric.value,
        rating: metric.rating,
        timestamp: Date.now(),
        url: window.location.href,
        userAgent: navigator.userAgent
      });
    });
    
    // 累积布局偏移
    getCLS((metric) => {
      this.recordMetric({
        name: 'CLS',
        value: metric.value,
        rating: metric.rating,
        timestamp: Date.now(),
        url: window.location.href,
        userAgent: navigator.userAgent
      });
    });
    
    // 首次内容绘制
    getFCP((metric) => {
      this.recordMetric({
        name: 'FCP',
        value: metric.value,
        rating: metric.rating,
        timestamp: Date.now(),
        url: window.location.href,
        userAgent: navigator.userAgent
      });
    });
    
    // 首字节时间
    getTTFB((metric) => {
      this.recordMetric({
        name: 'TTFB',
        value: metric.value,
        rating: metric.rating,
        timestamp: Date.now(),
        url: window.location.href,
        userAgent: navigator.userAgent
      });
    });
  }
  
  private initializeCustomMetrics() {
    // 查询执行时间
    window.addEventListener('query-start', () => {
      const startTime = performance.now();
      
      const handleQueryEnd = () => {
        const duration = performance.now() - startTime;
        this.recordMetric({
          name: 'Query Execution Time',
          value: duration,
          rating: duration < 1000 ? 'good' : duration < 3000 ? 'needs-improvement' : 'poor',
          timestamp: Date.now(),
          url: window.location.href,
          userAgent: navigator.userAgent
        });
        
        window.removeEventListener('query-end', handleQueryEnd);
      };
      
      window.addEventListener('query-end', handleQueryEnd);
    });
    
    // 页面导航时间
    this.measureNavigationTiming();
    
    // 资源加载时间
    this.measureResourceTiming();
  }
  
  private measureNavigationTiming() {
    window.addEventListener('load', () => {
      setTimeout(() => {
        const navigation = performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming;
        
        if (navigation) {
          // DNS查询时间
          this.recordMetric({
            name: 'DNS Lookup Time',
            value: navigation.domainLookupEnd - navigation.domainLookupStart,
            rating: 'good',
            timestamp: Date.now(),
            url: window.location.href,
            userAgent: navigator.userAgent
          });
          
          // 连接时间
          this.recordMetric({
            name: 'Connection Time',
            value: navigation.connectEnd - navigation.connectStart,
            rating: 'good',
            timestamp: Date.now(),
            url: window.location.href,
            userAgent: navigator.userAgent
          });
          
          // 页面加载时间
          this.recordMetric({
            name: 'Page Load Time',
            value: navigation.loadEventEnd - navigation.navigationStart,
            rating: 'good',
            timestamp: Date.now(),
            url: window.location.href,
            userAgent: navigator.userAgent
          });
        }
      }, 0);
    });
  }
  
  private measureResourceTiming() {
    const observer = new PerformanceObserver((list) => {
      list.getEntries().forEach((entry) => {
        if (entry.entryType === 'resource') {
          const resource = entry as PerformanceResourceTiming;
          
          // 只监控关键资源
          if (this.isCriticalResource(resource.name)) {
            this.recordMetric({
              name: `Resource Load Time - ${this.getResourceType(resource.name)}`,
              value: resource.responseEnd - resource.startTime,
              rating: 'good',
              timestamp: Date.now(),
              url: resource.name,
              userAgent: navigator.userAgent
            });
          }
        }
      });
    });
    
    observer.observe({ entryTypes: ['resource'] });
  }
  
  private recordMetric(metric: PerformanceMetric) {
    this.metrics.push(metric);
    
    // 立即发送关键指标
    if (['LCP', 'FID', 'CLS'].includes(metric.name)) {
      this.sendMetrics([metric]);
    }
  }
  
  private startReporting() {
    // 每30秒发送一次指标
    setInterval(() => {
      if (this.metrics.length > 0) {
        this.sendMetrics(this.metrics.splice(0));
      }
    }, 30000);
    
    // 页面卸载时发送剩余指标
    window.addEventListener('beforeunload', () => {
      if (this.metrics.length > 0) {
        navigator.sendBeacon(
          this.reportingEndpoint,
          JSON.stringify(this.metrics)
        );
      }
    });
  }
  
  private async sendMetrics(metrics: PerformanceMetric[]) {
    try {
      await fetch(this.reportingEndpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(metrics),
      });
    } catch (error) {
      console.warn('性能指标发送失败:', error);
    }
  }
  
  private isCriticalResource(url: string): boolean {
    return /\.(js|css|woff2?|png|jpg|svg)$/.test(url) ||
           url.includes('/api/') ||
           url.includes('chat2sql');
  }
  
  private getResourceType(url: string): string {
    if (url.endsWith('.js')) return 'JavaScript';
    if (url.endsWith('.css')) return 'CSS';
    if (/\.(woff2?|ttf|otf)$/.test(url)) return 'Font';
    if (/\.(png|jpg|jpeg|gif|svg|webp)$/.test(url)) return 'Image';
    if (url.includes('/api/')) return 'API';
    return 'Other';
  }
}

// 初始化性能监控
const performanceMonitor = new PerformanceMonitor();

export default performanceMonitor;
```

### 📈 错误监控和日志

```typescript
// src/utils/error-monitoring.ts

interface ErrorInfo {
  message: string;
  stack?: string;
  url: string;
  line?: number;
  column?: number;
  timestamp: number;
  userAgent: string;
  userId?: string;
  component?: string;
  props?: any;
}

class ErrorMonitor {
  private errorEndpoint = '/api/errors';
  private maxErrors = 50;
  private errorQueue: ErrorInfo[] = [];
  
  constructor() {
    this.initializeErrorHandlers();
  }
  
  private initializeErrorHandlers() {
    // JavaScript错误监控
    window.addEventListener('error', (event) => {
      this.captureError({
        message: event.message,
        stack: event.error?.stack,
        url: event.filename,
        line: event.lineno,
        column: event.colno,
        timestamp: Date.now(),
        userAgent: navigator.userAgent
      });
    });
    
    // Promise拒绝监控
    window.addEventListener('unhandledrejection', (event) => {
      this.captureError({
        message: `Unhandled Promise Rejection: ${event.reason}`,
        stack: event.reason?.stack,
        url: window.location.href,
        timestamp: Date.now(),
        userAgent: navigator.userAgent
      });
    });
    
    // 资源加载错误
    window.addEventListener('error', (event) => {
      if (event.target !== window) {
        this.captureError({
          message: `Resource Load Error: ${(event.target as any)?.src || (event.target as any)?.href}`,
          url: window.location.href,
          timestamp: Date.now(),
          userAgent: navigator.userAgent
        });
      }
    }, true);
  }
  
  // React错误边界支持
  captureReactError(error: Error, errorInfo: any) {
    this.captureError({
      message: error.message,
      stack: error.stack,
      url: window.location.href,
      timestamp: Date.now(),
      userAgent: navigator.userAgent,
      component: errorInfo.componentStack,
      props: errorInfo.props
    });
  }
  
  // 手动错误报告
  captureException(error: Error, context?: any) {
    this.captureError({
      message: error.message,
      stack: error.stack,
      url: window.location.href,
      timestamp: Date.now(),
      userAgent: navigator.userAgent,
      ...context
    });
  }
  
  private captureError(errorInfo: ErrorInfo) {
    // 过滤重复错误
    const isDuplicate = this.errorQueue.some(
      err => err.message === errorInfo.message && 
             err.url === errorInfo.url &&
             err.line === errorInfo.line
    );
    
    if (isDuplicate) return;
    
    this.errorQueue.push(errorInfo);
    
    // 限制队列大小
    if (this.errorQueue.length > this.maxErrors) {
      this.errorQueue.shift();
    }
    
    // 立即发送关键错误
    if (this.isCriticalError(errorInfo)) {
      this.sendErrors([errorInfo]);
    }
    
    console.error('Captured Error:', errorInfo);
  }
  
  private isCriticalError(error: ErrorInfo): boolean {
    return error.message.includes('ChunkLoadError') ||
           error.message.includes('Network Error') ||
           error.message.includes('TypeError') ||
           error.url.includes('/api/');
  }
  
  private async sendErrors(errors: ErrorInfo[]) {
    try {
      await fetch(this.errorEndpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(errors),
      });
    } catch (error) {
      console.warn('错误日志发送失败:', error);
    }
  }
  
  // 定期发送错误日志
  startReporting() {
    setInterval(() => {
      if (this.errorQueue.length > 0) {
        this.sendErrors(this.errorQueue.splice(0));
      }
    }, 60000); // 每分钟发送一次
    
    // 页面卸载时发送剩余错误
    window.addEventListener('beforeunload', () => {
      if (this.errorQueue.length > 0) {
        navigator.sendBeacon(
          this.errorEndpoint,
          JSON.stringify(this.errorQueue)
        );
      }
    });
  }
}

// React错误边界组件
export class ErrorBoundary extends Component<
  { children: ReactNode; fallback?: ComponentType<any> },
  { hasError: boolean; error?: Error }
> {
  constructor(props: any) {
    super(props);
    this.state = { hasError: false };
  }
  
  static getDerivedStateFromError(error: Error) {
    return { hasError: true, error };
  }
  
  componentDidCatch(error: Error, errorInfo: any) {
    errorMonitor.captureReactError(error, errorInfo);
  }
  
  render() {
    if (this.state.hasError) {
      const FallbackComponent = this.props.fallback || DefaultErrorFallback;
      return <FallbackComponent error={this.state.error} />;
    }
    
    return this.props.children;
  }
}

const DefaultErrorFallback = ({ error }: { error?: Error }) => (
  <div className="error-boundary">
    <h2>⚠️ 出现了错误</h2>
    <p>抱歉，应用遇到了意外错误。请刷新页面重试。</p>
    {process.env.NODE_ENV === 'development' && (
      <details>
        <summary>错误详情</summary>
        <pre>{error?.stack}</pre>
      </details>
    )}
  </div>
);

// 初始化错误监控
const errorMonitor = new ErrorMonitor();
errorMonitor.startReporting();

export default errorMonitor;
```

---

## 📚 最佳实践总结

### ✅ 构建优化最佳实践

1. **代码分割策略**
   - 按路由进行代码分割
   - 第三方库单独打包
   - 动态导入非关键组件

2. **资源优化**
   - 图片格式优化（WebP/AVIF）
   - 字体子集化和预加载
   - CSS和JS压缩

3. **缓存策略**
   - 静态资源长期缓存
   - HTML文件禁止缓存
   - API响应适当缓存

### ✅ 部署最佳实践

1. **容器化优化**
   - 多阶段构建减小镜像体积
   - 非root用户运行提升安全性
   - 健康检查确保服务可用

2. **服务配置**
   - Nginx性能调优
   - SSL/TLS安全配置
   - 负载均衡和故障转移

3. **监控告警**
   - 性能指标实时监控
   - 错误日志聚合分析
   - 自动化告警机制

### ❌ 常见陷阱

1. **性能问题**
   - 过大的bundle文件
   - 未优化的图片资源
   - 缺乏有效的缓存策略

2. **安全风险**
   - 暴露敏感信息
   - 缺乏安全头配置
   - 未及时更新依赖

3. **运维问题**
   - 缺乏监控和日志
   - 部署过程不够自动化
   - 没有有效的回滚机制

---

## 🔗 相关资源

- **Vite构建指南**：https://vitejs.dev/guide/build.html
- **Docker最佳实践**：https://docs.docker.com/develop/dev-best-practices/
- **Nginx配置指南**：https://nginx.org/en/docs/
- **Web性能优化**：https://web.dev/performance/
- **错误监控实践**：https://docs.sentry.io/platforms/javascript/

---

💡 **实施建议**：按照第4周的开发计划，先完成基础的构建和部署配置，然后逐步添加监控、告警和自动化功能，确保每个环节都经过充分测试。