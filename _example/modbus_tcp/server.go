package main

import (
	"encoding/binary"
	"log"
	"math"
	"os"
	"sync/atomic"
	"time"

	"github.com/goburrow/modbus"

	"github.com/tbrandon/mbserver"
)

// from: https://github.com/TechplexEngineer/modbus-sim/blob/main/main.go

func main() {
	if err := run(); err != nil {
		log.Printf("Error: %s", err)
		os.Exit(1)
	}
}

func run() error {
	serv := mbserver.NewServer()
	startTime := uint32(time.Now().Unix() & 0xffffffff)
	staMsb := uint16((startTime >> 16) & 0xffff)
	staLsb := uint16(startTime & 0xffff)
	log.Printf("msb:%d lsb:%d", staMsb, staLsb)

	for i := 0; i < 2000; i++ {
		serv.HoldingRegisters[i] = uint16(i)
	}

	word := []byte("Hello, World!!")
	for i := 0; i < len(word)-1; i += 2 {
		serv.HoldingRegisters[404+i/2] = binary.BigEndian.Uint16(word[i : i+2])
	}

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		var uptime uint32 = 0
		for {
			<-ticker.C
			atomic.AddUint32(&uptime, 1)
		}
	}()

	serv.RegisterFunctionHandler(modbus.FuncCodeReadInputRegisters, ReadHoldingRegisters) //note this is a hack
	serv.RegisterFunctionHandler(modbus.FuncCodeReadHoldingRegisters, ReadHoldingRegisters)

	listenAddr := "0.0.0.0:1502"

	log.Printf("Modbus Server listening on %s", listenAddr)
	err := serv.ListenTCP(listenAddr)
	if err != nil {
		return err
	}
	defer serv.Close()

	// Wait forever
	for {
		time.Sleep(1 * time.Second)
	}
}

//func ReadInputRegisters(s *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
//	register, numRegs, endRegister := registerAddressAndNumber(frame)
//	if endRegister > 65536 {
//		return []byte{}, &mbserver.IllegalDataAddress
//	}
//	return append([]byte{byte(numRegs * 2)}, mbserver.Uint16ToBytes(s.InputRegisters[register:endRegister])...), &mbserver.Success
//}

func ReadHoldingRegisters(s *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	register, numRegs, endRegister := registerAddressAndNumber(frame)
	if endRegister > 65536 {
		return []byte{}, &mbserver.IllegalDataAddress
	}
	log.Printf("Read r:%d, count:%d", register, numRegs)

	// get the current unix timestamp, converted as a 32-bit unsigned integer for simplicity
	unixTs := uint32(time.Now().Unix() & 0xffffffff)

	switch register {
	//200 "artificially generates error"
	case 201:
		return []byte{}, &mbserver.IllegalFunction
	case 202:
		return []byte{}, &mbserver.IllegalDataAddress
	case 203:
		return []byte{}, &mbserver.IllegalDataValue
	case 204:
		return []byte{}, &mbserver.SlaveDeviceFailure
	case 205:
		return []byte{}, &mbserver.AcknowledgeSlave
	case 206:
		return []byte{}, &mbserver.SlaveDeviceBusy
	case 207:
		return []byte{}, &mbserver.NegativeAcknowledge
	case 208:
		return []byte{}, &mbserver.MemoryParityError
	case 210:
		return []byte{}, &mbserver.GatewayPathUnavailable
	case 211:
		return []byte{}, &mbserver.GatewayTargetDeviceFailedtoRespond

	//300 uptime msb
	//301 uptime lsb
	case 300:
		fallthrough
	case 301:
		if numRegs > 2 {
			return []byte{}, &mbserver.IllegalDataAddress
		}
		time32 := uint32(time.Now().Unix() & 0xffffffff)

		staMsb := s.HoldingRegisters[302] // application start time msb
		staLsb := s.HoldingRegisters[303] // application start time lsb
		startTime := uint32(staMsb)<<16 + uint32(staLsb)

		uptime := time32 - startTime

		msb := uint16((uptime >> 16) & 0xffff)
		lsb := uint16(uptime & 0xffff)
		bits := []uint16{msb, lsb}
		return append([]byte{byte(numRegs * 2)}, mbserver.Uint16ToBytes(bits[register-300:endRegister-300])...), &mbserver.Success

	//400 unixtime msb
	//401 unixtime lsb
	case 400:
		fallthrough
	case 401:
		startingAddress := 400
		if numRegs > 2 {
			return []byte{}, &mbserver.IllegalDataAddress
		}
		msb := uint16((unixTs >> 16) & 0xffff)
		lsb := uint16(unixTs & 0xffff)
		bits := []uint16{msb, lsb}
		return append([]byte{byte(numRegs * 2)}, mbserver.Uint16ToBytes(bits[register-startingAddress:endRegister-startingAddress])...), &mbserver.Success

	//500 math.pi msb
	case 500:
		fallthrough
	//501 math.pi lsb
	case 501:
		startingAddress := 500
		if numRegs > 2 {
			return []byte{}, &mbserver.IllegalDataAddress
		}
		pi32 := math.Float32bits(math.Pi)
		msb := uint16((pi32 >> 16) & 0xffff)
		lsb := uint16(pi32 & 0xffff)
		bits := []uint16{msb, lsb}
		return append([]byte{byte(numRegs * 2)}, mbserver.Uint16ToBytes(bits[register-startingAddress:endRegister-startingAddress])...), &mbserver.Success
	}
	return append([]byte{byte(numRegs * 2)}, mbserver.Uint16ToBytes(s.HoldingRegisters[register:endRegister])...), &mbserver.Success
}

func registerAddressAndNumber(frame mbserver.Framer) (register int, numRegs int, endRegister int) {
	data := frame.GetData()
	register = int(binary.BigEndian.Uint16(data[0:2]))
	numRegs = int(binary.BigEndian.Uint16(data[2:4]))
	endRegister = register + numRegs
	return register, numRegs, endRegister
}
