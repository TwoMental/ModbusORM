package modbusorm

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"sort"

	"github.com/pkg/errors"
)

// SetValue: set value to modbus from values.
// Support types:
// | Type                 | PointDetails  Suggestion                             | Comment                   |
// |----------------------|------------------------------------------------------|---------------------------|
// | int8, int16          | Quantity=1, DataType=PointDataTypeS16                |                           |
// | int, int32           | Quantity=2, DataType=PointDataTypeS32                |                           |
// | uint8, uint16        | Quantity=1, DataType=PointDataTypeS16                |                           |
// | uint, uint32         | Quantity=2, DataType=PointDataTypeS32                |                           |
// | []int8, []int16      | Quantity=len, DataType=PointDataTypeS16              |                           |
// | []int, []int32       | Quantity=len*2, DataType=PointDataTypeS32            |                           |
// | []uint8, []uint16    | Quantity=len, DataType=PointDataTypeS16              |                           |
// | []uint, []uint32     | Quantity=len*2, DataType=PointDataTypeS32            |                           |
// | float32, float64     | Quantity=1, DataType=PointDataTypeS16/U16            |for fields with coefficient|
// | float32, float64     | Quantity=2, DataType=PointDataTypeS32/U32            |for fields with coefficient|
// | []float32, []float64 | Quantity=len, DataType=PointDataTypeS16/U16          |for fields with coefficient|
// | []float32, []float64 | Quantity=len*2, DataType=PointDataTypeS32/U32        |for fields with coefficient|
// | string               | Quantity=(len+1)/2, DataType=PointDataTypeU16        |                           |
// | OriginByte           | Quantity=(len+1)/2, DataType=PointDataTypeU16        |                           |
func (m *Modbus) SetValue(ctx context.Context, point string, data any) error {
	// check if the point exists
	fieldDetail, ok := m.points[point]
	if !ok {
		return fmt.Errorf("point for %s not found", point)
	}
	dataByte, err := m.valueToBytes(data, fieldDetail, fieldDetail.getQuantity())
	if err != nil {
		return errors.Wrap(err, "valueToBytes failed")
	}

	// connection
	conn, err := m.connPool.Get()
	if err != nil {
		return fmt.Errorf("conn slave failed: %w", err)
	}
	defer m.Put(ctx, conn)

	// write data
	return m.writeData(conn, fieldDetail.Addr, fieldDetail.getQuantity(), fieldDetail.RegisterType, dataByte)
}

func (m *Modbus) valueToBytes(data any, fieldDetail PointDetails, quantity uint16) ([]byte, error) {
	// check if data is OriginByte
	if reflect.TypeOf(data).Name() == OriginByteName {
		dataByte := reflect.ValueOf(data).Bytes()
		if len(dataByte) != int(quantity)*2 {
			return nil, fmt.Errorf("value length not match, want %d, got %d", quantity*2, len(dataByte))
		}
		return dataByte, nil
	}

	var buffer bytes.Buffer

	// check if data is a pointer, if it is, dereference it
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// calculate the value and write to buffer
	var err error
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value := float64(v.Int())/fieldDetail.getCoefficient() - fieldDetail.Offset
		err = binary.Write(&buffer, binary.BigEndian, int64(value))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value := float64(v.Uint())/fieldDetail.getCoefficient() - fieldDetail.Offset
		err = binary.Write(&buffer, binary.BigEndian, uint64(value))
	case reflect.Float32, reflect.Float64:
		valueFloat := (v.Float() / fieldDetail.getCoefficient()) - fieldDetail.Offset
		if fieldDetail.DataType == PointDataTypeU32 || fieldDetail.DataType == PointDataTypeS32 {
			err = binary.Write(&buffer, binary.BigEndian, int32(math.Round(valueFloat))) // math.Round避免精度损失
		} else {
			err = binary.Write(&buffer, binary.BigEndian, int16(math.Round(valueFloat)))
		}
	case reflect.String:
		b := string2Byte(v.String(), fieldDetail.OrderType)
		return adjustByteSliceLength(b, quantity, false), nil
	case reflect.Slice, reflect.Array:
		var elemQuantity uint16 = 1
		if fieldDetail.DataType == PointDataTypeU32 || fieldDetail.DataType == PointDataTypeS32 {
			elemQuantity = 2
		}
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i)
			bytes, err := m.valueToBytes(elem.Interface(), fieldDetail, elemQuantity)
			if err != nil {
				return nil, err
			}
			binary.Write(&buffer, binary.BigEndian, bytes)
		}
		return adjustByteSliceLength(buffer.Bytes(), quantity, false), nil
	default:
		return nil, fmt.Errorf("unsupported type: %s", v.Type())
	}

	if err != nil {
		return nil, err
	}

	return adjustByteSliceLength(buffer.Bytes(), quantity, true), nil
}

