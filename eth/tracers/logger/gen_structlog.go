package logger

import (
	"encoding/json"
	"math/big"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/hexutility"

	"github.com/tenderly/zkevm-erigon/common/math"
	"github.com/tenderly/zkevm-erigon/core/vm"
)

var _ = (*structLogMarshaling)(nil)

// MarshalJSON marshals as JSON.
func (s StructLog) MarshalJSON() ([]byte, error) {
	type StructLog struct {
		Pc            uint64                            `json:"pc"`
		Op            vm.OpCode                         `json:"op"`
		Gas           math.HexOrDecimal64               `json:"gas"`
		GasCost       math.HexOrDecimal64               `json:"gasCost"`
		Memory        hexutility.Bytes                  `json:"memory"`
		MemorySize    int                               `json:"memSize"`
		Stack         []*math.HexOrDecimal256           `json:"stack"`
		ReturnStack   []math.HexOrDecimal64             `json:"returnStack"`
		ReturnData    hexutility.Bytes                  `json:"returnData"`
		Storage       map[libcommon.Hash]libcommon.Hash `json:"-"`
		Depth         int                               `json:"depth"`
		RefundCounter uint64                            `json:"refund"`
		Err           error                             `json:"-"`
		OpName        string                            `json:"opName"`
		ErrorString   string                            `json:"error"`
	}
	var enc StructLog
	enc.Pc = s.Pc
	enc.Op = s.Op
	enc.Gas = math.HexOrDecimal64(s.Gas)
	enc.GasCost = math.HexOrDecimal64(s.GasCost)
	enc.Memory = s.Memory
	enc.MemorySize = s.MemorySize
	if s.Stack != nil {
		enc.Stack = make([]*math.HexOrDecimal256, len(s.Stack))
		for k, v := range s.Stack {
			enc.Stack[k] = (*math.HexOrDecimal256)(v)
		}
	}
	enc.ReturnData = s.ReturnData
	enc.Storage = s.Storage
	enc.Depth = s.Depth
	enc.RefundCounter = s.RefundCounter
	enc.Err = s.Err
	enc.OpName = s.OpName()
	enc.ErrorString = s.ErrorString()
	return json.Marshal(&enc)
}

// UnmarshalJSON unmarshals from JSON.
func (s *StructLog) UnmarshalJSON(input []byte) error {
	type StructLog struct {
		Pc            *uint64                           `json:"pc"`
		Op            *vm.OpCode                        `json:"op"`
		Gas           *math.HexOrDecimal64              `json:"gas"`
		GasCost       *math.HexOrDecimal64              `json:"gasCost"`
		Memory        *hexutility.Bytes                 `json:"memory"`
		MemorySize    *int                              `json:"memSize"`
		Stack         []*math.HexOrDecimal256           `json:"stack"`
		ReturnData    *hexutility.Bytes                 `json:"returnData"`
		Storage       map[libcommon.Hash]libcommon.Hash `json:"-"`
		Depth         *int                              `json:"depth"`
		RefundCounter *uint64                           `json:"refund"`
		Err           error                             `json:"-"`
	}
	var dec StructLog
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Pc != nil {
		s.Pc = *dec.Pc
	}
	if dec.Op != nil {
		s.Op = *dec.Op
	}
	if dec.Gas != nil {
		s.Gas = uint64(*dec.Gas)
	}
	if dec.GasCost != nil {
		s.GasCost = uint64(*dec.GasCost)
	}
	if dec.Memory != nil {
		s.Memory = *dec.Memory
	}
	if dec.MemorySize != nil {
		s.MemorySize = *dec.MemorySize
	}
	if dec.Stack != nil {
		s.Stack = make([]*big.Int, len(dec.Stack))
		for k, v := range dec.Stack {
			s.Stack[k] = (*big.Int)(v)
		}
	}
	if dec.ReturnData != nil {
		s.ReturnData = *dec.ReturnData
	}
	if dec.Storage != nil {
		s.Storage = dec.Storage
	}
	if dec.Depth != nil {
		s.Depth = *dec.Depth
	}
	if dec.RefundCounter != nil {
		s.RefundCounter = *dec.RefundCounter
	}
	if dec.Err != nil {
		s.Err = dec.Err
	}
	return nil
}
