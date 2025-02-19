package core_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	libcommon "github.com/tenderly/zkevm-erigon-lib/common"
	"github.com/tenderly/zkevm-erigon-lib/kv"
	"github.com/tenderly/zkevm-erigon-lib/kv/memdb"

	"github.com/tenderly/zkevm-erigon/core"
	"github.com/tenderly/zkevm-erigon/turbo/rpchelper"

	"github.com/tenderly/zkevm-erigon/common"
	"github.com/tenderly/zkevm-erigon/core/state"
	"github.com/tenderly/zkevm-erigon/core/types"
	"github.com/tenderly/zkevm-erigon/params"
	"github.com/tenderly/zkevm-erigon/params/networkname"
)

func TestGenesisBlockHashes(t *testing.T) {
	db := memdb.NewTestDB(t)
	check := func(network string) {
		genesis := core.GenesisBlockByChainName(network)
		tx, err := db.BeginRw(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		_, block, err := core.WriteGenesisBlock(tx, genesis, nil, "")
		require.NoError(t, err)
		expect := params.GenesisHashByChainName(network)
		require.NotNil(t, expect, network)
		require.Equal(t, block.Hash().Bytes(), expect.Bytes(), network)
	}
	for _, network := range networkname.All {
		check(network)
	}
}

func TestHermezBlockRoots(t *testing.T) {
	require := require.New(t)

	t.Run("Hermez Mainnet", func(t *testing.T) {
		block, _, err := core.GenesisToBlock(core.HermezMainnetGenesisBlock(), "")
		require.NoError(err)
		if block.Root() != params.HermezMainnetGenesisHash {
			t.Errorf("wrong Hermez Mainnet genesis state root, got %v, want %v", block.Root(), params.HermezMainnetGenesisHash)
		}
	})

	t.Run("Hermez Testnet", func(t *testing.T) {
		block, _, err := core.GenesisToBlock(core.HermezTestnetGenesisBlock(), "")
		require.NoError(err)
		if block.Root() != params.HermezTestnetGenesisHash {
			t.Errorf("wrong Hermez Testnet genesis state root, got %v, want %v", block.Root(), params.HermezTestnetGenesisHash)
		}
	})

	t.Run("Hermez Blueberry", func(t *testing.T) {
		block, _, err := core.GenesisToBlock(core.HermezBlueberryGenesisBlock(), "")
		require.NoError(err)
		if block.Root() != params.HermezBlueberryGenesisHash {
			t.Errorf("wrong Hermez Testnet genesis state root, got %v, want %v", block.Root(), params.HermezTestnetGenesisHash)
		}
	})

	t.Run("Hermez Cardona", func(t *testing.T) {
		block, _, err := core.GenesisToBlock(core.HermezCardonaGenesisBlock(), "")
		require.NoError(err)
		if block.Root() != params.HermezCardonaGenesisHash {
			t.Errorf("wrong Hermez Cardona genesis state root, got %v, want %v", block.Root(), params.HermezCardonaGenesisHash)
		}
	})

	t.Run("Hermez Cardona Internal", func(t *testing.T) {
		block, _, err := core.GenesisToBlock(core.HermezCardonaInternalGenesisBlock(), "")
		require.NoError(err)
		if block.Root() != params.HermezCardonaInternalGenesisHash {
			t.Errorf("wrong Hermez Cardona Internal genesis state root, got %v, want %v", block.Root(), params.HermezCardonaInternalGenesisHash)
		}
	})
}

func TestX1BlockRoots(t *testing.T) {
	require := require.New(t)
	t.Run("X1 Testnet", func(t *testing.T) {
		block, _, err := core.GenesisToBlock(core.X1TestnetGenesisBlock(), "")
		require.NoError(err)
		if block.Root() != params.X1TestnetGenesisHash {
			t.Errorf("wrong X1 Testnet genesis state root, got %v, want %v", block.Root(), params.X1TestnetGenesisHash)
		}
	})
}

func TestGenesisBlockRoots(t *testing.T) {
	require := require.New(t)
	var err error

	block, _, _ := core.GenesisToBlock(core.MainnetGenesisBlock(), "")
	if block.Hash() != params.MainnetGenesisHash {
		t.Errorf("wrong mainnet genesis hash, got %v, want %v", block.Hash(), params.MainnetGenesisHash)
	}

	block, _, err = core.GenesisToBlock(core.GnosisGenesisBlock(), "")
	require.NoError(err)
	if block.Root() != params.GnosisGenesisStateRoot {
		t.Errorf("wrong Gnosis Chain genesis state root, got %v, want %v", block.Root(), params.GnosisGenesisStateRoot)
	}
	if block.Hash() != params.GnosisGenesisHash {
		t.Errorf("wrong Gnosis Chain genesis hash, got %v, want %v", block.Hash(), params.GnosisGenesisHash)
	}

	block, _, err = core.GenesisToBlock(core.ChiadoGenesisBlock(), "")
	require.NoError(err)
	if block.Root() != params.ChiadoGenesisStateRoot {
		t.Errorf("wrong Chiado genesis state root, got %v, want %v", block.Root(), params.ChiadoGenesisStateRoot)
	}
	if block.Hash() != params.ChiadoGenesisHash {
		t.Errorf("wrong Chiado genesis hash, got %v, want %v", block.Hash(), params.ChiadoGenesisHash)
	}
}

func TestCommitGenesisIdempotency(t *testing.T) {
	_, tx := memdb.NewTestTx(t)
	genesis := core.GenesisBlockByChainName(networkname.HermezMainnetChainName)
	_, _, err := core.WriteGenesisBlock(tx, genesis, nil, "")
	require.NoError(t, err)
	seq, err := tx.ReadSequence(kv.EthTx)
	require.NoError(t, err)
	require.Equal(t, uint64(2), seq)

	_, _, err = core.WriteGenesisBlock(tx, genesis, nil, "")
	require.NoError(t, err)
	seq, err = tx.ReadSequence(kv.EthTx)
	require.NoError(t, err)
	require.Equal(t, uint64(2), seq)
}

func TestAllocConstructor(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	// This deployment code initially sets contract's 0th storage to 0x2a
	// and its 1st storage to 0x01c9.
	deploymentCode := common.FromHex("602a5f556101c960015560048060135f395ff35f355f55")

	funds := big.NewInt(1000000000)
	address := libcommon.HexToAddress("0x1000000000000000000000000000000000000001")
	genSpec := &types.Genesis{
		Config: params.AllProtocolChanges,
		Alloc: types.GenesisAlloc{
			address: {Constructor: deploymentCode, Balance: funds},
		},
	}
	db := memdb.NewTestDB(t)
	defer db.Close()
	_, _, err := core.CommitGenesisBlock(db, genSpec, "")
	require.NoError(err)

	tx, err := db.BeginRo(context.Background())
	require.NoError(err)
	defer tx.Rollback()

	//TODO: support historyV3
	reader, err := rpchelper.CreateHistoryStateReader(tx, 1, 0, false, genSpec.Config.ChainName)
	require.NoError(err)
	state := state.New(reader)
	balance := state.GetBalance(address)
	assert.Equal(funds, balance.ToBig())
	code := state.GetCode(address)
	assert.Equal(common.FromHex("5f355f55"), code)

	key0 := libcommon.HexToHash("0000000000000000000000000000000000000000000000000000000000000000")
	storage0 := &uint256.Int{}
	state.GetState(address, &key0, storage0)
	assert.Equal(uint256.NewInt(0x2a), storage0)
	key1 := libcommon.HexToHash("0000000000000000000000000000000000000000000000000000000000000001")
	storage1 := &uint256.Int{}
	state.GetState(address, &key1, storage1)
	assert.Equal(uint256.NewInt(0x01c9), storage1)
}
