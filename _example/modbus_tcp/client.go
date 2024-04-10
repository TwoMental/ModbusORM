package main

import (
	"context"
	"encoding/json"
	"log"

	modbusorm "github.com/TwoMental/modbus-orm"
)

func main() {
	conn := modbusorm.NewModbusTCP("localhost", 1502, point(),
		modbusorm.WithBlock(true),
	)
	if e := conn.Conn(); e != nil {
		log.Fatalf("Error: %s", e)
	}
	defer conn.Close()
	ctx := context.Background()
	data := &Data{}

	conn.GetValues(ctx, data)
	log.Printf("Read: %s", data.String())

	data.A = new(float64)
	data.B = 12.3
	data.C = []float64{10.1, 10.2, 10.3}
	data.E = "Yo"
	data.F = []int{1, -20, 30, -40, 50, 60, 70}
	conn.SetValues(ctx, data)
	log.Printf("Write: %s", data.String())

	conn.GetValues(ctx, data)
	log.Printf("Read: %s", data.String())
}

func point() modbusorm.Point {
	return modbusorm.Point{
		"a": modbusorm.PointDetails{
			Addr:        100,
			Quantity:    1,
			Coefficient: 0.1,
			DataType:    modbusorm.PointDataTypeU16,
		},
		"b": modbusorm.PointDetails{
			Addr:        101,
			Quantity:    1,
			Coefficient: 0.01,
			Offset:      0,
			DataType:    modbusorm.PointDataTypeU16,
		},
		"c": modbusorm.PointDetails{
			Addr:        600,
			Quantity:    3,
			Coefficient: 0.01,
			DataType:    modbusorm.PointDataTypeU16,
		},
		"d": modbusorm.PointDetails{
			Addr:     302,
			Quantity: 25,
			DataType: modbusorm.PointDataTypeU32,
		},
		"e": modbusorm.PointDetails{
			Addr:     404,
			Quantity: 7,
			DataType: modbusorm.PointDataTypeU16,
		},
		"f": modbusorm.PointDetails{
			Addr:     714,
			Quantity: 8,
			DataType: modbusorm.PointDataTypeS32,
		},
	}
}

type Data struct {
	A *float64  `morm:"a"`
	B float64   `morm:"b"`
	C []float64 `morm:"c"`
	// D modbusorm.OriginByte `morm:"d"`
	E string   `morm:"e"`
	F []int    `morm:"f"`
	Z *float64 `morm:"z"`
}

func (d *Data) String() string {
	jsonData, _ := json.Marshal(d)
	return string(jsonData)
}
