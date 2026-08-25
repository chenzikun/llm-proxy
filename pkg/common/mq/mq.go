package mq

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zicorn/llm-proxy/pkg/common/config"
)

var Client *amqp.Connection
var Channel *amqp.Channel
var connNotify chan *amqp.Error
var channelNotify chan *amqp.Error
var isConnected int32  // 使用int32代替bool，用于原子操作
var reconnecting int32 // 使用int32代替bool，用于原子操作

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

// 连接RabbitMQ并设置通知通道
func connect() error {
	if config.GetMQUrl() == "" {
		return nil
	}
	var err error

	// 设置连接配置，添加心跳检测
	mqConfig := amqp.Config{
		Heartbeat: 10 * time.Second, // 设置10秒的心跳间隔
	}

	Client, err = amqp.DialConfig(config.GetMQUrl(), mqConfig)
	if err != nil {
		return err
	}

	Channel, err = Client.Channel()
	if err != nil {
		return err
	}

	_, err = Channel.QueueDeclare(
		config.MQQueueName, // name
		true,               // durable
		false,              // delete when unused
		false,              // exclusive
		false,              // no-wait
		nil,                // arguments
	)
	if err != nil {
		return err
	}

	// 设置连接关闭通知
	connNotify = make(chan *amqp.Error)
	Client.NotifyClose(connNotify)

	// 设置通道关闭通知
	channelNotify = make(chan *amqp.Error)
	Channel.NotifyClose(channelNotify)

	atomic.StoreInt32(&isConnected, 1) // 设置连接状态为已连接
	return nil
}

// 重连RabbitMQ
func reconnect() {
	// 使用原子操作检查和设置reconnecting
	if atomic.LoadInt32(&reconnecting) == 1 {
		return
	}

	if !atomic.CompareAndSwapInt32(&reconnecting, 0, 1) {
		return // 如果有其他goroutine已经设置了reconnecting，则返回
	}

	const maxRetries = 5
	var retries int

	for retries < maxRetries {
		log.Printf("尝试重新连接RabbitMQ，第%d次尝试...", retries+1)

		// 关闭旧连接
		if Client != nil {
			Client.Close()
		}

		// 尝试重新连接
		err := connect()
		if err == nil {
			log.Println("成功重新连接到RabbitMQ")
			atomic.StoreInt32(&reconnecting, 0) // 重置reconnecting状态
			return
		}

		retries++
		log.Printf("重连失败: %s", err)

		// 如果还有重试次数，等待一段时间后再次尝试
		if retries < maxRetries {
			time.Sleep(time.Second * 2)
		}
	}

	// 达到最大重试次数后仍然失败
	atomic.StoreInt32(&reconnecting, 0) // 重置reconnecting状态
	log.Panicf("无法重新连接到RabbitMQ，已尝试%d次", maxRetries)
}

// 监控连接状态
func monitorConnection() {
	if config.GetMQUrl() == "" {
		return
	}
	for {
		select {
		case err := <-connNotify:
			if err != nil {
				atomic.StoreInt32(&isConnected, 0) // 设置连接状态为断开
				log.Printf("RabbitMQ连接断开: %s", err)
				reconnect()
			}
		case err := <-channelNotify:
			if err != nil {
				atomic.StoreInt32(&isConnected, 0) // 设置连接状态为断开
				log.Printf("RabbitMQ通道关闭: %s", err)
				reconnect()
			}
		}
	}
}

// 初始化MQ连接
func init() {
	err := connect()
	failOnError(err, "Failed to connect to RabbitMQ")

	// 启动连接监控
	go monitorConnection()
}

// 等待MQ连接恢复
func waitForConnection(timeout int) bool {
	// 检查连接状态
	if atomic.LoadInt32(&isConnected) == 1 {
		return true
	}

	log.Println("MQ连接已断开，等待重连...")
	// 等待重连完成
	for i := 0; i < timeout; i++ {
		time.Sleep(time.Second)
		if atomic.LoadInt32(&isConnected) == 1 {
			return true
		}
	}
	return false
}

// 发送消息到MQ
func publishMessage(ctx context.Context, data []byte) error {
	return Channel.PublishWithContext(ctx,
		"",                 // exchange
		config.MQQueueName, // routing key
		false,              // mandatory
		false,              // immediate
		amqp.Publishing{
			ContentType:  "text/plain",
			Body:         data,
			DeliveryMode: 2,
		})
}

// 推送消息到MQ队列
func Push(ctx context.Context, data []byte) {
	if config.GetMQUrl() == "" {
		return
	}
	// 尝试发送消息，最多重试3次
	const maxRetries = 3
	var err error

	for i := 0; i < maxRetries; i++ {
		// 检查连接状态并在断开时等待重连
		waitForConnection(5) // 最多等待5秒

		err = publishMessage(ctx, data)
		if err == nil {
			// 发送成功
			return
		}

		log.Printf("发送消息失败(第%d次尝试): %s", i+1, err)

		// 标记连接为断开状态
		atomic.StoreInt32(&isConnected, 0)
	}

	// 所有重试都失败
	if err != nil {
		log.Printf("消息发送失败，已尝试%d次: %s", maxRetries, err)
		// 这里不使用failOnError，避免因为单个消息发送失败而导致整个程序崩溃
	}
}
