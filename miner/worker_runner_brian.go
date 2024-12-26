package miner

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
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
		// AlgoType:          ALGO_GREEDY, //调用fillTransactionsAlgoWorker默认使用的算法
		// AlgoType: ALGO_GREEDY_BUCKETS,
		AlgoType: ALGO_GREEDY_MULTISNAP,
		//AlgoType: ALGO_GREEDY_BUCKETS_MULTISNAP,
		GasPrice: DefaultConfig.GasPrice, //给的Tip，builder在从txpool取数据时会设置filter只取大于tip的tx(??????)
		//BuilderTxSigningKey: nil,
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
//用于单独测试worker
func RunWorker(db ethdb.Database, bc *core.BlockChain, bundles []types.MevBundle) *worker {
	//配置新建一个worker
	w, _ := newPrefetchTestWorker(bc, bc.Config(), ethash.NewFaker(), db, nil)            //新建worker
	fmt.Println("Tx in Txpool cnt:", len(w.eth.TxPool().Pending(txpool.PendingFilter{}))) //txpool会自动读取目录下的transactions.rlp中的tx数据加入池内

	//往txpool添加bundle
	for _, bundle := range bundles {
		err := w.eth.TxPool().AddMevBundle(bundle.Txs, bundle.BlockNumber, bundle.Uuid, bundle.SigningAddress, bundle.MinTimestamp, bundle.MaxTimestamp, bundle.RevertingTxHashes)
		check(err)
	}
	//从txpool中取mevbundle（测试是否成功添加bundle以及有多少bundle）
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
	if res.block != nil {
		fmt.Println("New block number:", res.block.Header().Number)
		fmt.Println("Profit:", res.fees, "Wei")
		fmt.Println("New block tx cnt:", len(res.block.Transactions()))
		fmt.Println("New block hash:", res.block.Hash().Hex())
		fmt.Println("New block gasused:", res.block.GasUsed())
	} else {
		fmt.Println("No block be generated!!!!!!!!!!")
	}

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
	return w
}

// 全局参数
var BUILDER_CHAIN *core.BlockChain
var BUILDER_DATABASE ethdb.Database
var BUILDER_WORKER *worker

type PrefetchEnv struct {
	Chain    *core.BlockChain
	Database ethdb.Database
	Worker   *worker
}

// 初始化worker环境
func InitEnv(db ethdb.Database, bc *core.BlockChain) PrefetchEnv {
	//初始化全局参数
	BUILDER_CHAIN = bc
	BUILDER_DATABASE = db
	BUILDER_WORKER, _ = newPrefetchTestWorker(bc, bc.Config(), bc.Engine(), db, nil) //old engine: ethash.NewFaker()
	return PrefetchEnv{Chain: bc, Database: db, Worker: BUILDER_WORKER}
}

// ------------------------------------------------------以下为搬运builder worke_test中的测试函数------------------------------------------
// 有些地方修改过
// testWorkerBackend implements worker.Backend interfaces and wraps all information needed during the testing.
// type prefetchTestWorkerBackend struct {
// 	db      ethdb.Database
// 	txPool  *txpool.TxPool
// 	chain   *core.BlockChain
// 	genesis *core.Genesis
// }

// // implements worker.Backend interfaces
// func (b *prefetchTestWorkerBackend) BlockChain() *core.BlockChain { return b.chain }
// func (b *prefetchTestWorkerBackend) TxPool() *txpool.TxPool       { return b.txPool }

