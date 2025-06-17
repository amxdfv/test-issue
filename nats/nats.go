package natsLog

import (
	"time"

	"github.com/nats-io/nats.go"
)

// NatsConfig конфигурация для nats
type NatsConfig struct {
	ConnString string `yaml:"connection"`
}

// LogMessage сообщение в лог
type LogMessage struct {
	ID          int       `json:"id"`
	ProjectID   int       `json:"project_id"`
	Name        string    `json:"name,omitempty"`
	Description *string   `json:"description,omitempty"`
	Priority    int       `json:"priority,omitempty"`
	Removed     bool      `json:"removed,omitempty"`
	EventTime   time.Time `json:"event_time"`
}

// GetNats подключаемся к nats
func GetNats(cfg NatsConfig) (*nats.Conn, error) {
	return nats.Connect(cfg.ConnString)
}

// SendLog отправляем в лог
func SendLog(nc *nats.Conn, payload []byte) error {
	return nc.Publish("test_issue", payload)
}
