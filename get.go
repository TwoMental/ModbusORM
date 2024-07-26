package modbusorm

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"sort"
	"time"

	"github.com/pkg/errors"
)

// GetValue Get value from modbus and write to v.
/*
	point: the point name
	v: the value to write (must be a pointer)
*/
func (m *Modbus) GetValue(ctx context.Context, point string, v any) error {
	// check if the point exists
	fieldDetail, ok := m.points[point]
	if !ok {
		return fmt.Errorf("point for %s not found", point)
	}

	// connection
	conn, err := m.connPool.Get()
	if err != nil {
		return fmt.Errorf("conn slave failed: %w", err)
	}
	defer m.Put(ctx, conn)

	// read data
	data, err := m.readDataByType(conn, fieldDetail.Addr, fieldDetail.getQuantity(), fieldDetail.RegisterType)
	if err != nil {
		return fmt.Errorf("ReadHoldingRegisters for %s failed, %w", point, err)
	}

	dataFloat64Before, err := parseDataToFloat64(data, fieldDetail.DataType, fieldDetail.OrderType)
	if err != nil {
		return err
	}
	dataFloat64 := cal(dataFloat64Before, fieldDetail.getCoefficient()) + fieldDetail.Offset

	var parseErr error
	switch v := v.(type) {
	case *float32:
		*v = float32(dataFloat64)
	case *float64:
		*v = dataFloat64
	case *int:
		*v = int(dataFloat64)
	case *int8:
		*v = int8(dataFloat64)
	case *int16:
		*v = int16(dataFloat64)
	case *int32:
		*v = int32(dataFloat64)
	case *int64:
		*v = int64(dataFloat64)
	case *uint:
		*v = uint(dataFloat64)
	case *uint8:
		*v = uint8(dataFloat64)
	case *uint16:
		*v = uint16(dataFloat64)
	case *uint32:
		*v = uint32(dataFloat64)
	case *string:
		*v = byte2String(data, fieldDetail.OrderType)
	case *[]int, *[]int8, *[]int16, *[]int32, *[]int64, *[]uint, *[]uint8, *[]uint16, *[]uint32, *[]uint64:
		elemType := reflect.TypeOf(v).Elem().Elem()
		elemSize := elemType.Size()
		if len(data) < int(elemSize) {
			parseErr = fmt.Errorf("insufficient data for slice type: %v", elemType)
		} else {
			buf := bytes.NewBuffer(data)
			sliceValue := reflect.ValueOf(v).Elem()
			for buf.Len() >= int(elemSize) {
				elemValue := reflect.New(elemType).Elem()
				binary.Read(buf, binary.BigEndian, elemValue.Addr().Interface())
				sliceValue.Set(reflect.Append(sliceValue, elemValue))
			}
		}
	case *OriginByte:
		*v = data
	default:
		// TODO: other data type
		parseErr = fmt.Errorf("unsupported data type: %v", reflect.TypeOf(v))
	}
	return parseErr
}

// GetValues Get values from modbus and write to v.
/*
	Fields need to be set should have tag "morm"
	v should be a struct pointer
*/
func (m *Modbus) GetValues(ctx context.Context, v any, filter ...string) error {
	if m.withBlock {
		return m.getValuesBlock(ctx, v, filter...)
	}
	return m.getValuesSingle(ctx, v, filter...)
}

func (m *Modbus) getValuesSingle(ctx context.Context, v any, filter ...string) error {
	conn, err := m.connPool.Get()
	if err != nil {
		return fmt.Errorf("conn slave failed: %w", err)
	}
	defer m.Put(ctx, conn)
	return m.getValues(ctx, v, conn, filter...)
}

