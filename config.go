package modbusorm

import "time"

// modbusTCP Connection config of TCP
type modbusTCP struct {
	Host            string
	Port            int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
}

// modbusRTU Connection config of RTU
type modbusRTU struct {
	ComAddr  string
	BaudRate int
	DataBits int
	Parity   string // (N, E, O)
	StopBits int
}

type ModbusOption func(*Modbus)

// WithHost Set the host of the modbus TCP
func WithHost(host string) ModbusOption {
	return func(d *Modbus) {
		d.Host = host
	}
}

// WithPort Set the port of the modbus TCP
func WithPort(port int) ModbusOption {
	return func(d *Modbus) {
		d.Port = port
	}
}

// WithMaxOpenConns Set the max open connections of the modbus TCP
func WithMaxOpenConns(maxOpenConns int) ModbusOption {
	return func(d *Modbus) {
		d.MaxOpenConns = maxOpenConns
	}
}

// WithConnMaxLifetime Set the max connection lifetime of the modbus TCP
func WithConnMaxLifetime(connMaxLifetime time.Duration) ModbusOption {
	return func(d *Modbus) {
		d.ConnMaxLifetime = connMaxLifetime
	}
}

// WithComAddr Set the com address of the modbus RTU
func WithComAddr(comAddr string) ModbusOption {
	return func(d *Modbus) {
		d.ComAddr = comAddr
	}
}

// WithBaudRate Set the baud rate of the modbus RTU
func WithBaudRate(baudRate int) ModbusOption {
	return func(d *Modbus) {
		d.BaudRate = baudRate
	}
}

// WithDataBits Set the data bits of the modbus RTU
func WithDataBits(dataBits int) ModbusOption {
	return func(d *Modbus) {
		d.DataBits = dataBits
	}
}

// WithParity Set the parity of the modbus RTU
func WithParity(parity string) ModbusOption {
	return func(d *Modbus) {
		d.Parity = parity
	}
}

// WithStopBits Set the stop bits of the modbus RTU
func WithStopBits(stopBits int) ModbusOption {
	return func(d *Modbus) {
		d.StopBits = stopBits
	}
}

// WithTimeout Set the timeout of the modbus
func WithTimeout(timeout time.Duration) ModbusOption {
	return func(d *Modbus) {
		d.timeout = timeout
	}
}

// WithSlaveID Set the slave id of the modbus
func WithSlaveID(slaveID uint8) ModbusOption {
	return func(d *Modbus) {
		d.slaveID = slaveID
	}
}

// WithMaxQuantity Set the max quantity of the modbus
func WithMaxQuantity(maxQuantity uint16) ModbusOption {
	return func(d *Modbus) {
		d.maxQuantity = maxQuantity
	}
}
