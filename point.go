package modbusorm

// OriginByte the origin byte
type OriginByte []byte

const OriginByteName = "OriginByte"

// Point point table
type Point map[string]PointDetails

type PointDetails struct {
	// address, like 900, represents read/write from 900th register
	Addr uint16
	// quantity, like 2, represents 2 registers
	Quantity uint16
	// coefficient, like 0.1, represents the value should be multiplied by 0.1
	Coefficient float64
	// offset, like -10, represents the value should be subtracted by 10
	Offset float64
	// data type, like U16, represents the data type is unsigned 16 bits
	DataType PointDataType
	// order type, like LittleEndian, represents the byte order is low byte first
	OrderType OrderType
	// register type, default is HoldRegister
	RegisterType RegisterType
}

// getCoefficient get coefficient, if coefficient not set, return 1
func (p *PointDetails) getCoefficient() float64 {
	if p.Coefficient == 0 {
		return 1
	}
	return p.Coefficient
}

// getQuantity get quantity, if quantity not set, return 1
func (p *PointDetails) getQuantity() uint16 {
	if p.Quantity == 0 {
		return 1
	}
	return p.Quantity
}