func (m *Modbus) getValues(ctx context.Context, v any, conn Client, filter ...string) error {
	// validate v
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("not support for %s", val.Kind().String())
	}

	valueElem := val.Elem()
	typeElem := reflect.TypeOf(v).Elem()

	if valueElem.Kind() != reflect.Struct {
		return fmt.Errorf("not support for %s pointer", valueElem.Kind().String())
	}

	// filter
	var needFilter bool
	var filterMap map[string]bool
	if len(filter) != 0 {
		needFilter = true
		filterMap = parseFilter(filter)
	}

	for i := 0; i < valueElem.NumField(); i++ {
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
			if e := m.getValues(ctx, addr.Interface(), conn, filter...); e != nil {
				return e
			}
			continue
		}
		exist, fieldName := getPointTag(typeElem.Field(i))
		if !exist {
			continue
		}
		if needFilter && !filterMap[fieldName] {
			continue
		}
		fieldDetail, ok := m.points[fieldName]
		if !ok {
			continue
		}
		data, err := m.readData(conn, fieldDetail.Addr, fieldDetail.getQuantity(), fieldDetail.RegisterType)
		if err != nil {
			return fmt.Errorf("read data for %s failed, %w", fieldName, err)
		}

		dataFloat64Before, err := parseDataToFloat64(data, fieldDetail.DataType, fieldDetail.OrderType)
		if err != nil {
			return err
		}
		dataFloat64 := cal(dataFloat64Before, fieldDetail.getCoefficient()) + fieldDetail.Offset

		switch value.Type().Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			value.SetInt(int64(dataFloat64))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			value.SetUint(uint64(dataFloat64))
		case reflect.Float32, reflect.Float64:
			value.SetFloat(dataFloat64)
		case reflect.String:
			value.SetString(byte2String(data, fieldDetail.OrderType))
		case reflect.Pointer:
			ptrType := value.Type().Elem()
			newValue := reflect.New(ptrType)
			switch ptrType.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				newValue.Elem().SetInt(int64(dataFloat64))
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				newValue.Elem().SetUint(uint64(dataFloat64))
			case reflect.Float32, reflect.Float64:
				newValue.Elem().SetFloat(dataFloat64)
			case reflect.String:
				newValue.SetString(byte2String(data, fieldDetail.OrderType))
			default:
				return fmt.Errorf("parse for %s pointer not supported", value.Type().Kind())
			}
			value.Set(newValue)
		case reflect.Slice, reflect.Array:
			newSlice := reflect.MakeSlice(value.Type(), 0, 0)

			if value.Type().Name() == OriginByteName {
				for _, b := range data {
					newSlice = reflect.Append(newSlice, reflect.ValueOf(b))
				}
			} else {
				size := 2
				if fieldDetail.DataType == PointDataTypeU32 || fieldDetail.DataType == PointDataTypeS32 {
					size = 4
				}
				for i := 0; i+size < len(data); i += size {
					dataFloat64, err := parseDataToFloat64(data[i:i+size], fieldDetail.DataType, fieldDetail.OrderType)
					if err != nil {
						return err
					}
					dataFloat64 = cal(dataFloat64, fieldDetail.getCoefficient()) + fieldDetail.Offset
					newSlice = reflect.Append(newSlice, reflect.ValueOf(dataFloat64))
				}
			}

			value.Set(newSlice)
		default:
			return fmt.Errorf("parse for %s not supported", value.Type().Kind())
		}
		time.Sleep(1 * time.Millisecond)
	}
	return nil
}

// addrTodo is a map of address need to read
//
//	{
//		RegisterTypeCoil: {1: {}, 2: {}},
//		RegisterTypeHoldingRegister: {1: {}, 10: {}},
//	}
type addrTodo map[RegisterType]map[uint16]struct{}

func initAddrTodo() addrTodo {
	m := make(addrTodo)
	for _, k := range RegisterTypeList {
		m[k] = make(map[uint16]struct{})
	}
	return m
}

func (m *Modbus) getValuesBlock(ctx context.Context, v any, filter ...string) error {
	// Get the address blocks
	addrMap := initAddrTodo()
	filterMap := parseFilter(filter)
	err := m.collectAddresses(ctx, v, addrMap, filterMap)
	if err != nil {
		return errors.Wrap(err, "collectAddresses failed")
	}
	if len(addrMap) == 0 {
		return fmt.Errorf("no address found")
	}

	// each register type
	for _, k := range RegisterTypeList {
		if len(addrMap[k]) == 0 {
			continue
		}
		// Convert the map to block list
		blocks := m.addrMapToBlocks(addrMap[k])
		// Read the blocks
		if e := m.readBlocks(ctx, blocks, k); e != nil {
			return errors.Wrap(e, "readBlocks failed")
		}
		// Set the values
		if e := m.setAddressValues(ctx, v, blocks, k, filterMap); e != nil {
			return errors.Wrap(e, "setAddressValues failed")
		}
	}
	return nil
}

