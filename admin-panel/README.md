# SEO HTML Generator - 管理后台

基于 Vue 3 + TypeScript + Element Plus 的管理后台。

## 技术栈

- Vue 3 + TypeScript
- Vite
- Element Plus
- Pinia (状态管理)
- Vue Router 4
- Axios
- ECharts (图表)
- Sass

## 开发

```bash
# 安装依赖
npm install

# 启动开发服务器
npm run dev

# 构建生产版本
npm run build
```

## 目录结构

```
src/
├── api/          # API 接口
├── components/   # 公共组件
├── router/       # 路由配置
├── stores/       # Pinia 状态管理
├── styles/       # 全局样式
├── types/        # TypeScript 类型
├── utils/        # 工具函数
└── views/        # 页面视图
```

## 功能模块

- 登录认证
- 仪表盘（数据统计、图表）
- 站点管理（CRUD）
- 关键词管理（池管理、批量添加）
- 文章管理（富文本编辑）
- 图片管理（URL批量添加、预览）
- 蜘蛛日志（统计、筛选、清理）
- 系统设置（缓存管理）

## 后端 API

确保后端服务运行在 `http://localhost:8009`，开发时会自动代理 `/api` 请求。
