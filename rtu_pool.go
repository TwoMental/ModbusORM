package modbusorm

import (
	"time"

	"github.com/goburrow/modbus"
)

type ModbusRTUPool struct {
	client Client
}

func NewModbusRTUPool(client Client) (ConnPool, error) {
	return &ModbusRTUPool{
		client: client,
	}, nil
}

func (p *ModbusRTUPool) Get() (Client, error) {
	return p.client, nil
}

func (p *ModbusRTUPool) Put(conn Client) error {
	return nil
}

func (p *ModbusRTUPool) Close() error {
	return p.client.Close()
}

type ModbusRTUClient struct {
	Client     modbus.Client
	Handler    *modbus.RTUClientHandler
	createTime time.Time
}

func (c *ModbusRTUClient) Connect() error {
	return c.Handler.Connect()
}

func (c *ModbusRTUClient) Close() error {
	return c.Handler.Close()
}

func (c *ModbusRTUClient) IsAlive() bool {
	return true
}

func (c *ModbusRTUClient) CreateTime() time.Time {
	return c.createTime
}

func (c *ModbusRTUClient) ReadCoils(address, quantity uint16) (results []byte, err error) {
	return c.Client.ReadCoils(address, quantity)
}

func (c *ModbusRTUClient) ReadDiscreteInputs(address, quantity uint16) (results []byte, err error) {
	return c.Client.ReadDiscreteInputs(address, quantity)
}

func (c *ModbusRTUClient) WriteSingleCoil(address, value uint16) (results []byte, err error) {
	return c.Client.WriteSingleCoil(address, value)
}

func (c *ModbusRTUClient) WriteMultipleCoils(address, quantity uint16, value []byte) (results []byte, err error) {
	return c.Client.WriteMultipleCoils(address, quantity, value)
}

func (c *ModbusRTUClient) ReadInputRegisters(address, quantity uint16) (results []byte, err error) {
	return c.Client.ReadInputRegisters(address, quantity)
}

func (c *ModbusRTUClient) ReadHoldingRegisters(address, quantity uint16) (results []byte, err error) {
	return c.Client.ReadHoldingRegisters(address, quantity)
}

func (c *ModbusRTUClient) WriteSingleRegister(address, value uint16) (results []byte, err error) {
	return c.Client.WriteSingleRegister(address, value)
}

func (c *ModbusRTUClient) WriteMultipleRegisters(address, quantity uint16, value []byte) (results []byte, err error) {
	return c.Client.WriteMultipleRegisters(address, quantity, value)
}

func (c *ModbusRTUClient) ReadWriteMultipleRegisters(readAddress, readQuantity, writeAddress, writeQuantity uint16, value []byte) (results []byte, err error) {
	return c.Client.ReadWriteMultipleRegisters(readAddress, readQuantity, writeAddress, writeQuantity, value)
}

func (c *ModbusRTUClient) MaskWriteRegister(address, andMask, orMask uint16) (results []byte, err error) {
	return c.Client.MaskWriteRegister(address, andMask, orMask)
}

func (c *ModbusRTUClient) ReadFIFOQueue(address uint16) (results []byte, err error) {
	return c.Client.ReadFIFOQueue(address)
}
