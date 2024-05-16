package graph

import (
	"github.com/gateway-fm/cdk-erigon-lib/kv"
	"github.com/tenderly/zkevm-erigon/cmd/rpcdaemon/commands"
	"github.com/tenderly/zkevm-erigon/turbo/rpchelper"
	"github.com/tenderly/zkevm-erigon/turbo/services"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	GraphQLAPI  commands.GraphQLAPI
	db          kv.RoDB
	filters     *rpchelper.Filters
	blockReader services.FullBlockReader
}
