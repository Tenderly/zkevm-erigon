package stagedsync

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ledgerwatch/log/v3"
	"github.com/tenderly/secp256k1"
	libcommon "github.com/tenderly/zkevm-erigon-lib/common"
	"github.com/tenderly/zkevm-erigon-lib/common/cmp"
	"github.com/tenderly/zkevm-erigon-lib/common/hexutility"
	"github.com/tenderly/zkevm-erigon-lib/common/length"
	"github.com/tenderly/zkevm-erigon-lib/etl"
	"github.com/tenderly/zkevm-erigon-lib/kv"
	"github.com/tenderly/zkevm-erigon/chain"

	"github.com/tenderly/zkevm-erigon/common/dbutils"
	"github.com/tenderly/zkevm-erigon/common/debug"
	"github.com/tenderly/zkevm-erigon/core/rawdb"
	"github.com/tenderly/zkevm-erigon/core/types"
	"github.com/tenderly/zkevm-erigon/ethdb/prune"
	"github.com/tenderly/zkevm-erigon/sync_stages"
	"github.com/tenderly/zkevm-erigon/turbo/snapshotsync"
	"github.com/tenderly/zkevm-erigon/turbo/stages/headerdownload"
)

type SendersCfg struct {
	db              kv.RwDB
	batchSize       int
	blockSize       int
	bufferSize      int
	numOfGoroutines int
	readChLen       int
	badBlockHalt    bool
	tmpdir          string
	prune           prune.Mode
	chainConfig     *chain.Config
	blockRetire     *snapshotsync.BlockRetire
	hd              *headerdownload.HeaderDownload
}

func StageSendersCfg(db kv.RwDB, chainCfg *chain.Config, badBlockHalt bool, tmpdir string, prune prune.Mode, br *snapshotsync.BlockRetire, hd *headerdownload.HeaderDownload) SendersCfg {
	const sendersBatchSize = 10000
	const sendersBlockSize = 4096

	return SendersCfg{
		db:              db,
		batchSize:       sendersBatchSize,
		blockSize:       sendersBlockSize,
		bufferSize:      (sendersBlockSize * 10 / 20) * 10000, // 20*4096
		numOfGoroutines: secp256k1.NumOfContexts(),            // we can only be as parallels as our crypto library supports,
		readChLen:       4,
		badBlockHalt:    badBlockHalt,
		tmpdir:          tmpdir,
		chainConfig:     chainCfg,
		prune:           prune,
		blockRetire:     br,
		hd:              hd,
	}
}

