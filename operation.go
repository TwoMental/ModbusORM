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

	"github.com/goburrow/modbus"
	"github.com/pkg/errors"
	"golang.org/x/exp/constraints"
)

type ConnType uint8

const (
	ConnTypeTCP ConnType = 1
	ConnTypeRTU ConnType = 2
)

type Modbus struct {
	connType ConnType
	modbusTCP
	modbusRTU
	slaveID uint8
	timeout time.Duration

	points      Point
	maxQuantity uint16

	withBlock     bool
	maxBlockSize  uint16
	maxGapInBlock uint16

	connPool ConnPool
}

func newDefaultModbus() *Modbus {
	return &Modbus{
		slaveID:       1,
		maxQuantity:   125,
		timeout:       10 * time.Second,
		withBlock:     false,
		maxBlockSize:  100,
		maxGapInBlock: 10,
	}
}

func NewModbusTCP(host string, port int, point Point, opts ...ModbusOption) *Modbus {
	m := newDefaultModbus()
	m.connType = ConnTypeTCP
	m.modbusTCP = modbusTCP{
		Host:            host,
		Port:            port,
		MaxOpenConns:    3,
		ConnMaxLifetime: 30 * time.Minute,
	}
	m.points = point

	for _, opt := range opts {
		opt(m)
	}
	return m
}

func NewModbusRTU(comAddr string, point Point, opts ...ModbusOption) *Modbus {
	m := newDefaultModbus()
	m.connType = ConnTypeRTU
	m.modbusRTU = modbusRTU{
		ComAddr:  comAddr,
		BaudRate: 9600,
		DataBits: 8,
		Parity:   "N",
		StopBits: 1,
	}
	m.points = point

	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *Modbus) Conn() error {
	if m.connType == ConnTypeTCP {
		return m.connTCP()
	} else if m.connType == ConnTypeRTU {
		return m.connRTU()
	}
	return nil
}

func (m *Modbus) connTCP() error {
	addr := fmt.Sprintf("%s:%d", m.Host, m.Port)
	factory := func() (Client, error) {
		handler := modbus.NewTCPClientHandler(addr)
		handler.Timeout = m.timeout
		handler.SlaveId = m.slaveID
		if e := handler.Connect(); e != nil {
			return nil, e
		}
		client := modbus.NewClient(handler)
		return &ModbusTCPClient{Client: client, Handler: handler, createTime: time.Now()}, nil
	}
	config := ModbusTCPPoolConfig{
		MaxOpenConns:    m.MaxOpenConns,
		ConnMaxLifetime: m.ConnMaxLifetime,
	}

	pool, err := NewModbusTCPPool(config, factory)
	if err != nil {
		return fmt.Errorf("failed to create TCP pool: %w", err)
	}
	m.connPool = pool
	return nil
}

func (m *Modbus) connRTU() error {
	handler := modbus.NewRTUClientHandler(m.ComAddr)
	handler.BaudRate = m.BaudRate
	handler.DataBits = m.DataBits
	handler.Parity = m.Parity
	handler.StopBits = m.StopBits
	handler.SlaveId = m.slaveID
	handler.Timeout = m.timeout
	if e := handler.Connect(); e != nil {
		return e
	}
	client := modbus.NewClient(handler)

	pool, err := NewModbusRTUPool(&ModbusRTUClient{Client: client, Handler: handler, createTime: time.Now()})
	if err != nil {
		return fmt.Errorf("failed to create RTU pool: %w", err)
	}
	m.connPool = pool
	return nil

}

func (m *Modbus) Close() error {
	return m.connPool.Close()
}

