package modbusorm

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/goburrow/modbus"
)

type ModbusTCPPool struct {
	mutex       sync.Mutex
	connections chan Client
	factory     func() (Client, error)
	closed      bool
	config      ModbusTCPPoolConfig
}

type ModbusTCPPoolConfig struct {
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
}

type ModbusTCPClient struct {
	Client     modbus.Client
	Handler    *modbus.TCPClientHandler
	createTime time.Time
}

func (c *ModbusTCPClient) Connect() error {
	return c.Handler.Connect()
}

func (c *ModbusTCPClient) Close() error {
	return c.Handler.Close()
}

func (c *ModbusTCPClient) IsAlive() bool {
	_, err := c.Client.ReadHoldingRegisters(1, 1)
	if err != nil {
		if strings.Contains(err.Error(), "EOF") {
			return false
		} else if strings.Contains(err.Error(), "connection refused") {
			return false
		} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return false
		}
	}
	return true
}

func (c *ModbusTCPClient) CreateTime() time.Time {
	return c.createTime
}

func (c *ModbusTCPClient) ReadCoils(address, quantity uint16) (results []byte, err error) {
	return c.Client.ReadCoils(address, quantity)
}

func (c *ModbusTCPClient) ReadDiscreteInputs(address, quantity uint16) (results []byte, err error) {
	return c.Client.ReadDiscreteInputs(address, quantity)
}

func (c *ModbusTCPClient) WriteSingleCoil(address, value uint16) (results []byte, err error) {
	return c.Client.WriteSingleCoil(address, value)
}

func (c *ModbusTCPClient) WriteMultipleCoils(address, quantity uint16, value []byte) (results []byte, err error) {
	return c.Client.WriteMultipleCoils(address, quantity, value)
}

func (c *ModbusTCPClient) ReadInputRegisters(address, quantity uint16) (results []byte, err error) {
	return c.Client.ReadInputRegisters(address, quantity)
}

func (c *ModbusTCPClient) ReadHoldingRegisters(address, quantity uint16) (results []byte, err error) {
	return c.Client.ReadHoldingRegisters(address, quantity)
}

func (c *ModbusTCPClient) WriteSingleRegister(address, value uint16) (results []byte, err error) {
	return c.Client.WriteSingleRegister(address, value)
}

func (c *ModbusTCPClient) WriteMultipleRegisters(address, quantity uint16, value []byte) (results []byte, err error) {
	return c.Client.WriteMultipleRegisters(address, quantity, value)
}

func (c *ModbusTCPClient) ReadWriteMultipleRegisters(readAddress, readQuantity, writeAddress, writeQuantity uint16, value []byte) (results []byte, err error) {
	return c.Client.ReadWriteMultipleRegisters(readAddress, readQuantity, writeAddress, writeQuantity, value)
}

func (c *ModbusTCPClient) MaskWriteRegister(address, andMask, orMask uint16) (results []byte, err error) {
	return c.Client.MaskWriteRegister(address, andMask, orMask)
}

func (c *ModbusTCPClient) ReadFIFOQueue(address uint16) (results []byte, err error) {
	return c.Client.ReadFIFOQueue(address)
}

func NewModbusTCPPool(config ModbusTCPPoolConfig, factory func() (Client, error)) (ConnPool, error) {
	if factory == nil {
		return nil, ErrFactoryNil
	}
	if config.MaxOpenConns <= 0 {
		config.MaxOpenConns = 5
	}

	pool := &ModbusTCPPool{
		factory:     factory,
		connections: make(chan Client, config.MaxOpenConns),
		config:      config,
	}

	for i := 0; i < config.MaxOpenConns; i++ {
		conn, err := factory()
		if err != nil {
			return nil, err
		}
		pool.connections <- conn
	}

	return pool, nil
}

// Get get a connection from pool
func (p *ModbusTCPPool) Get() (Client, error) {
	if p.closed {
		return nil, ErrPoolClosed
	}

	select {
	case conn := <-p.connections:
		return conn, nil
	default:
		// if no avaliable connction, new one
		return p.factory()
	}
}

// Put put the connection to the pool
func (p *ModbusTCPPool) Put(conn Client) error {
	if p.closed {
		return conn.Close()
	}

	if time.Since(conn.CreateTime()) > p.config.ConnMaxLifetime {
		// if connection is expired, close it
		return conn.Close()
	}
	if !conn.IsAlive() {
		// if connection is not alive, close it
		return conn.Close()
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	select {
	case p.connections <- conn:
		return nil
	default:
		// if the pool is full, close the connection
		return conn.Close()
	}
}

// Close close the pool
func (p *ModbusTCPPool) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.closed {
		return ErrPoolClosed
	}

	p.closed = true

	for {
		select {
		case conn, ok := <-p.connections:
			if !ok {
				return nil
			}
			conn.Close()
		default:
			close(p.connections)
			return nil
		}
	}
}
