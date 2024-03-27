# ModbusORM
Object Relational Mapping (ORM) for Modbus

## What is ModbusORM
ModbusORM is a golang package allows you to read/write Modbus data by struct with tag (`morm`).

## Usage
- Define the points
    ```go
    // modbusorm.Point is a map with string key and modbusorm.PointDetails value.
    // key is the point name, which will be used in tag (morm)
    point := modbusorm.Point{
		"voltage": modbusorm.PointDetails{
			// Address of this point.
			Addr: 100,
			// Quantity of this poiont.
			Quantity: 1,
			// Coefficient of this point. Default 1.
			// For example:
			//      if read 10 from modbus server,
			//      and coefficient is 0.1,
			//      then you will get 1 in result.
			Coefficient: 0.1,
			// Data type of this point
			//      U16, S16, U32, S32
			DataType: modbusorm.PointDataTypeU16,
		},
	}
    ```
- Define a struct with `morm` tag.
    ```go
    type Data struct {
        Voltage     *float64             `morm:"voltage"`
        Temperature float64              `morm:"temperature"`
        Star        []float64            `morm:"star"`
        Origin      modbusorm.OriginByte `morm:"origin"`
        Word        string               `morm:"word"`
        Unknown     *float64             `morm:"unkonwn"`
    }
    ```
- Read/Write with your modbus server.
    ```go
    // new
	conn := modbusorm.NewModbusTCP(
		// Host of modbus server.
		"localhost",
		// Port of modbus server.
		1502,
		// Point define before.
		point,
		// Block mode setting. Default false.
		//  With block mode, ModbusORM will try to read data by block,
		//  rather than by single point.
		//  If set to true, two more parameters is avaliable.
		modbusorm.WithBlock(true),
		// Max block size. Default 100.
		//  Only work with block mode.
		modbusorm.WithMaxBlockSize(100),
		// Max gap in block. Default 10.
		//  Only work with block mode.
		modbusorm.WithMaxGapInBlock(10),
		// timeout setting.
		modbusorm.WithTimeout(10*time.Second),
		// max open connections in connection pool.
		modbusorm.WithMaxOpenConns(3),
		// max connection lifetime in connection pool.
		modbusorm.WithConnMaxLifetime(30*time.Minute),
	)
	// connect
	conn.Conn()
	// read
	data := &Data{}
	conn.GetValues(context.Background(), data)
    ```
- See more details in [_example](./_example/)

## Demo
- Modbus TCP
    - go to example folder:  `cd _example/modbus_tcp`
    - start a demo server: `go run server.go`
    - start a demo client: `go run client.go`

# TODOs
- [ ] README
- [x] Modbus RTU 
- [ ] Example
- [ ] More data type
- [ ] Logger