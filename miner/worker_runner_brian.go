package miner

import (
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/clique"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/txpool/legacypool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
)

var (
	//prefetchTestTxPoolConfig legacypool.Config
	//prefetchEthashChainConfig *params.ChainConfig

	//prefetchPendingTxs []*types.Transaction

	//⭐️配置builder
	prefetchTestConfig = &Config{
		Recommit:          DefaultConfig.Recommit,
		NewPayloadTimeout: DefaultConfig.NewPayloadTimeout,
		GasCeil:           DefaultConfig.GasCeil,
		AlgoType:          ALGO_GREEDY,            //调用fillTransactionsAlgoWorker默认使用的算法
		GasPrice:          DefaultConfig.GasPrice, //给的Tip，builder在从txpool取数据时会设置filter只取大于tip的tx(??????)
	}

	prefetchTestBankKey, _  = crypto.GenerateKey()
	prefetchTestBankAddress = crypto.PubkeyToAddress(prefetchTestBankKey.PublicKey)
	prefetchTestBankFunds   = big.NewInt(1000000000000000000)

	prefetchDefaultGenesisAlloc = types.GenesisAlloc{prefetchTestBankAddress: {Balance: prefetchTestBankFunds}}
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

/*
模拟环境，新建一个worker然后调用worker的函数模拟运行bundle或者打包区块

	func (w *worker) generateWork(params *generateParams) *newPayloadResult{
			->
		    fillTransactionsSelectAlgo
		      	->
				fillTransactionsAlgoWorker
					->
					getSimulatedBundles
						->
						simulateBundles
							->
							computeBundleGas
	}
*/
func RunBuilder(db ethdb.Database, bc *core.BlockChain, bundles []types.MevBundle) {
	//配置新建一个worker
	w, _ := newPrefetchTestWorker(bc, bc.GetChainConfig(), ethash.NewFaker(), db, nil) //新建worker
	fmt.Println("Tx in Txpool cnt:", len(w.eth.TxPool().Pending(txpool.PendingFilter{})))

	//往txpool添加bundle
	for _, bundle := range bundles {
		err := w.eth.TxPool().AddMevBundle(bundle.Txs, bundle.BlockNumber, bundle.Uuid, bundle.SigningAddress, bundle.MinTimestamp, bundle.MaxTimestamp, bundle.RevertingTxHashes)
		check(err)
	}
	var blockTimestamp uint64
	mev_bundles, _ := w.eth.TxPool().MevBundles(new(big.Int).Add(bc.CurrentBlock().Number, common.Big1), blockTimestamp)
	fmt.Println("MEV Bundle get from txpool cnt:", len(mev_bundles))

	//模拟启动generateWork打包一个区块
	timestamp := uint64(time.Now().Unix())
	testUserKey, _ := crypto.GenerateKey()
	testUserAddress := crypto.PubkeyToAddress(testUserKey.PublicKey)
	res := w.generateWork(&generateParams{ //启动generateWork
		parentHash:  w.chain.CurrentBlock().Hash(),
		timestamp:   timestamp, //w.chain.CurrentHeader().Time + 12
		coinbase:    testUserAddress,
		random:      common.Hash{},
		withdrawals: nil,
		beaconRoot:  nil,
		noTxs:       false,
		forceTime:   true,
		onBlock:     nil,
		gasLimit:    30_000_000,
	})
	//打印新块相关信息
	fmt.Println("New block number:", res.block.Header().Number)
	fmt.Println("Profit:", res.fees)
	fmt.Println("New block tx cnt:", len(res.block.Transactions()))
	fmt.Println("New block hash:", res.block.Hash().Hex())
	//fmt.Println(w.chain.CurrentBlock().Number)

	// 测试：fillTransactionsAlgoWorker
	// env, err := w.prepareWork(&generateParams{gasLimit: 30_000_000}) //新建simulate所需运行环境
	// check(err)
	// blockBundles, _, _, mempoolTxHashes, err := w.fillTransactionsAlgoWorker(nil, env)
	// check(err)
	// fmt.Println("blockBundles size:", len(blockBundles))
	// fmt.Println("mempoolTxHashes size:", len(mempoolTxHashes))

	// 测试：simulateBundles
	// pending := w.eth.TxPool().Pending(txpool.PendingFilter{ //提取池中交易
	// 	MinTip: uint256.MustFromBig(big.NewInt(0)),
	// })
	// fmt.Println("pending tx cnt:", len(pending))
	// simBundles, simSbundle, err := w.simulateBundles(env, nil, nil, pending) //调用simulate函数
	// check(err)
	// fmt.Println(simBundles, simSbundle)
}

// ------------------------------------------------------以下为搬运builder worke_test中的测试函数------------------------------------------
// 有些地方修改过
// testWorkerBackend implements worker.Backend interfaces and wraps all information needed during the testing.
type prefetchTestWorkerBackend struct {
	db      ethdb.Database
	txPool  *txpool.TxPool
	chain   *core.BlockChain
	genesis *core.Genesis
}

// implements worker.Backend interfaces
func (b *prefetchTestWorkerBackend) BlockChain() *core.BlockChain { return b.chain }
func (b *prefetchTestWorkerBackend) TxPool() *txpool.TxPool       { return b.txPool }

func newPrefetchTestWorkerBackend(chain *core.BlockChain, chainConfig *params.ChainConfig, engine consensus.Engine, db ethdb.Database, alloc types.GenesisAlloc /*n int,*/, gasLimit uint64) *prefetchTestWorkerBackend {
	if alloc == nil {
		alloc = prefetchDefaultGenesisAlloc
	}
	var gspec = &core.Genesis{
		Config:   chainConfig,
		GasLimit: gasLimit,
		Alloc:    alloc,
	}
	switch e := engine.(type) {
	case *clique.Clique:
		gspec.ExtraData = make([]byte, 32+common.AddressLength+crypto.SignatureLength)
		copy(gspec.ExtraData[32:32+common.AddressLength], prefetchTestBankAddress.Bytes())
		e.Authorize(prefetchTestBankAddress, func(account accounts.Account, s string, data []byte) ([]byte, error) {
			return crypto.Sign(crypto.Keccak256(data), prefetchTestBankKey)
		})
	case *ethash.Ethash:
	default:
		log.Fatalf("unexpected consensus engine type: %T", engine)
	}
	//chain, err := core.NewBlockChain(db, &core.CacheConfig{TrieDirtyDisabled: true}, gspec, nil, engine, vm.Config{}, nil, nil)
	// if err != nil {
	// 	log.Fatalf("core.NewBlockChain failed: %v", err)
	// }
	pool := legacypool.New(legacypool.DefaultConfig, chain)
	txpool, _ := txpool.New(legacypool.DefaultConfig.PriceLimit, chain, []txpool.SubPool{pool})

	return &prefetchTestWorkerBackend{
		db:      db,
		chain:   chain,
		txPool:  txpool,
		genesis: gspec,
	}
}

func newPrefetchTestWorker(chain *core.BlockChain, chainConfig *params.ChainConfig, engine consensus.Engine, db ethdb.Database, alloc types.GenesisAlloc /*, blocks int*/) (*worker, *prefetchTestWorkerBackend) {
	const GasLimit = 1_000_000_000_000_000_000
	backend := newPrefetchTestWorkerBackend(chain, chainConfig, engine, db, alloc, GasLimit)
	//backend.txPool.Add(prefetchPendingTxs, true, false, false)
	w := newWorker(prefetchTestConfig, chainConfig, engine, backend, new(event.TypeMux), nil, false, &flashbotsData{
		isFlashbots: prefetchTestConfig.AlgoType != ALGO_MEV_GETH,
		queue:       nil,
		bundleCache: NewBundleCache(),
		algoType:    prefetchTestConfig.AlgoType,
	})
	if prefetchTestConfig.BuilderTxSigningKey == nil {
		w.setEtherbase(prefetchTestBankAddress)
	}

	return w, backend
}