// GetValue Get value from modbus and write to v.
func (m *Modbus) GetValue(ctx context.Context, point string, v any) error {
	fieldDetail, ok := m.points[point]
	if !ok {
		return fmt.Errorf("point for %s not found", point)
	}
	conn, err := m.connPool.Get()
	if err != nil {
		return fmt.Errorf("conn slave failed: %w", err)
	}
	defer m.connPool.Put(conn)

	data, err := conn.ReadHoldingRegisters(fieldDetail.Addr, fieldDetail.Quantity)
	if err != nil {
		return fmt.Errorf("ReadHoldingRegisters for %s failed, %w", point, err)
	}

	dataFloat64Before, err := parseDataToFloat64(data, fieldDetail.DataType, fieldDetail.OrderType)
	if err != nil {
		return err
	}
	dataFloat64 := cal(dataFloat64Before, fieldDetail.GetCoefficient()) + fieldDetail.Offset

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
		*v = byte2String(data)
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
*/func (m *Modbus) GetValues(ctx context.Context, v any, filter ...string) error {
	if m.withBlock {
		return m.GetValuesBlock(ctx, v, filter...)
	}
	return m.GetValuesSingle(ctx, v, filter...)
}

type block struct {
	start   uint16
	end     uint16
	vaulues []byte
}

type blocks map[uint16]*block

func (m *Modbus) GetValuesBlock(ctx context.Context, v any, filter ...string) error {
	// Get the address blocks
	addrMap := make(map[uint16]struct{})
	filterMap := parseFilter(filter)
	m.collectAddresses(ctx, v, addrMap, filterMap)
	if len(addrMap) == 0 {
		return fmt.Errorf("no address found")
	}

	// Convert the map to block list
	bs := m.addrMapToBlocks(ctx, addrMap)

	// Read the blocks
	err := m.readBlocks(ctx, bs)
	if err != nil {
		return err
	}

	// Set the values
	return m.setAddressValues(ctx, v, bs, filterMap)
}

func (m *Modbus) addrMapToBlocks(_ context.Context, addrMap map[uint16]struct{}) blocks {
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
		if addrs[i] == end+1 {
			end = addrs[i]
		} else if addrs[i]-end <= m.maxGapInBlock && end-start+1+addrs[i]-end <= m.maxBlockSize {
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

func (m *Modbus) collectAddresses(ctx context.Context, v any, addrMap map[uint16]struct{}, filterMap map[string]bool) error {
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
		var quantity uint16 = 1
		if fieldDetail.Quantity != 0 {
			quantity = fieldDetail.Quantity
		}
		var j uint16 = 0
		for ; j < quantity; j++ {
			addrMap[fieldDetail.Addr+j] = struct{}{}
		}

	}
	return nil
}

func (m *Modbus) readBlocks(_ context.Context, bs blocks) error {
	// Get a connection
	conn, err := m.connPool.Get()
	if err != nil {
		return fmt.Errorf("conn slave failed: %w", err)
	}
	defer m.connPool.Put(conn)
	// Read each block
	for _, b := range bs {
		data, err := m.readHoldingRegisters(conn, uint16(b.start), uint16(b.end-b.start+1))
		if err != nil {
			return err
		}
		if len(data) != int(b.end-b.start+1)*2 {
			return fmt.Errorf("read block failed, want %d, got %d", (b.end-b.start+1)*2, len(data))
		}
		b.vaulues = data
		// Avoid make server too busy
		time.Sleep(1 * time.Millisecond)
	}
	return nil
}

func (m *Modbus) setAddressValues(ctx context.Context, v any, values blocks, filterMap map[string]bool) error {
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
			if e := m.setAddressValues(ctx, addr.Interface(), values, filterMap); e != nil {
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
		var quantity uint16 = 1
		if fieldDetail.Quantity != 0 {
			quantity = fieldDetail.Quantity
		}
		// find data
		data := m.getFieldData([]byte{}, values, fieldDetail.Addr, quantity)

		// set value
		dataFloat64Before, err := parseDataToFloat64(data, fieldDetail.DataType, fieldDetail.OrderType)
		if err != nil {
			return err
		}
		dataFloat64 := cal(dataFloat64Before, fieldDetail.GetCoefficient()) + fieldDetail.Offset

		switch value.Type().Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			value.SetInt(int64(dataFloat64))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			value.SetUint(uint64(dataFloat64))
		case reflect.Float32, reflect.Float64:
			value.SetFloat(dataFloat64)
		case reflect.String:
			value.SetString(byte2String(data))
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
				newValue.SetString(byte2String(data))
			default:
				return fmt.Errorf("parse for %s pointer not supported", value.Type().Kind())
			}
			value.Set(newValue)
		case reflect.Slice, reflect.Array:
			newSlice := reflect.MakeSlice(value.Type(), 0, 0)

			if value.Type().Name() == "OriginByte" {
				for _, b := range data {
					newSlice = reflect.Append(newSlice, reflect.ValueOf(b))
				}
			} else {
				size := 2
				if fieldDetail.DataType == PointDataTypeU32 || fieldDetail.DataType == PointDataTypeS32 {
					size = 4
				}
				for i := 0; i+size < len(data); i += size {
					dataFloat64Before, err := parseDataToFloat64(data[i:i+size], fieldDetail.DataType, fieldDetail.OrderType)
					if err != nil {
						return err
					}
					newSlice = reflect.Append(newSlice, reflect.ValueOf(cal(dataFloat64Before, fieldDetail.GetCoefficient())+fieldDetail.Offset))
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
			if addr+quantity <= block.end {
				data = append(data, block.vaulues[(addr-start)*2:(addr-start)*2+quantity*2]...)
				return data
			} else {
				data = append(data, block.vaulues[addr-start:]...)
				return m.getFieldData(data, values, block.end+1, quantity)
			}
		}
	}
	return data
}

func (m *Modbus) GetValuesSingle(ctx context.Context, v any, filter ...string) error {
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

	// conn
	conn, err := m.connPool.Get()
	if err != nil {
		return fmt.Errorf("conn slave failed: %w", err)
	}
	defer m.connPool.Put(conn)

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
			if e := m.GetValues(ctx, addr.Interface(), filter...); e != nil {
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
		data, err := m.readHoldingRegisters(conn, fieldDetail.Addr, fieldDetail.Quantity)
		if err != nil {
			return fmt.Errorf("ReadHoldingRegisters for %s failed, %w", fieldName, err)
		}
		dataFloat64Before, err := parseDataToFloat64(data, fieldDetail.DataType, fieldDetail.OrderType)
		if err != nil {
			return err
		}
		dataFloat64 := cal(dataFloat64Before, fieldDetail.GetCoefficient()) + fieldDetail.Offset
		switch value.Type().Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			value.SetInt(int64(dataFloat64))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			value.SetUint(uint64(dataFloat64))
		case reflect.Float32, reflect.Float64:
			value.SetFloat(dataFloat64)
		case reflect.String:
			value.SetString(byte2String(data))
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
				newValue.SetString(byte2String(data))
			default:
				return fmt.Errorf("parse for %s pointer not supported", value.Type().Kind())
			}
			value.Set(newValue)
		case reflect.Slice, reflect.Array:
			newSlice := reflect.MakeSlice(value.Type(), 0, 0)

			if value.Type().Name() == "OriginByte" {
				for _, b := range data {
					newSlice = reflect.Append(newSlice, reflect.ValueOf(b))
				}
			} else {
				size := 2
				if fieldDetail.DataType == PointDataTypeU32 || fieldDetail.DataType == PointDataTypeS32 {
					size = 4
				}
				for i := 0; i+size < len(data); i += size {
					dataFloat64Before, err := parseDataToFloat64(data[i:i+size], fieldDetail.DataType, fieldDetail.OrderType)
					if err != nil {
						return err
					}
					newSlice = reflect.Append(newSlice, reflect.ValueOf(cal(dataFloat64Before, fieldDetail.GetCoefficient())+fieldDetail.Offset))
				}
			}
			value.Set(newSlice)
		default:
			return fmt.Errorf("parse for %s not supported", value.Type().Kind())
		}
	}
	return nil
}

