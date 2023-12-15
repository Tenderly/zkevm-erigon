package main

import (
	"os"

	"github.com/ledgerwatch/log/v3"
	"github.com/spf13/cobra"
	"github.com/tenderly/erigon/erigon-lib/common"
	"github.com/tenderly/zkevm-erigon/cmd/rpcdaemon/cli"
	"github.com/tenderly/zkevm-erigon/cmd/rpcdaemon/commands"
	"github.com/tenderly/zkevm-erigon/consensus/ethash"
	"github.com/tenderly/zkevm-erigon/turbo/logging"
)

func main() {
	cmd, cfg := cli.RootCommand()
	rootCtx, rootCancel := common.RootContext()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		logging.SetupLoggerCmd("rpcdaemon", cmd)
		db, borDb, backend, txPool, mining, stateCache, blockReader, ff, agg, err := cli.RemoteServices(ctx, *cfg, log.Root(), rootCancel)
		if err != nil {
			log.Error("Could not connect to DB", "err", err)
			return nil
		}
		defer db.Close()
		if borDb != nil {
			defer borDb.Close()
		}

		// TODO: Replace with correct consensus Engine
		engine := ethash.NewFaker()
		apiList := commands.APIList(db, borDb, backend, txPool, mining, ff, stateCache, blockReader, agg, *cfg, engine, "")
		if err := cli.StartRpcServer(ctx, *cfg, apiList, nil); err != nil {
			log.Error(err.Error())
			return nil
		}

		return nil
	}

	if err := cmd.ExecuteContext(rootCtx); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}