// func newPrefetchTestWorkerBackend(chain *core.BlockChain, chainConfig *params.ChainConfig, engine consensus.Engine, db ethdb.Database, alloc types.GenesisAlloc /*n int,*/, gasLimit uint64) *prefetchTestWorkerBackend {
// 	if alloc == nil {
// 		alloc = prefetchDefaultGenesisAlloc
// 	}
// 	var gspec = &core.Genesis{
// 		Config:   chainConfig,
// 		GasLimit: gasLimit,
// 		Alloc:    alloc,
// 	}
// 	switch e := engine.(type) {
// 	case *clique.Clique:
// 		gspec.ExtraData = make([]byte, 32+common.AddressLength+crypto.SignatureLength)
// 		copy(gspec.ExtraData[32:32+common.AddressLength], prefetchTestBankAddress.Bytes())
// 		e.Authorize(prefetchTestBankAddress, func(account accounts.Account, s string, data []byte) ([]byte, error) {
// 			return crypto.Sign(crypto.Keccak256(data), prefetchTestBankKey)
// 		})
// 	case *ethash.Ethash:
// 	default:
// 		log.Fatalf("unexpected consensus engine type: %T", engine)
// 	}
// 	//chain, err := core.NewBlockChain(db, &core.CacheConfig{TrieDirtyDisabled: true}, gspec, nil, engine, vm.Config{}, nil, nil)
// 	// if err != nil {
// 	// 	log.Fatalf("core.NewBlockChain failed: %v", err)
// 	// }
// 	pool := legacypool.New(legacypool.DefaultConfig, chain)
// 	txpool, _ := txpool.New(legacypool.DefaultConfig.PriceLimit, chain, []txpool.SubPool{pool})

// 	return &prefetchTestWorkerBackend{
// 		db:      db,
// 		chain:   chain,
// 		txPool:  txpool,
// 		genesis: gspec,
// 	}
// }

type prefetchTestWorkerBackend struct {
	txPool *txpool.TxPool
	chain  *core.BlockChain
}

// 自定义LegacyPool配置
var TestLegacyPoolConfig = legacypool.Config{
	Journal:   "",
	Rejournal: time.Hour,

	PriceLimit: 1,
	PriceBump:  10,

	AccountSlots: 16,
	GlobalSlots:  4096 + 1024, // urgent + floating queue capacity with 4:1 ratio
	AccountQueue: 64,
	GlobalQueue:  1024,

	Lifetime:          3 * time.Hour,
	PrivateTxLifetime: 3 * 24 * time.Hour,
}

func newPrefetchTestWorkerBackend(chain *core.BlockChain) *prefetchTestWorkerBackend {
	pool := legacypool.New(TestLegacyPoolConfig, chain)
	txpool, _ := txpool.New(TestLegacyPoolConfig.PriceLimit, chain, []txpool.SubPool{pool})
	return &prefetchTestWorkerBackend{
		chain:  chain,
		txPool: txpool,
	}
}

// implements worker.Backend interfaces
func (b *prefetchTestWorkerBackend) BlockChain() *core.BlockChain { return b.chain }
func (b *prefetchTestWorkerBackend) TxPool() *txpool.TxPool       { return b.txPool }

