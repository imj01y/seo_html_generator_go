# 简化蜘蛛检测器

## 目标

将蜘蛛检测从"YAML配置 + 正则编译 + LRU缓存 + fsnotify热加载"简化为纯关键词 `strings.Contains` 匹配。

## 变更清单

### 删除
- `api/internal/service/spider_config.go` — 整个文件
- `api/pkg/config/spiders.yaml` — 配置文件

### 重写
- `api/internal/service/spider_detector.go` — 硬编码关键词 + strings.Contains

### 微调
- `handler/spider_detector.go` — GetSpiderConfig 简化返回（去缓存统计）

### 不动
- router.go `/api/spiders/*` 路由、日志/统计/趋势 API、前端

## 检测逻辑

```
lowerUA = strings.ToLower(ua)
遍历关键词表: strings.Contains(lowerUA, keyword) → 命中返回 type+name
```

关键词全部存小写，UA 转小写后匹配，兼容大小写。
