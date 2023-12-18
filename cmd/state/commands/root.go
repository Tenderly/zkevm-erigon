package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/spf13/cobra"
	chain2 "github.com/tenderly/zkevm-erigon/chain"
	"github.com/tenderly/zkevm-erigon/core/types"

	"github.com/tenderly/zkevm-erigon/turbo/debug"
	"github.com/tenderly/zkevm-erigon/turbo/logging"

	"github.com/tenderly/zkevm-erigon/cmd/utils"
	"github.com/tenderly/zkevm-erigon/core"
	"github.com/tenderly/zkevm-erigon/params"
)

var (
	genesisPath string
	genesis     *types.Genesis
	chainConfig *chain2.Config
)

func init() {
	utils.CobraFlags(rootCmd, debug.Flags, utils.MetricFlags, logging.Flags)
	rootCmd.PersistentFlags().StringVar(&genesisPath, "genesis", "", "path to genesis.json file")
}

var rootCmd = &cobra.Command{
	Use:   "state",
	Short: "state is a utility for Stateless ethereum clients",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if err := debug.SetupCobra(cmd); err != nil {
			panic(err)
		}

		genesis, chainConfig = getChainGenesisAndConfig()
		if genesisPath != "" {
			genesis = genesisFromFile(genesisPath)
		}
		if genesis.Config != nil && genesis.Config.ChainID.Cmp(chainConfig.ChainID) != 0 {
			utils.Fatalf("provided genesis.json chain configuration is invalid: expected chainId to be %v, got %v",
				chainConfig.ChainID.String(), genesis.Config.ChainID.String())
		}

		if chaindata == "" {
			chaindata = filepath.Join(datadirCli, "chaindata")
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		debug.Exit()
	},
}

func genesisFromFile(genesisPath string) *types.Genesis {
	file, err := os.Open(genesisPath)
	if err != nil {
		utils.Fatalf("Failed to read genesis file: %v", err)
	}
	defer file.Close()

	genesis := new(types.Genesis)
	if err := json.NewDecoder(file).Decode(genesis); err != nil {
		utils.Fatalf("invalid genesis file: %v", err)
	}
	return genesis
}

func getChainGenesisAndConfig() (genesis *types.Genesis, chainConfig *chain2.Config) {
	if chain == "" {
		genesis, chainConfig = core.MainnetGenesisBlock(), params.MainnetChainConfig
	} else {
		genesis, chainConfig = core.GenesisBlockByChainName(chain), params.ChainConfigByChainName(chain)
	}
	return genesis, chainConfig
}

func Execute() {
	ctx, cancel := common.RootContext()
	defer cancel()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