func newPrefetchTestWorker(chain *core.BlockChain, chainConfig *params.ChainConfig, engine consensus.Engine, db ethdb.Database, alloc types.GenesisAlloc /*, blocks int*/) (*worker, *prefetchTestWorkerBackend) {
	//const GasLimit = 1_000_000_000_000_000_000
	//backend := newPrefetchTestWorkerBackend(chain, chainConfig, engine, db, alloc, GasLimit)
	backend := newPrefetchTestWorkerBackend(chain)
	//backend.txPool.Add(prefetchPendingTxs, true, false, false)

	//Brian Add:新建builder的Key，并写入builder配置文件
	var err error
	prefetchTestConfig.BuilderTxSigningKey, err = crypto.GenerateKey()
	check(err)
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

// public newPrefetchTestWorker
func NewPrefetchTestWorker(chain *core.BlockChain, chainConfig *params.ChainConfig, engine consensus.Engine, db ethdb.Database, alloc types.GenesisAlloc /*, blocks int*/) (*WorkerPtr, *prefetchTestWorkerBackend) {
	w, backend := newPrefetchTestWorker(chain, chainConfig, engine, db, alloc)
	return &WorkerPtr{Ptr: w}, backend
}

// // ---------------------------------------Invoke by builder runner-----------------------------------------
// TODO: we should move to a setup similar to catalyst local blocks & payload ids
func PrefetchBuildBlock(attrs *types.BuilderPayloadAttributes, sealedBlockCallback BlockHookFn) error {
	// Send a request to generate a full block in the background.
	// The result can be obtained via the returned channel.
	args := &BuildPayloadArgs{
		Parent:       attrs.HeadHash,
		Timestamp:    uint64(attrs.Timestamp),
		FeeRecipient: attrs.SuggestedFeeRecipient,
		GasLimit:     attrs.GasLimit,
		Random:       attrs.Random,
		Withdrawals:  attrs.Withdrawals,
		BeaconRoot:   attrs.ParentBeaconBlockRoot,
		BlockHook:    sealedBlockCallback,
	}

	payload, err := PrefetchBuildPayload(args)
	if err != nil {
		fmt.Println("Failed to build payload", "err", err)
		return err
	}

	resCh := make(chan *engine.ExecutionPayloadEnvelope, 1)
	go func() {
		resCh <- payload.ResolveFull()
	}()

	timer := time.NewTimer(4 * time.Second) //Brian Add ⭐️每个block限时打包4秒钟
	defer timer.Stop()

	select {
	case payload := <-resCh:
		if payload == nil {
			return errors.New("received nil payload from sealing work")
		}
		return nil
	case <-timer.C:
		payload.Cancel()
		fmt.Println("timeout waiting for block", "parent hash", attrs.HeadHash, "slot", attrs.Slot)
		return errors.New("timeout waiting for block result")
	}

}

func PrefetchBuildPayload(args *BuildPayloadArgs) (*Payload, error) {
	// Build the initial version with no transaction included. It should be fast
	// enough to run. The empty payload can at least make sure there is something
	// to deliver for not missing slot.
	var empty *newPayloadResult
	emptyParams := &generateParams{
		timestamp:   args.Timestamp,
		forceTime:   true,
		parentHash:  args.Parent,
		coinbase:    args.FeeRecipient,
		random:      args.Random,
		gasLimit:    args.GasLimit,
		withdrawals: args.Withdrawals,
		beaconRoot:  args.BeaconRoot,
		noTxs:       true,
	}

	// for _, worker := range w.workers {
	// 	empty = worker.getSealingBlock(emptyParams)
	// 	if empty.err != nil {
	// 		log.Error("could not start async block construction", "isFlashbotsWorker", worker.flashbots.isFlashbots, "#bundles", worker.flashbots.maxMergedBundles)
	// 		continue
	// 	}
	// 	break
	// }

	var w *worker = BUILDER_WORKER //用我们实验环境的WORKER
	empty = w.getSealingBlock(emptyParams)
	if empty.err != nil {
		fmt.Println("could not start async block construction:", empty.err)
	}

	if empty == nil || empty.block == nil {
		return nil, errors.New("no worker could build an empty block")
	}

	// Construct a payload object for return.
	payload := newPayload(empty.block, args.Id())

	// if len(w.workers) == 0 {
	// 	return payload, nil
	// }

	// Keep separate payloads for each worker so that ResolveFull actually resolves the best of all workers
	workerPayloads := []*Payload{}

	workerPayload := newPayload(empty.block, args.Id())
	workerPayloads = append(workerPayloads, workerPayload)
	fullParams := &generateParams{
		timestamp:   args.Timestamp,
		forceTime:   true,
		parentHash:  args.Parent,
		coinbase:    args.FeeRecipient,
		random:      args.Random,
		withdrawals: args.Withdrawals,
		beaconRoot:  args.BeaconRoot,
		gasLimit:    args.GasLimit,
		noTxs:       false,
		onBlock:     args.BlockHook,
	}

	go func(w *worker) { //Brian Add ⭐️并行getSealBlock 获取block
		// Update routine done elsewhere!
		start := time.Now()
		r := w.getSealingBlock(fullParams)
		if r.err == nil {
			workerPayload.update(r, time.Since(start))
		} else {
			fmt.Println("Error while sealing block", "err", r.err)
			workerPayload.Cancel()
		}
	}(w)

	go payload.resolveBestFullPayload(workerPayloads)

	return payload, nil
}
