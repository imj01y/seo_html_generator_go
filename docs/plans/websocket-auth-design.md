# WebSocket 首条消息认证方案

## 概述

浏览器 WebSocket API 无法在握手时传递自定义 Header（如 `Authorization`），因此采用「首条消息认证」方案：先建立连接，再通过第一条消息传递 JWT Token 进行认证。

## 通信协议

```
客户端                                     服务端
   |                                          |
   |-------- WebSocket 握手 (无认证) -------->|
   |<-------- 连接建立成功 -------------------|
   |                                          |
   |-- {"type":"auth","token":"jwt..."} ---->|
   |                                          | 验证 JWT
   |<-- {"type":"auth_ok"} ------------------|  (成功)
   |                                          |
   |-- 业务消息 ----------------------------->|
   |<-- 业务响应 -----------------------------|
```

认证失败时：
```
   |<-- {"type":"auth_fail","message":"..."} -|
   |          (服务端关闭连接)                 |
```

## 消息格式

### 认证请求（客户端 → 服务端）
```json
{
  "type": "auth",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### 认证成功（服务端 → 客户端）
```json
{
  "type": "auth_ok"
}
```

### 认证失败（服务端 → 客户端）
```json
{
  "type": "auth_fail",
  "message": "Token 已过期"
}
```

## 超时设计

- **认证超时**：10 秒（连接后 10 秒内必须完成认证）
- **业务超时**：根据具体业务设置

## 后端实现（Go + Gin + Gorilla WebSocket）

### 路由配置
```go
// 不使用 AuthMiddleware，认证在 handler 内部完成
r.GET("/ws/your-endpoint", yourHandler.HandleWebSocket)
```

### Handler 实现
```go
func (h *YourHandler) HandleWebSocket(c *gin.Context) {
    // 获取配置用于 JWT 验证
    cfg, exists := c.Get("config")
    if !exists {
        c.JSON(500, gin.H{"error": "配置未加载"})
        return
    }
    secret := cfg.(*config.Config).Auth.SecretKey

    // 升级为 WebSocket
    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    // 设置认证超时（10 秒）
    conn.SetReadDeadline(time.Now().Add(10 * time.Second))

    // 读取认证消息
    var authReq struct {
        Type  string `json:"type"`
        Token string `json:"token"`
    }
    if err := conn.ReadJSON(&authReq); err != nil {
        h.sendAuthFail(conn, "读取认证消息失败")
        return
    }

    if authReq.Type != "auth" || authReq.Token == "" {
        h.sendAuthFail(conn, "首条消息必须是认证请求")
        return
    }

    // 验证 JWT Token
    _, err = core.VerifyToken(authReq.Token, secret)
    if err != nil {
        if err == core.ErrTokenExpired {
            h.sendAuthFail(conn, "Token 已过期")
        } else {
            h.sendAuthFail(conn, "无效的 Token")
        }
        return
    }

    // 认证成功
    conn.WriteJSON(map[string]string{"type": "auth_ok"})

    // 清除读取超时，恢复正常
    conn.SetReadDeadline(time.Time{})

    // 继续处理业务逻辑...
}

func (h *YourHandler) sendAuthFail(conn *websocket.Conn, msg string) {
    conn.WriteJSON(map[string]string{
        "type":    "auth_fail",
        "message": msg,
    })
}
```

## 前端实现（TypeScript）

```typescript
interface WSHandlers {
  onMessage: (data: any) => void
  onError?: (error: string) => void
  onClose?: () => void
}

export function connectWithAuth(
  url: string,
  handlers: WSHandlers
): { send: (data: any) => void; close: () => void } {
  const token = localStorage.getItem('token')

  if (!token) {
    handlers.onError?.('未登录，请先登录')
    return { send: () => {}, close: () => {} }
  }

  let ws: WebSocket | null = null
  let authenticated = false

  try {
    ws = new WebSocket(url)
  } catch (e) {
    handlers.onError?.(`WebSocket 创建失败: ${e}`)
    return { send: () => {}, close: () => {} }
  }

  ws.onopen = () => {
    // 发送认证消息
    ws?.send(JSON.stringify({ type: 'auth', token }))
  }

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)

      if (msg.type === 'auth_ok') {
        authenticated = true
        return
      }

      if (msg.type === 'auth_fail') {
        handlers.onError?.(`认证失败: ${msg.message || '未知错误'}`)
        ws?.close()
        return
      }

      // 业务消息
      handlers.onMessage(msg)
    } catch {
      // 忽略解析错误
    }
  }

  ws.onerror = () => {
    handlers.onError?.('WebSocket 连接失败')
  }

  ws.onclose = () => {
    handlers.onClose?.()
  }

  return {
    send: (data: any) => {
      if (authenticated && ws?.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify(data))
      }
    },
    close: () => {
      ws?.close()
    }
  }
}
```

## Nginx 配置

确保 WebSocket 路由正确代理：

```nginx
location /ws/ {
    resolver 127.0.0.11 valid=10s ipv6=off;
    set $upstream_app api:8080;
    proxy_pass http://$upstream_app;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_read_timeout 86400;
}
```

## 安全考虑

1. **Token 不暴露在 URL** - 避免出现在日志、浏览器历史中
2. **认证超时** - 防止连接挂起占用资源
3. **服务端主动关闭** - 认证失败后立即关闭连接

## 适用场景

- 需要 JWT 认证的 WebSocket 连接
- 实时日志推送
- 实时状态更新
- 长连接任务监控
