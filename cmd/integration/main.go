package main

import (
	"fmt"
	"os"

	"github.com/tenderly/zkevm-erigon-lib/common"
	"github.com/tenderly/zkevm-erigon/cmd/integration/commands"
)

func main() {
	rootCmd := commands.RootCommand()
	ctx, _ := common.RootContext()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
