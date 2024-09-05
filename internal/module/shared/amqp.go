package shared

import (
	"time"

	"github.com/knadh/koanf/v2"
	"github.com/rs/zerolog"
	amqplib "github.com/streadway/amqp"
) //导入mq包

type Amqp struct {
	Conn             *amqplib.Connection
	Channel          *amqplib.Channel
	Exchange         string // 交换机
	ExchangeType     string // 交换机类型
	url              string //MQ链接字符串
	logger           zerolog.Logger
	keepliveInterval time.Duration
	retryCount       int
}

// 创建结构体实例
func NewRabbitMQ(cfg *koanf.Koanf, logger zerolog.Logger) *Amqp {
	amqp := Amqp{
		Exchange:         cfg.String("amqp.exchange"),
		ExchangeType:     cfg.String("amqp.exchange-type"),
		url:              cfg.String("amqp.url"),
		logger:           logger,
		retryCount:       cfg.Int("amqp.retry-count"),
		keepliveInterval: cfg.Duration("amqp.keeplive-interval"),
	}

	return &amqp
}

func (a *Amqp) keeplive() {
	var err error
	for {
		for i := 1; i <= a.retryCount; i++ {
			// 建立链接
			if a.Conn == nil || a.Conn.IsClosed() {
				a.Conn, err = amqplib.Dial(a.url)
				if err != nil {
					if i == a.retryCount {
						// 超出重试次数，直接退出服务
						a.Close()
						a.logger.Panic().Msgf("Failed to connect to Amqp: %v. Retrying in %v...\n", err, i)
						return
					} else {
						// 失败，等待重试
						a.logger.Warn().Msgf("Failed to connect to Amqp: %v. Retrying in %v...\n", err, i)
					}
				}
			}

			// Channel已建立
			if a.Conn != nil && a.Channel != nil {
				// 连接与Channel正常，退出循环
				break
			}

			// 建立Channel
			a.Channel, err = a.Conn.Channel()
			if err != nil {
				// 失败，等待重试
				a.logger.Warn().Msgf("Failed to create Channel to Amqp: %v. Retrying in %v...\n", err, i)
			} else {
				// 创建交换机
				err = a.Channel.ExchangeDeclare(
					a.Exchange,
					a.ExchangeType,
					true,
					false,
					false,
					false,
					nil,
				)
				if err != nil {
					a.logger.Warn().Msgf("Failed to create Exchange to Amqp: %v. Retrying in %v...\n", err, i)
				}
				// 成功，退出循环
				break
			}
		}

		time.Sleep(a.keepliveInterval)
	}
}

func (a *Amqp) Connect() {
	var err error
	a.Conn, err = amqplib.Dial(a.url)
	if err != nil {
		// 失败，等待重试
		a.logger.Error().Msgf("%s:%s\n", "amqp链接失败", err)
		return
	}

	//创建Channel
	a.Channel, err = a.Conn.Channel()
	if err != nil {
		a.logger.Error().Msgf("%s:%s\n", "创建Channel失败", err)
		return
	}

	err = a.Channel.ExchangeDeclare(
		a.Exchange,
		a.ExchangeType,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		a.logger.Error().Msgf("%s:%s\n", "创建交换机失败", err)
		return
	}

	go a.keeplive()
}

// 释放资源,建议NewRabbitMQ获取实例后 配合defer使用
func (a *Amqp) Close() {
	a.Conn.Close()
	a.Channel.Close()
}
