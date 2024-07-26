package modbusorm

// ConnType connection type
type ConnType uint8

const (
	ConnTypeTCP ConnType = 1
	ConnTypeRTU ConnType = 2
)

// PointDataType  point data type
type PointDataType uint8

const (
	PointDataTypeU16 PointDataType = iota
	PointDataTypeS16
	PointDataTypeU32
	PointDataTypeS32
)

// RegisterType register type
type RegisterType uint8

const (
	RegisterTypeDefault         RegisterType = iota // default = RegisterTypeHoldingRegister
	RegisterTypeCoil                                // Coil (0x01-Read single or multiple, 0x05-Write single, 0x15-Write multiple)
	RegisterTypeDiscreteInput                       // Discrete Input (0x02-Read single or multiple)
	RegisterTypeInputRegister                       // Input Register (0x04-Read single or multiple)
	RegisterTypeHoldingRegister                     // Holding Register (0x03-Read single or multiple, 0x06-Write single, 0x16-Write multiple
)

var RegisterTypeList = []RegisterType{
	RegisterTypeDefault,
	RegisterTypeCoil,
	RegisterTypeDiscreteInput,
	RegisterTypeInputRegister,
	RegisterTypeHoldingRegister,
}

// OrderType order type
type OrderType uint8

const (
	OrderTypeDefault      OrderType = iota // default = BigEndian
	OrderTypeBigEndian                     // high byte first
	OrderTypeLittleEndian                  // low byte first
)
