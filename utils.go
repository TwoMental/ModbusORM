package modbusorm

import (
	"encoding/binary"
	"fmt"
	"reflect"
)

// parseDataToFloat64 transform data to float64
func parseDataToFloat64(data []byte, dataType PointDataType, order ...OrderType) (float64, error) {
	var dataFloat64Before float64
	binaryOrder := OrderTypeDefault
	if len(order) > 0 {
		binaryOrder = order[0]
	}
	switch dataType {
	case PointDataTypeU16:
		dataFloat64Before = float64(binary.BigEndian.Uint16(data))
	case PointDataTypeS16:
		dataFloat64Before = float64(int16(binary.BigEndian.Uint16(data)))
	case PointDataTypeU32:
		dataFloat64Before = float64(binaryUint32(data, binaryOrder))
	case PointDataTypeS32:
		dataFloat64Before = float64(int32(binaryUint32(data, binaryOrder)))
	default:
		return 0, fmt.Errorf("unsupported data type: %d", dataType)
	}
	return dataFloat64Before, nil
}

// binaryUint32 reverse the byte order and convert to uint32
func binaryUint32(data []byte, order OrderType) uint32 {
	if order == OrderTypeLittleEndian {
		data[0], data[2] = data[2], data[0]
		data[1], data[3] = data[3], data[1]
	}
	return binary.BigEndian.Uint32(data)
}

// getPointTag get morm tag
func getPointTag(field reflect.StructField) (bool, string) {
	name := field.Tag.Get("morm")
	if name == "-" || name == "" {
		return false, ""
	}
	return true, name
}

// byte2String convert byte to string
func byte2String(data []byte, order OrderType) string {
	if len(data)%2 != 0 {
		data = append(data, 0x00)
	}

	if order == OrderTypeLittleEndian {
		for i := 0; i < len(data); i += 2 {
			data[i], data[i+1] = data[i+1], data[i]
		}
	}

	for i, b := range data {
		if b == 0x00 {
			return string(data[:i])
		}
	}
	return string(data)
}

// string2Byte convert string to byte
func string2Byte(data string, order OrderType) []byte {
	result := []byte(data)

	if len(result)%2 != 0 {
		result = append(result, 0x00)
	}

	if order == OrderTypeLittleEndian {
		for i := 0; i < len(data); i += 2 {
			result[i], result[i+1] = result[i+1], result[i]
		}
	}

	return result
}

type block struct {
	start  uint16
	end    uint16
	values []byte
}

type blocks map[uint16]*block
