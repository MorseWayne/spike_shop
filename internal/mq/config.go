// Package mq 提供RabbitMQ消息队列配置和连接管理
package mq

import (
	"crypto/tls"
	"fmt"
	"time"
)

// Config RabbitMQ配置
type Config struct {
	// 连接配置
	Host     string `mapstructure:"host" json:"host"`
	Port     int    `mapstructure:"port" json:"port"`
	Username string `mapstructure:"username" json:"username"`
	Password string `mapstructure:"password" json:"password"`
	VHost    string `mapstructure:"vhost" json:"vhost"`

	// TLS配置
	UseTLS                bool   `mapstructure:"use_tls" json:"use_tls"`
	TLSCertFile           string `mapstructure:"tls_cert_file" json:"tls_cert_file"`
	TLSKeyFile            string `mapstructure:"tls_key_file" json:"tls_key_file"`
	TLSCACertFile         string `mapstructure:"tls_ca_cert_file" json:"tls_ca_cert_file"`
	TLSServerName         string `mapstructure:"tls_server_name" json:"tls_server_name"`
	TLSInsecureSkipVerify bool   `mapstructure:"tls_insecure_skip_verify" json:"tls_insecure_skip_verify"`

	// 连接池配置
	MaxConnections    int           `mapstructure:"max_connections" json:"max_connections"`
	MaxChannels       int           `mapstructure:"max_channels" json:"max_channels"`
	ConnectionTimeout time.Duration `mapstructure:"connection_timeout" json:"connection_timeout"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval" json:"heartbeat_interval"`

	// 重连配置
	EnableReconnect      bool          `mapstructure:"enable_reconnect" json:"enable_reconnect"`
	ReconnectInterval    time.Duration `mapstructure:"reconnect_interval" json:"reconnect_interval"`
	MaxReconnectAttempts int           `mapstructure:"max_reconnect_attempts" json:"max_reconnect_attempts"`

	// 生产者配置
	Producer *ProducerConfig `mapstructure:"producer" json:"producer"`

	// 消费者配置
	Consumer *ConsumerConfig `mapstructure:"consumer" json:"consumer"`

	// 交换机和队列配置
	Exchanges []*ExchangeConfig `mapstructure:"exchanges" json:"exchanges"`
	Queues    []*QueueConfig    `mapstructure:"queues" json:"queues"`
}

// ProducerConfig 生产者配置
type ProducerConfig struct {
	// 发布确认
	EnableConfirm  bool          `mapstructure:"enable_confirm" json:"enable_confirm"`
	ConfirmTimeout time.Duration `mapstructure:"confirm_timeout" json:"confirm_timeout"`

	// 重试配置
	EnableRetry      bool          `mapstructure:"enable_retry" json:"enable_retry"`
	MaxRetryAttempts int           `mapstructure:"max_retry_attempts" json:"max_retry_attempts"`
	RetryInterval    time.Duration `mapstructure:"retry_interval" json:"retry_interval"`

	// 批量发送
	EnableBatch  bool          `mapstructure:"enable_batch" json:"enable_batch"`
	BatchSize    int           `mapstructure:"batch_size" json:"batch_size"`
	BatchTimeout time.Duration `mapstructure:"batch_timeout" json:"batch_timeout"`

	// 发送超时
	PublishTimeout time.Duration `mapstructure:"publish_timeout" json:"publish_timeout"`
}

// ConsumerConfig 消费者配置
type ConsumerConfig struct {
	// 消费配置
	PrefetchCount int  `mapstructure:"prefetch_count" json:"prefetch_count"`
	PrefetchSize  int  `mapstructure:"prefetch_size" json:"prefetch_size"`
	AutoAck       bool `mapstructure:"auto_ack" json:"auto_ack"`
	Exclusive     bool `mapstructure:"exclusive" json:"exclusive"`
	NoLocal       bool `mapstructure:"no_local" json:"no_local"`
	NoWait        bool `mapstructure:"no_wait" json:"no_wait"`

	// 重试配置
	EnableRetry      bool          `mapstructure:"enable_retry" json:"enable_retry"`
	MaxRetryAttempts int           `mapstructure:"max_retry_attempts" json:"max_retry_attempts"`
	RetryInterval    time.Duration `mapstructure:"retry_interval" json:"retry_interval"`

	// 死信队列
	EnableDLX     bool   `mapstructure:"enable_dlx" json:"enable_dlx"`
	DLXExchange   string `mapstructure:"dlx_exchange" json:"dlx_exchange"`
	DLXRoutingKey string `mapstructure:"dlx_routing_key" json:"dlx_routing_key"`

	// 消费超时
	ConsumeTimeout time.Duration `mapstructure:"consume_timeout" json:"consume_timeout"`

	// 并发消费
	ConcurrentConsumers int `mapstructure:"concurrent_consumers" json:"concurrent_consumers"`
}

// ExchangeConfig 交换机配置
type ExchangeConfig struct {
	Name       string                 `mapstructure:"name" json:"name"`
	Type       string                 `mapstructure:"type" json:"type"` // direct, topic, fanout, headers
	Durable    bool                   `mapstructure:"durable" json:"durable"`
	AutoDelete bool                   `mapstructure:"auto_delete" json:"auto_delete"`
	Internal   bool                   `mapstructure:"internal" json:"internal"`
	NoWait     bool                   `mapstructure:"no_wait" json:"no_wait"`
	Args       map[string]interface{} `mapstructure:"args" json:"args"`
}