func (m *Modbus) addrMapToBlocks(addrMap map[uint16]struct{}) blocks {
	// Convert the map to a slice of addresses
	addrs := make([]uint16, 0, len(addrMap))
	for addr := range addrMap {
		addrs = append(addrs, addr)
	}

	// Sort the addresses
	sort.Slice(addrs, func(i, j int) bool { return addrs[i] < addrs[j] })

	// Group continuous addresses into blocks, merge blocks with small gaps
	bs := make(blocks)
	start := addrs[0]
	end := addrs[0]
	for i := 1; i < len(addrs); i++ {
		if addrs[i] == end+1 && addrs[i]-start+1 <= m.maxBlockSize {
			end = addrs[i]
		} else if addrs[i]-end <= m.maxGapInBlock && addrs[i]-start+1 <= m.maxBlockSize {
			end = addrs[i]
		} else {
			bs[start] = &block{start: start, end: end}
			start = addrs[i]
			end = addrs[i]
		}
	}
	bs[start] = &block{start: start, end: end}

	return bs
}

func (m *Modbus) collectAddresses(ctx context.Context, v any, addrMap addrTodo, filterMap map[string]bool) error {
	// validate v
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("not support for %s", val.Kind().String())
	}

	valueElem := val.Elem()
	typeElem := reflect.TypeOf(v).Elem()

	if valueElem.Kind() != reflect.Struct {
		return fmt.Errorf("not support for %s pointer", valueElem.Kind().String())
	}

	// filter
	needFilter := len(filterMap) != 0

	for i := 0; i < valueElem.NumField(); i++ {
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
			if e := m.collectAddresses(ctx, addr.Interface(), addrMap, filterMap); e != nil {
				return e
			}
			continue
		}
		exist, fieldName := getPointTag(typeElem.Field(i))
		if !exist {
			continue
		}
		if needFilter && !filterMap[fieldName] {
			continue
		}
		fieldDetail, ok := m.points[fieldName]
		if !ok {
			continue
		}
		for j := uint16(0); j < fieldDetail.getQuantity(); j++ {
			addrMap[fieldDetail.RegisterType][fieldDetail.Addr+j] = struct{}{}
		}

	}
	return nil
}

func (m *Modbus) readBlocks(ctx context.Context, blocks blocks, registerType RegisterType) error {
	// Get a connection
	conn, err := m.connPool.Get()
	if err != nil {
		return fmt.Errorf("conn slave failed: %w", err)
	}
	defer m.Put(ctx, conn)
	// Read each block
	for _, block := range blocks {
		data, err := m.readData(conn, uint16(block.start), uint16(block.end-block.start+1), registerType)
		if err != nil {
			return err
		}
		if len(data) != int(block.end-block.start+1)*2 {
			return fmt.Errorf("read block failed, want %d, got %d", (block.end-block.start+1)*2, len(data))
		}
		block.values = data
		// Sleep for a while to avoid too frequent requests
		time.Sleep(1 * time.Millisecond)
	}
	return nil
}

