// Package router 提供秒杀相关的路由注册
package router

import (
	"github.com/gin-gonic/gin"

	"github.com/MorseWayne/spike_shop/internal/api"
	"github.com/MorseWayne/spike_shop/internal/limiter"
	"github.com/MorseWayne/spike_shop/internal/middleware"
)

// RegisterSpikeRoutes 注册秒杀相关路由
func RegisterSpikeRoutes(
	r *gin.RouterGroup,
	spikeHandler *api.SpikeHandler,
	jwtMiddleware gin.HandlerFunc,
	adminMiddleware gin.HandlerFunc,
	spikeLimiter limiter.Limiter,
	apiLimiter limiter.Limiter,
) {
	// 秒杀API路由组
	spikeGroup := r.Group("/spike")
	{
		// 健康检查（无需认证）
		spikeGroup.GET("/health", spikeHandler.HealthCheck)

		// 公开接口（无需认证）
		public := spikeGroup.Group("")
		{
			// 获取活跃秒杀活动列表
			public.GET("/events",
				limiter.APIRateLimitMiddleware(apiLimiter),
				spikeHandler.GetActiveEvents)

			// 获取秒杀活动详情
			public.GET("/events/:id",
				limiter.APIRateLimitMiddleware(apiLimiter),
				spikeHandler.GetSpikeEventDetail)

			// 获取秒杀统计信息
			public.GET("/events/:id/stats",
				limiter.APIRateLimitMiddleware(apiLimiter),
				spikeHandler.GetSpikeStats)
		}

		// 需要用户认证的接口
		authenticated := spikeGroup.Group("")
		authenticated.Use(jwtMiddleware)
		{
			// 参与秒杀（重要接口，使用专门的秒杀限流）
			authenticated.POST("/participate",
				limiter.SpikeRateLimitMiddleware(spikeLimiter),
				middleware.IdempotencyMiddleware(),
				spikeHandler.ParticipateSpike)

			// 用户订单相关
			orders := authenticated.Group("/orders")
			{
				// 获取用户秒杀订单列表
				orders.GET("",
					limiter.APIRateLimitMiddleware(apiLimiter),
					spikeHandler.GetUserSpikeOrders)

				// 获取秒杀订单详情
				orders.GET("/:id",
					limiter.APIRateLimitMiddleware(apiLimiter),
					spikeHandler.GetSpikeOrderDetail)

				// 取消秒杀订单
				orders.POST("/:id/cancel",
					limiter.APIRateLimitMiddleware(apiLimiter),
					middleware.IdempotencyMiddleware(),
					spikeHandler.CancelSpikeOrder)
			}
		}
	}

	// 管理员接口
	adminGroup := r.Group("/admin/spike")
	adminGroup.Use(jwtMiddleware, adminMiddleware)
	{
		// 库存预热
		adminGroup.POST("/events/:id/warmup",
			limiter.APIRateLimitMiddleware(apiLimiter),
			spikeHandler.WarmupStock)
	}
}

// RegisterSpikeRoutesWithConfig 使用配置注册秒杀路由
func RegisterSpikeRoutesWithConfig(
	r *gin.RouterGroup,
	spikeHandler *api.SpikeHandler,
	config *SpikeRoutesConfig,
) {
	RegisterSpikeRoutes(
		r,
		spikeHandler,
		config.JWTMiddleware,
		config.AdminMiddleware,
		config.SpikeLimiter,
		config.APILimiter,
	)
}

// SpikeRoutesConfig 秒杀路由配置
type SpikeRoutesConfig struct {
	JWTMiddleware   gin.HandlerFunc // JWT认证中间件
	AdminMiddleware gin.HandlerFunc // 管理员权限中间件
	SpikeLimiter    limiter.Limiter // 秒杀专用限流器
	APILimiter      limiter.Limiter // API通用限流器
}
