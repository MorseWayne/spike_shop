# main.go 中中间件链的代码解析

  在第37-45行，代码实现了一个中间件链的构建过程，这是Go Web应用程序中常见的一种设计模式，用于在请求到达实际处理函数之前和响应返回客户端之后执行一系列通用操作。

  ## 代码分析

  ```go:/home/wayne/source/spike_shop/cmd/spike-server/main.go
  // Build middleware chain: request ID -> recovery -> timeout -> CORS -> access log
  handler := mw.RequestID(mux)
  handler = mw.Recovery(lg)(handler)
  handler = mw.Timeout(cfg.App.RequestTimeout)(handler)
  handler = mw.CORS(mw.CORSConfig{
      AllowedOrigins: cfg.CORS.AllowedOrigins,
      AllowedMethods: cfg.CORS.AllowedMethods,
      AllowedHeaders: cfg.CORS.AllowedHeaders,
  })(handler)
  handler = mw.AccessLog(lg)(handler)
  ```

  ## 中间件链的工作原理

  这段代码使用了**函数式中间件模式**，通过函数闭包和组合构建了一个处理链。每个中间件接收一个`http.Handler`，返回一个增强后的新`http.Handler`。请求会按照从内到外的顺序通过这些中间件，而响应则按照从外到内的顺序返回。

  ## 各中间件的作用

  1. **RequestID中间件**
     - 为每个请求生成并注入一个唯一标识符
     - 便于在分布式系统中追踪请求流
     - 是链路追踪的基础组件

  2. **Recovery中间件**
     - 捕获请求处理过程中的panic异常
     - 防止单个请求的异常导致整个服务器崩溃
     - 将异常信息记录到日志系统，便于问题排查

  3. **Timeout中间件**
     - 限制请求处理的最大时间
     - 防止长时间运行的请求占用过多资源
     - 超时后会返回适当的错误响应

  4. **CORS中间件**
     - 处理跨域资源共享（Cross-Origin Resource Sharing）
     - 根据配置设置允许的源、HTTP方法和头部
     - 确保前端应用可以安全地从不同域访问API

  5. **AccessLog中间件**
     - 记录每个请求的访问日志
     - 包括请求方法、路径、状态码、响应时间等信息
     - 为监控和审计提供数据支持

  ## 中间件执行顺序

  当请求进入系统时，会按照以下顺序通过中间件：

  1. 首先到达最外层的`AccessLog`中间件
  2. 然后是`CORS`中间件
  3. 接着是`Timeout`中间件
  4. 然后是`Recovery`中间件
  5. 最后是最内层的`RequestID`中间件
  6. 最终到达实际的请求处理器（在本例中是`/healthz`端点）

  响应返回时，顺序则完全相反：从内到外依次通过各个中间件。

  ## 为什么使用中间件链

  这种设计模式有以下几个优点：

  1. **关注点分离**：每个中间件只负责一项特定功能
  2. **代码复用**：可以在不同的路由或应用中重用相同的中间件
  3. **可组合性**：可以灵活地添加、删除或重新排序中间件
  4. **统一处理**：对所有请求应用一致的处理策略，如日志记录、错误处理等

  ## 后续应用

  这个中间件链最终被传递给HTTP服务器：

  ```go
  addr := fmt.Sprintf("%d", cfg.App.Port)
  lg.Sugar().Infow("server starting", "addr", addr)
  srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
  if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
      lg.Sugar().Fatalw("server error", "err", err)
  }
  ```

  这样，所有到达服务器的请求都会经过完整的中间件链处理。
          
