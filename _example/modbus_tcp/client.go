package main

import (
	"context"
	"encoding/json"
	"log"

	modbusorm "github.com/TwoMental/modbus-orm"
)

func main() {
	conn := modbusorm.NewModbusTCP("localhost", 1502, point(),
		modbusorm.WithBlock(false),
	)
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
		"star": modbusorm.PointDetails{
			Addr:        600,
			Quantity:    3,
			Coefficient: 0.1,
			DataType:    modbusorm.PointDataTypeU16,
		},
		"origin": modbusorm.PointDetails{
			Addr:     302,
			Quantity: 25,
			DataType: modbusorm.PointDataTypeU32,
		},
		"word": modbusorm.PointDetails{
			Addr:     404,
			Quantity: 7,
			DataType: modbusorm.PointDataTypeU16,
		},
	}
}

type Data struct {
	Voltage     *float64             `morm:"voltage"`
	Temperature float64              `morm:"temperature"`
	Star        []float64            `morm:"star"`
	Origin      modbusorm.OriginByte `morm:"origin"`
	Word        string               `morm:"word"`
	Unknown     *float64             `morm:"unkonwn"`
}
