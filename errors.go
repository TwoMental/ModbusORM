package modbusorm

import "errors"

var (
	ErrPoolClosed = errors.New("modbus pool is closed")
	ErrFactoryNil = errors.New("factory cannot be nil")
)
