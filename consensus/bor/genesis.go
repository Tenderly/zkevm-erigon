package bor

import (
	"math/big"

	"github.com/tenderly/zkevm-erigon/consensus"
	"github.com/tenderly/zkevm-erigon/consensus/bor/clerk"
)

//go:generate mockgen -destination=./genesis_contract_mock.go -package=bor . GenesisContract
type GenesisContract interface {
	CommitState(event *clerk.EventRecordWithTime, syscall consensus.SystemCall) error
	LastStateId(syscall consensus.SystemCall) (*big.Int, error)
}
