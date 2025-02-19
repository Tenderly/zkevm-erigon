package rpchelper

import (
	"context"
	"sync/atomic"

	remote "github.com/tenderly/erigon/erigon-lib/gointerfaces/remote"
	types2 "github.com/tenderly/erigon/erigon-lib/gointerfaces/types"
	libcommon "github.com/tenderly/zkevm-erigon-lib/common"
	"github.com/tenderly/zkevm-erigon-lib/kv"

	"github.com/tenderly/zkevm-erigon/core/types"
	"github.com/tenderly/zkevm-erigon/p2p"
)

// ApiBackend - interface which must be used by API layer
// implementation can work with local Ethereum object or with Remote (grpc-based) one
// this is reason why all methods are accepting context and returning error
type ApiBackend interface {
	Etherbase(ctx context.Context) (libcommon.Address, error)
	NetVersion(ctx context.Context) (uint64, error)
	NetPeerCount(ctx context.Context) (uint64, error)
	ProtocolVersion(ctx context.Context) (uint64, error)
	ClientVersion(ctx context.Context) (string, error)
	Subscribe(ctx context.Context, cb func(*remote.SubscribeReply)) error
	SubscribeLogs(ctx context.Context, cb func(*remote.SubscribeLogsReply), requestor *atomic.Value) error
	BlockWithSenders(ctx context.Context, tx kv.Getter, hash libcommon.Hash, blockHeight uint64) (block *types.Block, senders []libcommon.Address, err error)
	EngineNewPayload(ctx context.Context, payload *types2.ExecutionPayload) (*remote.EnginePayloadStatus, error)
	EngineForkchoiceUpdated(ctx context.Context, request *remote.EngineForkChoiceUpdatedRequest) (*remote.EngineForkChoiceUpdatedResponse, error)
	EngineGetPayload(ctx context.Context, payloadId uint64) (*remote.EngineGetPayloadResponse, error)
	NodeInfo(ctx context.Context, limit uint32) ([]p2p.NodeInfo, error)
	Peers(ctx context.Context) ([]*p2p.PeerInfo, error)
	PendingBlock(ctx context.Context) (*types.Block, error)
	EngineGetPayloadBodiesByHashV1(ctx context.Context, request *remote.EngineGetPayloadBodiesByHashV1Request) (*remote.EngineGetPayloadBodiesV1Response, error)
	EngineGetPayloadBodiesByRangeV1(ctx context.Context, request *remote.EngineGetPayloadBodiesByRangeV1Request) (*remote.EngineGetPayloadBodiesV1Response, error)
}