func (m *Modbus) setAddressValues(ctx context.Context, v any, values blocks, registerType RegisterType, filterMap map[string]bool) error {
	needFilter := len(filterMap) != 0
	valueElem := reflect.ValueOf(v).Elem()
	typeElem := reflect.TypeOf(v).Elem()

	for i := 0; i < valueElem.NumField(); i++ {
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
			if e := m.setAddressValues(ctx, addr.Interface(), values, registerType, filterMap); e != nil {
				return e
			}
			continue
		}
		exist, fieldName := getPointTag(typeElem.Field(i))
		if !exist {
			continue
		}
		if needFilter && !filterMap[fieldName] {
			continue
		}
		fieldDetail, ok := m.points[fieldName]
		if !ok || fieldDetail.RegisterType != registerType {
			continue
		}
		// find data
		data := m.getFieldData([]byte{}, values, fieldDetail.Addr, fieldDetail.getQuantity())

		// set value
		dataFloat64Before, err := parseDataToFloat64(data, fieldDetail.DataType, fieldDetail.OrderType)
		if err != nil {
			return err
		}
		dataFloat64 := cal(dataFloat64Before, fieldDetail.getCoefficient()) + fieldDetail.Offset
		switch value.Type().Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			value.SetInt(int64(dataFloat64))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			value.SetUint(uint64(dataFloat64))
		case reflect.Float32, reflect.Float64:
			value.SetFloat(dataFloat64)
		case reflect.String:
			value.SetString(byte2String(data, fieldDetail.OrderType))
		case reflect.Pointer:
			// get the type pointed to by the pointer
			ptrType := value.Type().Elem()
			newValue := reflect.New(ptrType)
			// create a new instance based on the type
			switch ptrType.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				newValue.Elem().SetInt(int64(dataFloat64))
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				newValue.Elem().SetUint(uint64(dataFloat64))
			case reflect.Float32, reflect.Float64:
				newValue.Elem().SetFloat(dataFloat64)
			case reflect.String:
				newValue.SetString(byte2String(data, fieldDetail.OrderType))
			default:
				return fmt.Errorf("parse for %s pointer not supported", value.Type().Kind())
			}
			// set the new pointer to the field
			value.Set(newValue)
		case reflect.Slice, reflect.Array:
			elemType := value.Type().Elem()
			newSlice := reflect.MakeSlice(reflect.SliceOf(elemType), 0, 0)

			if value.Type().Name() == OriginByteName {
				for _, b := range data {
					newSlice = reflect.Append(newSlice, reflect.ValueOf(b))
				}
			} else {
				size := 2
				if fieldDetail.DataType == PointDataTypeU32 || fieldDetail.DataType == PointDataTypeS32 {
					size = 4
				}
				for i := 0; i+size <= len(data); i += size {
					dataFloat64Before, err := parseDataToFloat64(data[i:i+size], fieldDetail.DataType, fieldDetail.OrderType)
					if err != nil {
						return err
					}
					val := reflect.ValueOf(cal(dataFloat64Before, fieldDetail.getCoefficient()) + fieldDetail.Offset)
					if elemType.Kind() == reflect.Float64 {
						newSlice = reflect.Append(newSlice, val)
					} else {
						newSlice = reflect.Append(newSlice, val.Convert(elemType))
					}
				}
			}

			value.Set(newSlice)
		default:
			return fmt.Errorf("parse for %s not supported", value.Type().Kind())
		}
	}
	return nil
}

func (m *Modbus) getFieldData(data []byte, values blocks, addr uint16, quantity uint16) []byte {
	for start, block := range values {
		if start <= addr && addr <= block.end {
			if addr+quantity-1 <= block.end {
				data = append(data, block.values[(addr-start)*2:(addr-start)*2+quantity*2]...)
				return data
			} else {
				data = append(data, block.values[(addr-start)*2:]...)
				return m.getFieldData(data, values, block.end+1, quantity-(block.end-addr+1))
			}
		}
	}
	return data
}

// readData allow to read quantiry larger than maxQuantity
func (m *Modbus) readData(conn Client, addr uint16, quantity uint16, registerType RegisterType) (results []byte, err error) {
	if quantity <= m.maxQuantity {
		return m.readDataByType(conn, addr, quantity, registerType)
	}
	for quantity > 0 {
		currentQuantity := min(quantity, m.maxQuantity)
		data, err := m.readDataByType(conn, addr, currentQuantity, registerType)
		if err != nil {
			return nil, err
		}
		results = append(results, data...)
		addr += currentQuantity
		quantity -= currentQuantity
	}
	return results, nil
}

// readDataByType reads data (not allow to exceed maxQuantity)
func (m *Modbus) readDataByType(conn Client, addr uint16, quantity uint16, registerType RegisterType) ([]byte, error) {
	switch registerType {
	case RegisterTypeHoldingRegister, RegisterTypeDefault:
		return conn.ReadHoldingRegisters(addr, quantity)
	case RegisterTypeInputRegister:
		return conn.ReadInputRegisters(addr, quantity)
	case RegisterTypeCoil:
		return conn.ReadCoils(addr, quantity)
	case RegisterTypeDiscreteInput:
		return conn.ReadDiscreteInputs(addr, quantity)
	default:
		return nil, fmt.Errorf("unsupported register type: %v", registerType)
	}
}

func (m Modbus) Put(ctx context.Context, conn Client) {
	if err := m.connPool.Put(conn); err != nil {
		// for log
	}
}

func min(a, b uint16) uint16 {
	if a < b {
		return a
	}
	return b
}

func parseFilter(fields []string) map[string]bool {
	m := map[string]bool{}
	for _, field := range fields {
		m[field] = true
	}
	return m
}

func cal(before, c float64) float64 {
	point := 1 / c
	return math.Round(before*c*point) / point
}
