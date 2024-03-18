package modbusorm

// OriginByte the origin byte
type OriginByte []byte

// PointDataType point data type
type PointDataType uint8

const (
	PointDataTypeU16 PointDataType = iota
	PointDataTypeS16
	PointDataTypeU32
	PointDataTypeS32
)

// OrderType order type
type OrderType uint8

const (
	OrderTypeDefault      OrderType = iota // default
	OrderTypeBigEndian                     // high byte first
	OrderTypeLittleEndian                  // low byte first
)

// Point point table
type Point map[string]PointDetails

type PointDetails struct {
	// address, like 900, represents read/write from 900th register
	Addr uint16
	// quantiry, like 2, represents 2 registers
	Quantity uint16
	// coefficient, like 0.1, represents the value should be multiplied by 0.1
	Coefficient float64
	// offset, like -10, represents the value should be subtracted by 10
	Offset float64
	// data type, like U16, represents the data type is unsigned 16 bits
	DataType PointDataType
	// order type, like BigEndian, represents the byte order is high byte first
	OrderType OrderType
}

// GetCoefficient get coefficient, if coefficient not set, return 1
func (p *PointDetails) GetCoefficient() float64 {
	if p.Coefficient == 0 {
		return 1
	}
	return p.Coefficient
}
