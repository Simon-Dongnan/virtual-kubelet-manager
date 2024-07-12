package mqtt

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"os"
	"time"
)

const (
	// Qos0 means message only published once
	Qos0 = iota

	// Qos1 means message must be consumed
	Qos1

	// Qos2 means message must be consumed only once
	Qos2
)

type Client struct {
	client mqtt.Client
}

type ClientConfig struct {
	Broker                string
	Port                  int
	ClientID              string
	Username              string
	Password              string
	CAPath                string
	ClientCrtPath         string
	ClientKeyPath         string
	CleanSession          bool
	KeepAlive             time.Duration
	DefaultMessageHandler mqtt.MessageHandler
	OnConnectHandler      mqtt.OnConnectHandler
	ConnectionLostHandler mqtt.ConnectionLostHandler
}

var defaultMessageHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	log.G(context.Background()).Infof("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
}

var defaultOnConnectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	log.G(context.Background()).Info("Connected")
}

var defaultConnectionLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	log.G(context.Background()).Warnf("Connect lost: %v\n", err)
}

// newTlsConfig create a tls config using client config
func newTlsConfig(cfg *ClientConfig) (*tls.Config, error) {
	config := tls.Config{
		InsecureSkipVerify: true,
	}

	certpool := x509.NewCertPool()
	ca, err := os.ReadFile(cfg.CAPath)
	if err != nil {
		return nil, err
	}
	certpool.AppendCertsFromPEM(ca)
	config.RootCAs = certpool
	if cfg.ClientCrtPath != "" {
		// Import client certificate/key pair
		clientKeyPair, err := tls.LoadX509KeyPair(cfg.ClientCrtPath, cfg.ClientKeyPath)
		if err != nil {
			return nil, err
		}
		config.Certificates = []tls.Certificate{clientKeyPair}
		config.ClientAuth = tls.NoClientCert
	}

	return &config, nil
}

// NewMqttClient create a new client using client config
func NewMqttClient(cfg *ClientConfig) (*Client, error) {
	opts := mqtt.NewClientOptions()
	broker := ""
	opts.SetClientID(cfg.ClientID)
	if cfg.CAPath != "" {
		// tls configured
		tlsConfig, err := newTlsConfig(cfg)
		if err != nil {
			return nil, err
		}
		opts.SetTLSConfig(tlsConfig)
		broker = fmt.Sprintf("ssl://%s:%d", cfg.Broker, cfg.Port)
	} else {
		broker = fmt.Sprintf("tcp://%s:%d", cfg.Broker, cfg.Port)
		opts.SetUsername(cfg.Username)
		opts.SetPassword(cfg.Password)
	}

	opts.AddBroker(broker)

	if cfg.DefaultMessageHandler == nil {
		cfg.DefaultMessageHandler = defaultMessageHandler
	}

	if cfg.OnConnectHandler == nil {
		cfg.OnConnectHandler = defaultOnConnectHandler
	}

	if cfg.ConnectionLostHandler == nil {
		cfg.ConnectionLostHandler = defaultConnectionLostHandler
	}

	if cfg.KeepAlive == 0 {
		cfg.KeepAlive = time.Minute
	}

	opts.SetDefaultPublishHandler(cfg.DefaultMessageHandler)
	opts.SetAutoReconnect(true)
	opts.SetKeepAlive(cfg.KeepAlive)
	opts.SetCleanSession(cfg.CleanSession)
	opts.SetOnConnectHandler(cfg.OnConnectHandler)
	opts.SetConnectionLostHandler(cfg.ConnectionLostHandler)
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}
	return &Client{
		client: client,
	}, nil
}

// PubWithTimeout publish a message to target topic with timeout config, return false if send failed or timeout
func (c *Client) PubWithTimeout(topic string, qos byte, msg interface{}, timeout time.Duration) bool {
	return c.client.Publish(topic, qos, true, msg).WaitTimeout(timeout)
}

// Pub publish a message to target topic, waiting for publish operation finish, return false if send failed
func (c *Client) Pub(topic string, qos byte, msg interface{}) bool {
	return c.client.Publish(topic, qos, true, msg).Wait()
}

// SubWithTimeout subscribe a topic with callback, return false if subscription's creation fail or creation timeout
func (c *Client) SubWithTimeout(topic string, qos byte, timeout time.Duration, callBack mqtt.MessageHandler) bool {
	return c.client.Subscribe(topic, qos, callBack).WaitTimeout(timeout)
}

// Sub subscribe a topic with callback, return false if subscription's creation fail
func (c *Client) Sub(topic string, qos byte, callBack mqtt.MessageHandler) bool {
	return c.client.Subscribe(topic, qos, callBack).Wait()
}

// UnSub unsubscribe a topic
func (c *Client) UnSub(topic string) bool {
	return c.client.Unsubscribe(topic).Wait()
}