// adjustByteSliceLength adjusts the length of the byte slice to match quantity
/*
	data: the byte slice to adjust
	quantity: the ideal length of the byte slice
	head: true means need to fill/trim in front, false means need to fill/trim in the back
*/
func adjustByteSliceLength(data []byte, quantity uint16, head bool) []byte {
	quantity *= 2
	if len(data) < int(quantity) {
		// need to fill
		padding := make([]byte, int(quantity)-len(data))
		if head {
			data = append(padding, data...)
		} else {
			data = append(data, padding...)
		}
	} else if len(data) > int(quantity) {
		// need to trim
		if head {
			data = data[len(data)-int(quantity):]
		} else {
			data = data[:quantity]
		}
	}
	return data
}

// SetValues: Set values to modbus from v.
/*
	Fields need to be set should have tag "morm"
	Notice:
		1. If the field type is not a pointer and is empty, the default value will be set
		2. If the field type is a pointer and is empty, the value will not be set
*/
func (m *Modbus) SetValues(ctx context.Context, v any) error {
	// collect the addresses and values to write
	addrValue := initaddrValueTodo()
	if e := m.gatherAddrValue(ctx, v, addrValue); e != nil {
		return errors.Wrap(e, "gatherAddrValue failed")
	}

	for _, k := range RegisterTypeList {
		if len(addrValue[k]) == 0 {
			continue
		}
		// covert to block
		var blockData blocks
		if m.withBlock {
			blockData = m.addrValueToBlocks(addrValue[k])
		}
		// write values
		if e := m.writeValues(ctx, blockData, k); e != nil {
			return errors.Wrap(e, "writeValues failed")
		}
	}
	return nil
}

// addrValueTodo is a map of address need to write
/*
	{
		RegisterTypeCoil: {1: {}, 2: {}},
		RegisterTypeHoldingRegister: {1: {}, 10: {}},
	}
*/
type addrValueTodo map[RegisterType][]*block

func initaddrValueTodo() addrValueTodo {
	m := make(addrValueTodo)
	for _, k := range RegisterTypeList {
		m[k] = make([]*block, 0)
	}
	return m
}

func (m *Modbus) gatherAddrValue(ctx context.Context, v any, addrValues addrValueTodo) error {
	// the actual value and type
	var valueElem reflect.Value = reflect.ValueOf(v)
	var typeElem reflect.Type
	if valueElem.Kind() == reflect.Ptr || valueElem.Kind() == reflect.Interface {
		valueElem = valueElem.Elem()
		typeElem = reflect.TypeOf(v).Elem()
	} else {
		typeElem = reflect.TypeOf(v)
	}

	// Check if the value is a struct
	if typeElem.Kind() != reflect.Struct {
		return fmt.Errorf("v must be struct or pointer of struct, not %s", typeElem.Kind())
	}

	fieldNum := valueElem.NumField()
	for i := 0; i < fieldNum; i++ {
		value := valueElem.Field(i)
		if value.Kind() == reflect.Struct {
			// dive
			if !value.CanAddr() {
				continue
			}
			addr := value.Addr()
			if !addr.IsValid() || !addr.CanInterface() {
				continue
			}
			if e := m.gatherAddrValue(ctx, addr.Interface(), addrValues); e != nil {
				return fmt.Errorf("gatherAddrValue for %s failed: %w", typeElem.Field(i).Name, e)
			}
			continue
		}

		// jump if the field is not a pointer
		if value.Kind() == reflect.Pointer {
			if value.IsZero() {
				continue
			} else {
				value = value.Elem()
			}
		}
		exist, fieldName := getPointTag(typeElem.Field(i))
		if !exist {
			continue
		}
		fieldDetail, ok := m.points[fieldName]
		if !ok {
			continue
		}
		var base uint16 = 1
		if fieldDetail.DataType == PointDataTypeU32 || fieldDetail.DataType == PointDataTypeS32 {
			base = 2
		}
		if fieldDetail.getQuantity() == base {
			var valueFloat float64
			if value.CanInt() {
				valueFloat = float64(value.Int())
			} else if value.CanUint() {
				valueFloat = float64(value.Uint())
			} else if value.CanFloat() {
				valueFloat = value.Float()
			} else {
				continue
			}
			buf := new(bytes.Buffer)
			var err error
			if base == 1 {
				err = binary.Write(buf, binary.BigEndian, uint16((valueFloat-fieldDetail.Offset)/fieldDetail.getCoefficient()))
			} else {
				err = binary.Write(buf, binary.BigEndian, uint32((valueFloat-fieldDetail.Offset)/fieldDetail.getCoefficient()))
			}
			if err != nil {
				return fmt.Errorf("binary.Write failed: %w", err)
			}

			addrValues[fieldDetail.RegisterType] = append(addrValues[fieldDetail.RegisterType], &block{
				start:  fieldDetail.Addr,
				end:    fieldDetail.Addr + fieldDetail.getQuantity() - 1,
				values: buf.Bytes(),
			})
		} else {
			switch value.Kind() {
			case reflect.String:
				byteLen := int(fieldDetail.getQuantity() * 2)
				stringByte := []byte(value.String())
				if len(stringByte) > byteLen {
					stringByte = stringByte[:byteLen]
				} else if len(stringByte) < byteLen {
					stringByte = append(stringByte, make([]byte, byteLen-len(stringByte))...)
				}
				addrValues[fieldDetail.RegisterType] = append(addrValues[fieldDetail.RegisterType], &block{
					start:  fieldDetail.Addr,
					end:    fieldDetail.Addr + fieldDetail.getQuantity() - 1,
					values: stringByte,
				})
			case reflect.Array, reflect.Slice:
				if value.Type().Name() == OriginByteName {
					addrValues[fieldDetail.RegisterType] = append(addrValues[fieldDetail.RegisterType], &block{
						start:  fieldDetail.Addr,
						end:    fieldDetail.Addr + fieldDetail.getQuantity() - 1,
						values: value.Bytes(),
					})
				} else {
					buf := new(bytes.Buffer)
					for j := 0; j < value.Len(); j++ {
						var valueFloat float64
						if value.Index(j).CanInt() {
							valueFloat = float64(value.Index(j).Int())
						} else if value.Index(j).CanUint() {
							valueFloat = float64(value.Index(j).Uint())
						} else if value.Index(j).CanFloat() {
							valueFloat = value.Index(j).Float()
						} else {
							continue
						}
						err := binary.Write(buf, binary.BigEndian, uint16((valueFloat-fieldDetail.Offset)/fieldDetail.getCoefficient()))
						if err != nil {
							return fmt.Errorf("binary.Write failed: %w", err)
						}
					}
					addrValues[fieldDetail.RegisterType] = append(addrValues[fieldDetail.RegisterType], &block{
						start:  fieldDetail.Addr,
						end:    fieldDetail.Addr + fieldDetail.getQuantity() - 1,
						values: buf.Bytes(),
					})
				}
			}
		}
	}
	return nil
}

