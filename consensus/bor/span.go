package bor

import (
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/tenderly/zkevm-erigon/consensus"
	"github.com/tenderly/zkevm-erigon/consensus/bor/heimdall/span"
	"github.com/tenderly/zkevm-erigon/consensus/bor/valset"
)

//go:generate mockgen -destination=./span_mock.go -package=bor . Spanner
type Spanner interface {
	GetCurrentSpan(syscall consensus.SystemCall) (*span.Span, error)
	GetCurrentValidators(blockNumber uint64, signer libcommon.Address, getSpanForBlock func(blockNum uint64) (*span.HeimdallSpan, error)) ([]*valset.Validator, error)
	CommitSpan(heimdallSpan span.HeimdallSpan, syscall consensus.SystemCall) error
}
