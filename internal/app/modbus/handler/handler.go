/**
 * Copyright 2019 Rightech IoT. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package handler

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/Rightech/ric-edge/pkg/jsonrpc"
	"github.com/Rightech/ric-edge/third_party/goburrow/modbus"
	"github.com/stretchr/objx"
)

type PackagerFn func(byte) modbus.Packager

type Service struct {
	transport      modbus.Transporter
	packagerGetter PackagerFn
}

func New(transport modbus.Transporter, pGetter PackagerFn) Service {
	return Service{transport, pGetter}
}

func (s Service) getClient(slaveID byte) modbus.Client {
	return modbus.NewClient2(s.packagerGetter(slaveID), s.transport)
}

func (s Service) Call(req jsonrpc.Request) (res interface{}, err error) {
	switch req.Method {
	case "modbus-read-coil":
		res, err = s.readCoils(req.Params)
	case "modbus-read-discrete":
		res, err = s.readDiscreteInputs(req.Params)
	case "modbus-write-coil":
		res, err = s.writeSingleCoil(req.Params)
	case "modbus-write-multiple-coils":
		res, err = s.writeMultipleCoils(req.Params)
	case "modbus-read-input":
		res, err = s.readInputRegisters(req.Params)
	case "modbus-read-holding":
		res, err = s.readHoldingRegisters(req.Params)
	case "modbus-write-register":
		res, err = s.writeSingleRegister(req.Params)
	case "modbus-write-multiple-registers":
		res, err = s.writeMultipleRegisters(req.Params)
	// case "read-write-multiple-registers":
	// 	res, err = s.h.ReadWriteMultipleRegisters(req.Params)
	// case "mask-write-register":
	// 	res, err = s.h.MaskWriteRegister(req.Params)
	// case "read-fifo-queue":
	// 	res, err = s.h.ReadFIFOQueue(req.Params)
	default:
		err = jsonrpc.ErrMethodNotFound.AddData("method", req.Method)
	}

	return
}

const (
	maxUint16 = int64(^uint16(0))
	minUint16 = int64(0)

	maxByte = int64(255)
	minByte = int64(0)
)

func parseResult(b []byte) []uint16 {
	if len(b)%2 != 0 {
		res := make([]uint16, len(b))
		for i, v := range b {
			res[i] = uint16(v)
		}

		return res
	}

	res := make([]uint16, 0, len(b)/2)

	for i := 0; i < len(b)-1; i += 2 {
		res = append(res, binary.BigEndian.Uint16(b[i:i+2]))
	}

	return res
}

func getInt64(params objx.Map, k string, def ...int64) (int64, error) {
	val := params.Get(k)
	if val.IsNil() {
		if len(def) > 0 {
			return def[0], nil
		}

		return 0, jsonrpc.ErrInvalidParams.AddData("msg", k+" required")
	}

	number, ok := val.Data().(json.Number)
	if !ok {
		return 0, jsonrpc.ErrInvalidParams.AddData("msg", k+" should be number")
	}

	var (
		value int64
		err   error
	)

	if value, err = number.Int64(); err != nil {
		return 0, jsonrpc.ErrInvalidParams.AddData("msg", k+" should be int")
	}

	return value, nil
}

func getSlaveID(params objx.Map) (byte, error) {
	value, err := getInt64(params, "slave_id", 0)
	if err != nil {
		return 0, err
	}

	if !(minByte <= value && value <= maxByte) {
		return 0, jsonrpc.ErrInvalidParams.AddData("msg", "slave_id should be byte")
	}

	return byte(value), nil
}

func getUint16(params objx.Map, k string, def ...int64) (uint16, error) {
	value, err := getInt64(params, k, def...)
	if err != nil {
		return 0, err
	}

	if !(minUint16 <= value && value <= maxUint16) {
		return 0, jsonrpc.ErrInvalidParams.AddData("msg", k+" should be uint16")
	}

	return uint16(value), nil
}

func getTwoUint16(params objx.Map, k1, k2 string) (uint16, uint16, error) {
	v1, err := getUint16(params, k1)
	if err != nil {
		return 0, 0, err
	}

	v2, err := getUint16(params, k2)
	if err != nil {
		return 0, 0, err
	}

	return v1, v2, nil
}

func getBytes(params objx.Map, k string) ([]byte, error) {
	v1 := params.Get(k)

	if !v1.IsStr() {
		return nil, jsonrpc.ErrInvalidParams.AddData("msg", k+" required and should be base64")
	}

	decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(v1.Str()))
	decoded, err := ioutil.ReadAll(decoder)
	if err != nil {
		return nil, jsonrpc.ErrInvalidParams.AddData("msg", err.Error())
	}

	return decoded, nil
}

func getAddrAndQuantity(params objx.Map) (uint16, uint16, error) {
	return getTwoUint16(params, "address", "quantity")
}

func getAddrAndValue(params objx.Map) (uint16, uint16, error) {
	return getTwoUint16(params, "address", "value")
}

func (s Service) readCoils(params objx.Map) (interface{}, error) {
	addr, quantity, err := getAddrAndQuantity(params)
	if err != nil {
		return nil, err
	}

	slaveID, err := getSlaveID(params)
	if err != nil {
		return nil, err
	}

	cli := s.getClient(slaveID)

	res, err := cli.ReadCoils(addr, quantity)
	if err != nil {
		return nil, err
	}

	return parseResult(res), nil
}

func (s Service) readDiscreteInputs(params objx.Map) (interface{}, error) {
	addr, quantity, err := getAddrAndQuantity(params)
	if err != nil {
		return nil, err
	}

	slaveID, err := getSlaveID(params)
	if err != nil {
		return nil, err
	}

	cli := s.getClient(slaveID)

	res, err := cli.ReadDiscreteInputs(addr, quantity)
	if err != nil {
		return nil, err
	}

	return parseResult(res), nil
}

const modbusTrueValue = 0xFF00

func (s Service) writeSingleCoil(params objx.Map) (interface{}, error) {
	addr, value, err := getAddrAndValue(params)
	if err != nil {
		return nil, err
	}

	if value > 1 {
		return nil, jsonrpc.ErrInvalidParams.
			AddData("msg", "bad value. only 0 or 1 allowed").
			AddData("v", value)
	}

	if value == 1 {
		value = modbusTrueValue
	}

	slaveID, err := getSlaveID(params)
	if err != nil {
		return nil, err
	}

	cli := s.getClient(slaveID)

	res, err := cli.WriteSingleCoil(addr, value)
	if err != nil {
		return nil, err
	}

	result := parseResult(res)

	if result[0] == modbusTrueValue {
		result[0] = 1
	}

	return result[0], nil
}

func (s Service) writeMultipleCoils(params objx.Map) (interface{}, error) {
	addr, quantity, err := getAddrAndQuantity(params)
	if err != nil {
		return nil, err
	}

	value, err := getBytes(params, "value")
	if err != nil {
		return nil, err
	}

	slaveID, err := getSlaveID(params)
	if err != nil {
		return nil, err
	}

	cli := s.getClient(slaveID)

	res, err := cli.WriteMultipleCoils(addr, quantity, value)
	if err != nil {
		return nil, err
	}

	return parseResult(res), nil
}

func (s Service) readInputRegisters(params objx.Map) (interface{}, error) {
	addr, quantity, err := getAddrAndQuantity(params)
	if err != nil {
		return nil, err
	}

	slaveID, err := getSlaveID(params)
	if err != nil {
		return nil, err
	}

	cli := s.getClient(slaveID)

	res, err := cli.ReadInputRegisters(addr, quantity)
	if err != nil {
		return nil, err
	}

	return parseResult(res), nil
}

func (s Service) readHoldingRegisters(params objx.Map) (interface{}, error) {
	addr, quantity, err := getAddrAndQuantity(params)
	if err != nil {
		return nil, err
	}

	slaveID, err := getSlaveID(params)
	if err != nil {
		return nil, err
	}

	cli := s.getClient(slaveID)

	res, err := cli.ReadHoldingRegisters(addr, quantity)
	if err != nil {
		return nil, err
	}

	return parseResult(res), nil
}

func (s Service) writeSingleRegister(params objx.Map) (interface{}, error) {
	addr, value, err := getAddrAndValue(params)
	if err != nil {
		return nil, err
	}

	slaveID, err := getSlaveID(params)
	if err != nil {
		return nil, err
	}

	cli := s.getClient(slaveID)

	res, err := cli.WriteSingleRegister(addr, value)
	if err != nil {
		return nil, err
	}

	return parseResult(res), nil
}

func (s Service) writeMultipleRegisters(params objx.Map) (interface{}, error) {
	addr, quantity, err := getAddrAndQuantity(params)
	if err != nil {
		return nil, err
	}

	value, err := getBytes(params, "value")
	if err != nil {
		return nil, err
	}

	slaveID, err := getSlaveID(params)
	if err != nil {
		return nil, err
	}

	cli := s.getClient(slaveID)

	res, err := cli.WriteMultipleRegisters(addr, quantity, value)
	if err != nil {
		return nil, err
	}

	return parseResult(res), nil
}

// func (s Service) readWriteMultipleRegisters(params objx.Map) (interface{}, error) {
// 	readAddr, readQuantity, err := getTwoUint16(params, "read_address", "read_quantity")
// 	if err != nil {
// 		return nil, err
// 	}

// 	writeAddr, writeQuantity, err := getTwoUint16(params, "write_address", "write_quantity")
// 	if err != nil {
// 		return nil, err
// 	}

// 	value, err := getBytes(params, "value")
// 	if err != nil {
// 		return nil, err
// 	}

// 	return s.cli.ReadWriteMultipleRegisters(readAddr, readQuantity, writeAddr, writeQuantity, value)
// }

// func (s Service) maskWriteRegister(params objx.Map) (interface{}, error) {
// 	addr, andMask, err := getTwoUint16(params, "address", "and_mask")
// 	if err != nil {
// 		return nil, err
// 	}

// 	orMask, err := getUint16(params, "or_mask")
// 	if err != nil {
// 		return nil, err
// 	}

// 	return s.cli.MaskWriteRegister(addr, andMask, orMask)
// }

// func (s Service) readFIFOQueue(params objx.Map) (interface{}, error) {
// 	addr, err := getUint16(params, "address")
// 	if err != nil {
// 		return nil, err
// 	}

// 	return s.cli.ReadFIFOQueue(addr)
// }
