package stagedsync

import (
	"github.com/tenderly/zkevm-erigon/sync_stages"
	"github.com/tenderly/zkevm-erigon/turbo/stages/bodydownload"
)

type DownloaderGlue interface {
	SpawnHeaderDownloadStage([]func() error, *sync_stages.StageState, sync_stages.Unwinder) error
	SpawnBodyDownloadStage(string, string, *sync_stages.StageState, sync_stages.Unwinder, *bodydownload.PrefetchedBlocks) (bool, error)
}
