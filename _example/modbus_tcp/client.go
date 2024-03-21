package main

import (
	"context"
	"encoding/json"
	"log"

	modbusorm "github.com/TwoMental/modbus-orm"
)

func main() {
	conn := modbusorm.NewModbusTCP("localhost", 1502, point())
	if e := conn.Conn(); e != nil {
		log.Fatalf("Error: %s", e)
	}
	defer conn.Close()
	data := &Data{}
	conn.GetValues(context.Background(), data)
	b, _ := json.Marshal(data)
	log.Printf("Data: %s", b)
}

func point() modbusorm.Point {
	return modbusorm.Point{
		"voltage": modbusorm.PointDetails{
			Addr:        100,
			Quantity:    1,
			Coefficient: 0.1,
			DataType:    modbusorm.PointDataTypeU16,
		},
		"temperature": modbusorm.PointDetails{
			Addr:        101,
			Quantity:    1,
			Coefficient: 0.01,
			Offset:      -10,
			DataType:    modbusorm.PointDataTypeU16,
		},
	}
}

type Data struct {
	Voltage     *float64 `morm:"voltage"`
	Temperature float64  `morm:"temperature"`
	Unknown     *float64 `morm:"unkonwn"`
}
