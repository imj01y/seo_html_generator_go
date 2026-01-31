package core

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// PoolReloader 池配置热更新监听器
type PoolReloader struct {
	redis         *redis.Client
	templateFuncs *TemplateFuncsManager
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewPoolReloader 创建热更新监听器
func NewPoolReloader(rdb *redis.Client, templateFuncs *TemplateFuncsManager) *PoolReloader {
	ctx, cancel := context.WithCancel(context.Background())
	return &PoolReloader{
		redis:         rdb,
		templateFuncs: templateFuncs,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start 启动监听
func (r *PoolReloader) Start() {
	go r.listen()
	log.Info().Msg("Pool reloader started, listening on pool:reload channel")
}

// Stop 停止监听
func (r *PoolReloader) Stop() {
	r.cancel()
	log.Info().Msg("Pool reloader stopped")
}

// listen 监听 Redis 消息
func (r *PoolReloader) listen() {
	pubsub := r.redis.Subscribe(r.ctx, "pool:reload")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-r.ctx.Done():
			return
		case msg := <-ch:
			if msg == nil {
				// Channel closed, stop listening
				return
			}
			r.handleMessage(msg.Payload)
		}
	}
}

// poolReloadMessage Redis 消息结构
type poolReloadMessage struct {
	Action string `json:"action"`
	Sizes  struct {
		ClsPoolSize          int `json:"cls_pool_size"`
		URLPoolSize          int `json:"url_pool_size"`
		KeywordEmojiPoolSize int `json:"keyword_emoji_pool_size"`
		NumberPoolSize       int `json:"number_pool_size"`
	} `json:"sizes"`
}

// handleMessage 处理消息
func (r *PoolReloader) handleMessage(payload string) {
	var msg poolReloadMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		log.Error().Err(err).Msg("Failed to parse pool reload message")
		return
	}

	if msg.Action != "reload" {
		log.Debug().Str("action", msg.Action).Msg("Ignoring non-reload message")
		return
	}

	log.Info().
		Int("cls", msg.Sizes.ClsPoolSize).
		Int("url", msg.Sizes.URLPoolSize).
		Int("keyword_emoji", msg.Sizes.KeywordEmojiPoolSize).
		Int("number", msg.Sizes.NumberPoolSize).
		Msg("Applying pool size configuration")

	// 调整 Go 对象池大小
	if r.templateFuncs != nil {
		r.templateFuncs.ResizePools(&PoolSizeConfig{
			ClsPoolSize:          msg.Sizes.ClsPoolSize,
			URLPoolSize:          msg.Sizes.URLPoolSize,
			KeywordEmojiPoolSize: msg.Sizes.KeywordEmojiPoolSize,
			NumberPoolSize:       msg.Sizes.NumberPoolSize,
		})
	}

	log.Info().Msg("Pool configuration applied successfully")
}
