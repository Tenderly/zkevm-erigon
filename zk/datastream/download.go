package datastream

import (
	"context"

	"github.com/tenderly/zkevm-erigon/zk/datastream/client"
	"github.com/tenderly/zkevm-erigon/zk/datastream/types"
	"github.com/pkg/errors"
)

// Download a set amount of blocks from datastream server to channel
func DownloadL2Blocks(ctx context.Context, datastreamUrl string, fromBlock uint64, l2BlocksAmount int) (*[]types.FullL2Block, *[]types.GerUpdate, map[uint64][]byte, uint64, error) {
	// Create client
	c := client.NewClient(ctx, datastreamUrl, 0, 0)

	// Start client (connect to the server)
	defer c.Stop()
	if err := c.Start(); err != nil {
		return nil, nil, nil, 0, errors.Wrap(err, "failed to start client")
	}

	// Create bookmark
	bookmark := types.NewL2BlockBookmark(fromBlock)

	// Read all entries from server
	l2Blocks, gerUpdates, bookmarks, entriesRead, err := c.ReadEntries(bookmark, l2BlocksAmount)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	return l2Blocks, gerUpdates, bookmarks, entriesRead, nil
}