func SpawnRecoverSendersStage(cfg SendersCfg, s *sync_stages.StageState, u sync_stages.Unwinder, tx kv.RwTx, toBlock uint64, ctx context.Context, quiet bool) error {
	if cfg.blockRetire != nil && cfg.blockRetire.Snapshots() != nil && cfg.blockRetire.Snapshots().Cfg().Enabled && s.BlockNumber < cfg.blockRetire.Snapshots().BlocksAvailable() {
		s.BlockNumber = cfg.blockRetire.Snapshots().BlocksAvailable()
	}

	quitCh := ctx.Done()
	useExternalTx := tx != nil
	if !useExternalTx {
		var err error
		tx, err = cfg.db.BeginRw(context.Background())
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// zk
	prevStageProgress, errStart := sync_stages.GetStageProgress(tx, sync_stages.Batches)
	if errStart != nil {
		return errStart
	}

	if prevStageProgress == 0 {
		prevStageProgress, errStart = sync_stages.GetStageProgress(tx, sync_stages.Headers)
		if errStart != nil {
			return errStart
		}
	}

	var to = prevStageProgress
	if toBlock > 0 {
		to = cmp.Min(prevStageProgress, toBlock)
	}
	if to < s.BlockNumber {
		return nil
	}
	logPrefix := s.LogPrefix()
	if !quiet && to > s.BlockNumber+16 {
		log.Info(fmt.Sprintf("[%s] Started", logPrefix), "from", s.BlockNumber, "to", to)
	}

	logEvery := time.NewTicker(30 * time.Second)
	defer logEvery.Stop()

	canonicalC, err := tx.Cursor(kv.HeaderCanonical)
	if err != nil {
		return err
	}
	defer canonicalC.Close()

	startFrom := s.BlockNumber + 1
	currentHeaderIdx := uint64(0)
	canonical := make([]libcommon.Hash, to-s.BlockNumber)

	for k, v, err := canonicalC.Seek(hexutility.EncodeTs(startFrom)); k != nil; k, v, err = canonicalC.Next() {
		if err != nil {
			return err
		}
		if err := libcommon.Stopped(quitCh); err != nil {
			return err
		}

		if currentHeaderIdx >= to-s.BlockNumber { // if header stage is ehead of body stage
			break
		}

		copy(canonical[currentHeaderIdx][:], v)
		currentHeaderIdx++

		select {
		default:
		case <-logEvery.C:
			log.Info(fmt.Sprintf("[%s] Preload headers", logPrefix), "block_number", binary.BigEndian.Uint64(k))
		}
	}
	log.Trace(fmt.Sprintf("[%s] Read canonical hashes", logPrefix), "amount", len(canonical))

	jobs := make(chan *senderRecoveryJob, cfg.batchSize)
	out := make(chan *senderRecoveryJob, cfg.batchSize)
	wg := new(sync.WaitGroup)
	wg.Add(cfg.numOfGoroutines)
	ctx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()
	for i := 0; i < cfg.numOfGoroutines; i++ {
		go func(threadNo int) {
			defer debug.LogPanic()
			defer wg.Done()
			// each goroutine gets it's own crypto context to make sure they are really parallel
			recoverSenders(ctx, logPrefix, secp256k1.ContextForThread(threadNo), cfg.chainConfig, jobs, out, quitCh)
		}(i)
	}

	collectorSenders := etl.NewCollector(logPrefix, cfg.tmpdir, etl.NewSortableBuffer(etl.BufferOptimalSize))
	defer collectorSenders.Close()

	errCh := make(chan senderRecoveryError)
	go func() {
		defer debug.LogPanic()
		defer close(errCh)
		defer cancelWorkers()
		var ok bool
		var j *senderRecoveryJob
		for {
			select {
			case <-quitCh:
				return
			case <-logEvery.C:
				n := s.BlockNumber
				if j != nil {
					n += uint64(j.index)
				}
				log.Info(fmt.Sprintf("[%s] Recovery", logPrefix), "block_number", n, "ch", fmt.Sprintf("%d/%d", len(jobs), cap(jobs)))
			case j, ok = <-out:
				if !ok {
					return
				}
				if j.err != nil {
					errCh <- senderRecoveryError{err: j.err, blockNumber: j.blockNumber, blockHash: j.blockHash}
					return
				}

				k := make([]byte, 4)
				binary.BigEndian.PutUint32(k, uint32(j.index))
				index := int(binary.BigEndian.Uint32(k))
				if err := collectorSenders.Collect(dbutils.BlockBodyKey(s.BlockNumber+uint64(index)+1, canonical[index]), j.senders); err != nil {
					errCh <- senderRecoveryError{err: j.err}
					return
				}
			}
		}
	}()

	var minBlockNum uint64 = math.MaxUint64
	var minBlockHash libcommon.Hash
	var minBlockErr error
	handleRecoverErr := func(recErr senderRecoveryError) error {
		if recErr.blockHash == (libcommon.Hash{}) {
			return recErr.err
		}

		if recErr.blockNumber < minBlockNum {
			minBlockNum = recErr.blockNumber
			minBlockHash = recErr.blockHash
			minBlockErr = recErr.err
		}
		return nil
	}

	bodiesC, err := tx.Cursor(kv.BlockBody)
	if err != nil {
		return err
	}
	defer bodiesC.Close()

Loop:
	for k, _, err := bodiesC.Seek(hexutility.EncodeTs(startFrom)); k != nil; k, _, err = bodiesC.Next() {
		if err != nil {
			return err
		}
		if err := libcommon.Stopped(quitCh); err != nil {
			return err
		}

		blockNumber := binary.BigEndian.Uint64(k[:8])
		blockHash := libcommon.BytesToHash(k[8:])

		if blockNumber > to {
			break
		}

		if canonical[blockNumber-s.BlockNumber-1] != blockHash {
			// non-canonical case
			continue
		}

		body := rawdb.ReadCanonicalBodyWithTransactions(tx, blockHash, blockNumber)
		if body == nil {
			log.Warn(fmt.Sprintf("[%s] ReadCanonicalBodyWithTransactions can't find block", logPrefix), "num", blockNumber, "hash", blockHash)
			continue
		}

		select {
		case recoveryErr := <-errCh:
			if recoveryErr.err != nil {
				cancelWorkers()
				if err := handleRecoverErr(recoveryErr); err != nil {
					return err
				}
				break Loop
			}
		case jobs <- &senderRecoveryJob{body: body, key: k, blockNumber: blockNumber, blockHash: blockHash, index: int(blockNumber - s.BlockNumber - 1)}:
		}
	}

	close(jobs)
	wg.Wait()
	close(out)
	for recoveryErr := range errCh {
		if recoveryErr.err != nil {
			cancelWorkers()
			if err := handleRecoverErr(recoveryErr); err != nil {
				return err
			}
		}
	}
	if minBlockErr != nil {
		log.Error(fmt.Sprintf("[%s] Error recovering senders for block %d %x): %v", logPrefix, minBlockNum, minBlockHash, minBlockErr))
		if cfg.badBlockHalt {
			return minBlockErr
		}
		minHeader := rawdb.ReadHeader(tx, minBlockHash, minBlockNum)
		if cfg.hd != nil {
			cfg.hd.ReportBadHeaderPoS(minBlockHash, minHeader.ParentHash)
		}
		if to > s.BlockNumber {
			u.UnwindTo(minBlockNum-1, minBlockHash)
		}
	} else {
		if err := collectorSenders.Load(tx, kv.Senders, etl.IdentityLoadFunc, etl.TransformArgs{
			Quit: quitCh,
			LogDetailsLoad: func(k, v []byte) (additionalLogArguments []interface{}) {
				return []interface{}{"block", binary.BigEndian.Uint64(k)}
			},
		}); err != nil {
			return err
		}
		if err = s.Update(tx, to); err != nil {
			return err
		}
	}

	if !useExternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

type senderRecoveryError struct {
	err         error
	blockNumber uint64
	blockHash   libcommon.Hash
}

type senderRecoveryJob struct {
	body        *types.Body
	key         []byte
	senders     []byte
	blockHash   libcommon.Hash
	blockNumber uint64
	index       int
	err         error
}

func recoverSenders(ctx context.Context, logPrefix string, cryptoContext *secp256k1.Context, config *chain.Config, in, out chan *senderRecoveryJob, quit <-chan struct{}) {
	var job *senderRecoveryJob
	var ok bool
	for {
		select {
		case job, ok = <-in:
			if !ok {
				return
			}
			if job == nil {
				return
			}
		case <-ctx.Done():
			return
		case <-quit:
			return
		}

		body := job.body
		signer := types.MakeSigner(config, job.blockNumber)
		job.senders = make([]byte, len(body.Transactions)*length.Addr)
		for i, tx := range body.Transactions {
			from, err := signer.SenderWithContext(cryptoContext, tx)
			if err != nil {
				job.err = fmt.Errorf("%s: error recovering sender for tx=%x, %w", logPrefix, tx.Hash(), err)
				break
			}
			copy(job.senders[i*length.Addr:], from[:])
		}

		// prevent sending to close channel
		if err := libcommon.Stopped(quit); err != nil {
			job.err = err
		} else if err = libcommon.Stopped(ctx.Done()); err != nil {
			job.err = err
		}
		out <- job

		if errors.Is(job.err, libcommon.ErrStopped) {
			return
		}
	}
}

func UnwindSendersStage(s *sync_stages.UnwindState, tx kv.RwTx, cfg SendersCfg, ctx context.Context) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = cfg.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	if err = s.Done(tx); err != nil {
		return err
	}
	if !useExternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func PruneSendersStage(s *sync_stages.PruneState, tx kv.RwTx, cfg SendersCfg, ctx context.Context) (err error) {
	logEvery := time.NewTicker(logInterval)
	defer logEvery.Stop()
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = cfg.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}
	sn := cfg.blockRetire.Snapshots()
	if !(sn != nil && sn.Cfg().Enabled && sn.Cfg().Produce) && cfg.prune.TxIndex.Enabled() {
		to := cfg.prune.TxIndex.PruneTo(s.ForwardProgress)
		if err = rawdb.PruneTable(tx, kv.Senders, to, ctx, 1_000); err != nil {
			return err
		}
	}

	if !useExternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