// QueueConfig 队列配置
type QueueConfig struct {
	Name       string                 `mapstructure:"name" json:"name"`
	Durable    bool                   `mapstructure:"durable" json:"durable"`
	AutoDelete bool                   `mapstructure:"auto_delete" json:"auto_delete"`
	Exclusive  bool                   `mapstructure:"exclusive" json:"exclusive"`
	NoWait     bool                   `mapstructure:"no_wait" json:"no_wait"`
	Args       map[string]interface{} `mapstructure:"args" json:"args"`

	// 绑定配置
	Bindings []*BindingConfig `mapstructure:"bindings" json:"bindings"`
}

// BindingConfig 绑定配置
type BindingConfig struct {
	Exchange   string                 `mapstructure:"exchange" json:"exchange"`
	RoutingKey string                 `mapstructure:"routing_key" json:"routing_key"`
	NoWait     bool                   `mapstructure:"no_wait" json:"no_wait"`
	Args       map[string]interface{} `mapstructure:"args" json:"args"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Host:     "localhost",
		Port:     5672,
		Username: "guest",
		Password: "guest",
		VHost:    "/",

		UseTLS: false,

		MaxConnections:    10,
		MaxChannels:       100,
		ConnectionTimeout: 30 * time.Second,
		HeartbeatInterval: 10 * time.Second,

		EnableReconnect:      true,
		ReconnectInterval:    5 * time.Second,
		MaxReconnectAttempts: 10,

		Producer: &ProducerConfig{
			EnableConfirm:    true,
			ConfirmTimeout:   5 * time.Second,
			EnableRetry:      true,
			MaxRetryAttempts: 3,
			RetryInterval:    1 * time.Second,
			EnableBatch:      false,
			BatchSize:        100,
			BatchTimeout:     1 * time.Second,
			PublishTimeout:   10 * time.Second,
		},

		Consumer: &ConsumerConfig{
			PrefetchCount:       10,
			PrefetchSize:        0,
			AutoAck:             false,
			Exclusive:           false,
			NoLocal:             false,
			NoWait:              false,
			EnableRetry:         true,
			MaxRetryAttempts:    3,
			RetryInterval:       1 * time.Second,
			EnableDLX:           true,
			DLXExchange:         "dlx",
			DLXRoutingKey:       "failed",
			ConsumeTimeout:      30 * time.Second,
			ConcurrentConsumers: 1,
		},
	}
}

// GetConnectionURL 获取连接URL
func (c *Config) GetConnectionURL() string {
	scheme := "amqp"
	if c.UseTLS {
		scheme = "amqps"
	}

	return fmt.Sprintf("%s://%s:%s@%s:%d%s",
		scheme, c.Username, c.Password, c.Host, c.Port, c.VHost)
}

// GetTLSConfig 获取TLS配置
func (c *Config) GetTLSConfig() (*tls.Config, error) {
	if !c.UseTLS {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		ServerName:         c.TLSServerName,
		InsecureSkipVerify: c.TLSInsecureSkipVerify,
	}

	// 如果提供了证书文件，加载客户端证书
	if c.TLSCertFile != "" && c.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.TLSCertFile, c.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host is required")
	}

	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	if c.Username == "" {
		return fmt.Errorf("username is required")
	}

	if c.MaxConnections <= 0 {
		return fmt.Errorf("max_connections must be greater than 0")
	}

	if c.MaxChannels <= 0 {
		return fmt.Errorf("max_channels must be greater than 0")
	}

	if c.ConnectionTimeout <= 0 {
		return fmt.Errorf("connection_timeout must be greater than 0")
	}

	if c.HeartbeatInterval <= 0 {
		return fmt.Errorf("heartbeat_interval must be greater than 0")
	}

	if c.Producer != nil {
		if err := c.Producer.Validate(); err != nil {
			return fmt.Errorf("producer config validation failed: %w", err)
		}
	}

	if c.Consumer != nil {
		if err := c.Consumer.Validate(); err != nil {
			return fmt.Errorf("consumer config validation failed: %w", err)
		}
	}

	return nil
}

// Validate 验证生产者配置
func (c *ProducerConfig) Validate() error {
	if c.ConfirmTimeout <= 0 {
		return fmt.Errorf("confirm_timeout must be greater than 0")
	}

	if c.MaxRetryAttempts < 0 {
		return fmt.Errorf("max_retry_attempts must be >= 0")
	}

	if c.RetryInterval <= 0 {
		return fmt.Errorf("retry_interval must be greater than 0")
	}

	if c.BatchSize <= 0 {
		return fmt.Errorf("batch_size must be greater than 0")
	}

	if c.BatchTimeout <= 0 {
		return fmt.Errorf("batch_timeout must be greater than 0")
	}

	if c.PublishTimeout <= 0 {
		return fmt.Errorf("publish_timeout must be greater than 0")
	}

	return nil
}

// Validate 验证消费者配置
func (c *ConsumerConfig) Validate() error {
	if c.PrefetchCount < 0 {
		return fmt.Errorf("prefetch_count must be >= 0")
	}

	if c.PrefetchSize < 0 {
		return fmt.Errorf("prefetch_size must be >= 0")
	}

	if c.MaxRetryAttempts < 0 {
		return fmt.Errorf("max_retry_attempts must be >= 0")
	}

	if c.RetryInterval <= 0 {
		return fmt.Errorf("retry_interval must be greater than 0")
	}

	if c.ConsumeTimeout <= 0 {
		return fmt.Errorf("consume_timeout must be greater than 0")
	}

	if c.ConcurrentConsumers <= 0 {
		return fmt.Errorf("concurrent_consumers must be greater than 0")
	}

	return nil
}
