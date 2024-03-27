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

// WithBlock Set the read by block or not
/*
	If the block is true, ModbusORM will read by block,
	otherwise, ModbusORM will read by single point.
	This mode is set to reduce the number of requests.
	But for those devices that do not support block read, please set it to false.

	For example, if you have a struct like this:
	type Data struct {
		Voltage     *float64             `morm:"voltage"` 		// address 100
		Temperature float64              `morm:"temperature"` 	// address 101
		Star        []float64            `morm:"star"` 			// address 102-104
	}

	It's much more efficient to read by block (one request versus three requests).

	With `WithMaxBlockSize` and `WithMaxGapInBlock` you can control the size of block and the request number.
*/
func WithBlock(block bool) ModbusOption {
	return func(d *Modbus) {
		d.withBlock = block
	}
}

// WithMaxBlockSize Set the max block size of the modbus
func WithMaxBlockSize(maxBlockSize uint16) ModbusOption {
	return func(d *Modbus) {
		d.maxBlockSize = maxBlockSize
	}
}

// WithMaxGapInBlock Set the max gap in block of the modbus
func WithMaxGapInBlock(maxGapInBlock uint16) ModbusOption {
	return func(d *Modbus) {
		d.maxGapInBlock = maxGapInBlock
	}
}