// readHoldingRegisters allow to read quantiry larger than maxQuantity
func (m *Modbus) readHoldingRegisters(conn Client, address uint16, quantity uint16) (results []byte, err error) {
	if quantity <= m.maxQuantity {
		return conn.ReadHoldingRegisters(address, quantity)
	}
	for quantity > 0 {
		currentQuantity := min(quantity, m.maxQuantity)
		data, err := conn.ReadHoldingRegisters(address, currentQuantity)
		if err != nil {
			return nil, err
		}
		results = append(results, data...)
		address += currentQuantity
		quantity -= currentQuantity
	}
	return results, nil
}

func min[T constraints.Ordered](a, b T) T {
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

// SetValue set value to modbus from values.
func (m *Modbus) SetValue(ctx context.Context, point string, value any) error {
	fieldDetail, ok := m.points[point]
	if !ok {
		return fmt.Errorf("point for %s not found", point)
	}

	var buf bytes.Buffer
	err := binary.Write(&buf, binary.BigEndian, value)
	if err != nil {
		return err
	}
	data := buf.Bytes()

	quantity := uint16(len(data) / 2)
	if quantity != fieldDetail.Quantity {
		return fmt.Errorf("value length not match, want %d, got %d", fieldDetail.Quantity, quantity)
	}

	conn, err := m.connPool.Get()
	if err != nil {
		return fmt.Errorf("conn slave failed: %w", err)
	}
	defer m.connPool.Put(conn)

	var writeErr error
	if fieldDetail.Quantity == 1 {
		_, writeErr = conn.WriteSingleRegister(fieldDetail.Addr, binary.BigEndian.Uint16(data))
	} else {
		_, writeErr = conn.WriteMultipleRegisters(fieldDetail.Addr, quantity, data)
	}
	return writeErr
}

// SetValues: Set values to modbus from v.
/*
	Fields need to be set should have tag "morm"
*/
func (m *Modbus) SetValues(ctx context.Context, v any) error {
	addrValue, err := m.gatherAddrValue(ctx, v)
	if err != nil {
		return errors.Wrap(err, "gatherAddrValue failed")
	}
	return m.writeValues(ctx, addrValue)
}

type addrValue struct {
	addr     uint16
	quantity uint16
	value    uint16
	values   []byte
}

func (m *Modbus) gatherAddrValue(ctx context.Context, v any) ([]addrValue, error) {
	// real value and type
	var valueElem reflect.Value = reflect.ValueOf(v)
	var typeElem reflect.Type
	if valueElem.Kind() == reflect.Ptr || valueElem.Kind() == reflect.Interface {
		valueElem = valueElem.Elem()
		typeElem = reflect.TypeOf(v).Elem()
	} else {
		typeElem = reflect.TypeOf(v)
	}

	if typeElem.Kind() != reflect.Struct {
		return nil, fmt.Errorf("v must be struct or pointer of struct, not %s", typeElem.Kind())
	}

	fieldNum := valueElem.NumField()
	addrValues := make([]addrValue, 0, fieldNum)
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
			sub, err := m.gatherAddrValue(ctx, addr.Interface())
			if err != nil {
				return nil, fmt.Errorf("gatherAddrValue for %s failed: %w", typeElem.Field(i).Name, err)
			}
			addrValues = append(addrValues, sub...)
			continue
		}

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
		if fieldDetail.Quantity == 1 {
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
			addrValues = append(addrValues, addrValue{addr: fieldDetail.Addr, quantity: fieldDetail.Quantity, value: uint16((valueFloat - fieldDetail.Offset) / fieldDetail.GetCoefficient())})
		} else {
			// TODO: coefficent and offset
			addrValues = append(addrValues, addrValue{addr: fieldDetail.Addr, quantity: fieldDetail.Quantity, values: []byte(value.String())})
		}

	}
	return addrValues, nil
}

func (m *Modbus) writeValues(_ context.Context, addrValues []addrValue) error {
	// conn
	conn, err := m.connPool.Get()
	if err != nil {
		return fmt.Errorf("conn slave failed: %w", err)
	}
	defer m.connPool.Put(conn)

	// set
	for _, v := range addrValues {
		if v.quantity <= 1 {
			if _, err := conn.WriteSingleRegister(v.addr, v.value); err != nil {
				return errors.Wrap(err, "WriteSingleRegister failed")
			}
		} else {
			if _, err := conn.WriteMultipleRegisters(v.addr, v.quantity, v.values); err != nil {
				return errors.Wrap(err, "WriteMultipleRegisters failed")
			}
		}
	}
	return nil
}
