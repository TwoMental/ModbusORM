package modbusorm

import (
	"time"

	"github.com/goburrow/modbus"
)

type Client interface {
	modbus.Client
	Connect() error
	Close() error
	IsAlive() bool
	CreateTime() time.Time
}

type ConnPool interface {
	Get() (Client, error)
	Put(conn Client) error
	Close() error
}