func (m *Modbus) addrValueToBlocks(rawData []*block) blocks {
	// Convert the map to a slice of addresses
	addrs := make([]*block, 0, len(rawData))
	for _, v := range rawData {
		if v.start == v.end {
			addrs = append(addrs, v)
		} else {
			for i := 0; i <= int(v.end-v.start); i++ {
				addrs = append(addrs, &block{start: v.start + uint16(i), end: v.start + uint16(i), values: v.values[i*2 : (i+1)*2]})
			}
		}
	}

	// Sort the addresses
	sort.Slice(addrs, func(i, j int) bool { return addrs[i].start < addrs[j].start })

	// Group continuous addresses into blocks, merge blocks with small gaps
	bs := make(blocks)
	start := addrs[0].start
	end := addrs[0].start
	values := addrs[0].values
	for i := 1; i < len(addrs); i++ {
		if addrs[i].start == end+1 && end-start+1 <= m.maxBlockSize {
			end = addrs[i].end
			values = append(values, addrs[i].values...)
		} else {
			bs[start] = &block{start: start, end: end, values: values}
			start = addrs[i].start
			end = addrs[i].end
			values = addrs[i].values
		}
	}
	bs[start] = &block{start: start, end: end, values: values}

	return bs
}

func (m *Modbus) writeValues(_ context.Context, data blocks, registerType RegisterType) error {
	// conn
	conn, err := m.connPool.Get()
	if err != nil {
		return fmt.Errorf("conn slave failed: %w", err)
	}
	defer m.Put(context.Background(), conn)

	// set
	for _, b := range data {
		if e := m.writeData(conn, b.start, b.end-b.start+1, registerType, b.values); e != nil {
			return e
		}
	}
	return nil
}

// writeData allow to write data that exceeds maxQuantity
func (m *Modbus) writeData(conn Client, addr uint16, quantity uint16, registerType RegisterType, data []byte) error {
	if quantity <= m.maxQuantity {
		return m.writeDataByType(conn, addr, quantity, registerType, data)
	}
	for quantity > 0 {
		curAddr := addr
		// calculate the quantity of this request
		currentQuantity := min(quantity, m.maxQuantity)
		// send request
		err := m.writeDataByType(conn, curAddr, currentQuantity, registerType, data[(curAddr-addr)*2:(curAddr-addr)*2+currentQuantity*2])
		if err != nil {
			return err
		}
		// update the start address and remaining quantity
		curAddr += currentQuantity
		quantity -= currentQuantity
	}
	return nil
}

// writeDataByType writes data according to the register type (not allowed to exceed maxQuantity)
func (m *Modbus) writeDataByType(conn Client, addr uint16, quanity uint16, registerType RegisterType, data []byte) error {
	var err error
	switch registerType {
	case RegisterTypeCoil:
		if quanity == 1 {
			_, err = conn.WriteSingleCoil(addr, binary.BigEndian.Uint16(data))
		} else {
			_, err = conn.WriteMultipleCoils(addr, quanity, data)
		}
	case RegisterTypeHoldingRegister, RegisterTypeDefault:
		if quanity == 1 {
			_, err = conn.WriteSingleRegister(addr, binary.BigEndian.Uint16(data))
		} else {
			_, err = conn.WriteMultipleRegisters(addr, quanity, data)
		}
	default:
		err = fmt.Errorf("unsupported register type for write: %d", registerType)
	}
	return err
}
