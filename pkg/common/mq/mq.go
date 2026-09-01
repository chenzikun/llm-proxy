package mq

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zicorn/llm-proxy/pkg/common/config"
)

const (
	heartbeat           = 10 * time.Second
	minReconnectDelay   = 5 * time.Second
	maxReconnectDelay   = time.Minute
	publishRetries      = 3
	waitConnectedTimout = 5 * time.Second
)

var (
	mu          sync.RWMutex
	client      *amqp.Connection
	channel     *amqp.Channel
	isConnected int32
	connecting  int32
)

// Enabled 未配置 RabbitMQ 地址时整个上报链路关闭。
func Enabled() bool {
	return config.GetMQUrl() != ""
}

func init() {
	if !Enabled() {
		return
	}
	go keepConnected()
}

func connect() error {
	conn, err := amqp.DialConfig(config.GetMQUrl(), amqp.Config{Heartbeat: heartbeat})
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return err
	}

	_, err = ch.QueueDeclare(
		config.MQQueueName, // name
		true,               // durable
		false,              // delete when unused
		false,              // exclusive
		false,              // no-wait
		nil,                // arguments
	)
	if err != nil {
		conn.Close()
		return err
	}

	connNotify := make(chan *amqp.Error, 1)
	conn.NotifyClose(connNotify)
	channelNotify := make(chan *amqp.Error, 1)
	ch.NotifyClose(channelNotify)

	mu.Lock()
	client, channel = conn, ch
	mu.Unlock()
	atomic.StoreInt32(&isConnected, 1)

	go watch(connNotify, channelNotify)
	return nil
}

func watch(connNotify, channelNotify chan *amqp.Error) {
	select {
	case err := <-connNotify:
		log.Printf("RabbitMQ 连接断开: %v", err)
	case err := <-channelNotify:
		log.Printf("RabbitMQ 通道关闭: %v", err)
	}

	atomic.StoreInt32(&isConnected, 0)
	mu.Lock()
	if client != nil {
		client.Close()
	}
	client, channel = nil, nil
	mu.Unlock()

	go keepConnected()
}

// keepConnected 后台重连，同一时刻只允许一个重连循环。
func keepConnected() {
	if !atomic.CompareAndSwapInt32(&connecting, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&connecting, 0)

	delay := minReconnectDelay
	for atomic.LoadInt32(&isConnected) == 0 {
		err := connect()
		if err == nil {
			log.Println("已连接到 RabbitMQ")
			return
		}
		log.Printf("连接 RabbitMQ 失败，%s 后重试: %v", delay, err)

		time.Sleep(delay)
		if delay < maxReconnectDelay {
			delay *= 2
		}
	}
}

func currentChannel() *amqp.Channel {
	if atomic.LoadInt32(&isConnected) == 0 {
		return nil
	}
	mu.RLock()
	defer mu.RUnlock()
	return channel
}

// waitForConnection 等待后台重连完成，超时返回 false。
func waitForConnection(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&isConnected) == 1 {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return atomic.LoadInt32(&isConnected) == 1
}

// Push 推送消息到 MQ 队列，失败只记录日志，不影响主流程。
func Push(ctx context.Context, data []byte) {
	if !Enabled() {
		return
	}

	var err error
	for i := 0; i < publishRetries; i++ {
		ch := currentChannel()
		if ch == nil {
			go keepConnected()
			waitForConnection(waitConnectedTimout)
			if ch = currentChannel(); ch == nil {
				err = errors.New("RabbitMQ 未连接")
				continue
			}
		}

		err = ch.PublishWithContext(ctx,
			"",                 // exchange
			config.MQQueueName, // routing key
			false,              // mandatory
			false,              // immediate
			amqp.Publishing{
				ContentType:  "text/plain",
				Body:         data,
				DeliveryMode: 2,
			})
		if err == nil {
			return
		}
		atomic.StoreInt32(&isConnected, 0)
	}

	log.Printf("消息推送失败，已尝试%d次: %v", publishRetries, err)
}
