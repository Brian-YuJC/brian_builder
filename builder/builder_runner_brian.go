package builder

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/miner"
	"github.com/flashbots/go-boost-utils/utils"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

var BUILDER_BEACONCLIENT *BeaconClient

func InitBeaconClient() *BeaconClient {
	//var endpoint string = "http://10.119.187.21:9000"
	var endpoint string = "http://10.119.187.21:5052"
	var SlotsInEpoch uint64 = 32
	var SecondsInSlot uint64 = 12

	BUILDER_BEACONCLIENT = NewBeaconClient(endpoint, SlotsInEpoch, SecondsInSlot)
	return BUILDER_BEACONCLIENT
}

// 用于模拟运行整个builder，会调用miner/worker_runner_brian.go中的函数
func PrefetchOnPayloadAttribute(attrs *types.BuilderPayloadAttributes) {
	//w, _ := miner.NewPrefetchTestWorker(bc, db)
	slotCtx /*slotCtxCancel*/, _ := context.WithTimeout(context.Background(), 12*time.Second) //Brian Add: 设置prefetchRunBuildingJob的超时时间为12s
	proposerPubkey, err := utils.HexToPubkey(string("0xFF11FF11FF11FF11FF11FF11FF11FF11FF11FF11FF11FF11FF11FF11FF11FF11FF11FF11FF11FF11FF11FF11FF11FF11"))
	check(err)
	vd := ValidatorData{}
	// attrs := &types.BuilderPayloadAttributes{
	// 	Timestamp:             hexutil.Uint64(1587266067),
	// 	Random:                common.Hash{0x05, 0x10},
	// 	SuggestedFeeRecipient: common.Address{0x04, 0x10},
	// 	GasLimit:              uint64(0),
	// 	Slot:                  uint64(25),
	// }
	prefetchRunBuildingJob(slotCtx, proposerPubkey, vd, attrs)
}

// ------------------------------------------------------------------------------------------

func prefetchRunBuildingJob(slotCtx context.Context, proposerPubkey phase0.BLSPubKey, vd ValidatorData, attrs *types.BuilderPayloadAttributes) {
	ctx, cancel := context.WithTimeout(slotCtx, 12*time.Second) //Brian Add: 设置prefetchRunRetryLoop的超时时间为12s
	defer cancel()
	// Submission queue for the given payload attributes
	// multiple jobs can run for different attributes fot the given slot
	// 1. When new block is ready we check if its profit is higher than profit of last best block
	//    if it is we set queueBest* to values of the new block and notify queueSignal channel.
	// 2. Submission goroutine waits for queueSignal and submits queueBest* if its more valuable than
	//    queueLastSubmittedProfit keeping queueLastSubmittedProfit to be the profit of the last submission.
	//    Submission goroutine is globally rate limited to have fixed rate of submissions for all jobs.
	// Brian Add:
	// 	给定负载属性的提交队列对于给定的任务槽，可以运行针对不同属性的多个作业
	// 1.  当新块准备好时，我们检查它的利润是否高于上一个最佳块的利润。如果是，我们将queueBest*设置为新块的值，并通知队列信号通道。
	// 2.  提交goroutine等待queueSignal并提交queueBest*，如果它比queueLastSubmittedProfit更有价值，
	//     则保持queueLastSubmittedProfit为上次提交的利润。提交goroutine是全球速率限制的，所有工作都有固定的提交速率。
	var (
		queueSignal = make(chan struct{}, 1)

		queueMu                sync.Mutex
		queueLastSubmittedHash common.Hash
		queueBestEntry         blockQueueEntry
	)

	// log.Debug("runBuildingJob", "slot", attrs.Slot, "parent", attrs.HeadHash, "payloadTimestamp", uint64(attrs.Timestamp))

	// submitBestBlock := func() {
	// 	queueMu.Lock()
	// 	if queueBestEntry.block.Hash() != queueLastSubmittedHash {
	// 		submitBlockOpts := builder.SubmitBlockOpts{
	// 			Block:             queueBestEntry.block,
	// 			BlockValue:        queueBestEntry.blockValue,
	// 			BlobSidecars:      queueBestEntry.blobSidecars,
	// 			OrdersClosedAt:    queueBestEntry.ordersCloseTime,
	// 			SealedAt:          queueBestEntry.sealedAt,
	// 			CommitedBundles:   queueBestEntry.commitedBundles,
	// 			AllBundles:        queueBestEntry.allBundles,
	// 			UsedSbundles:      queueBestEntry.usedSbundles,
	// 			ProposerPubkey:    proposerPubkey,
	// 			ValidatorData:     vd,
	// 			PayloadAttributes: attrs,
	// 		}
	// 		err := b.onSealedBlock(submitBlockOpts)

	// 		if err != nil {
	// 			log.Error("could not run sealed block hook", "err", err)
	// 		} else {
	// 			queueLastSubmittedHash = queueBestEntry.block.Hash()
	// 		}
	// 	}
	// 	queueMu.Unlock()
	// }

	// // Avoid submitting early into a given slot. For example if slots have 12 second interval, submissions should
	// // not begin until 8 seconds into the slot.
	// slotTime := time.Unix(int64(attrs.Timestamp), 0).UTC()
	// slotSubmitStartTime := slotTime.Add(-b.submissionOffsetFromEndOfSlot)

	// // Empties queue, submits the best block for current job with rate limit (global for all jobs)
	// go runResubmitLoop(ctx, b.limiter, queueSignal, submitBestBlock, slotSubmitStartTime)

	// Populates queue with submissions that increase block profit
	blockHook := func(block *types.Block, blockValue *big.Int, sidecars []*types.BlobTxSidecar, ordersCloseTime time.Time,
		committedBundles, allBundles []types.SimulatedBundle, usedSbundles []types.UsedSBundle,
	) {
		if ctx.Err() != nil {
			return
		}

		sealedAt := time.Now()

		queueMu.Lock()
		defer queueMu.Unlock()
		if block.Hash() != queueLastSubmittedHash {
			queueBestEntry = blockQueueEntry{
				block:           block,
				blockValue:      new(big.Int).Set(blockValue),
				blobSidecars:    sidecars,
				ordersCloseTime: ordersCloseTime,
				sealedAt:        sealedAt,
				commitedBundles: committedBundles,
				allBundles:      allBundles,
				usedSbundles:    usedSbundles,
			}

			fmt.Println("\033[32m"+time.Now().Format("【2006-01-02 15:04:05.000】")+"\033[0m", "\033[34m【queueBestEntry Block Info】\033[0m") // Brian Add                                                                                                                     //Brian Add
			//打印新块相关信息
			if queueBestEntry.block != nil {
				fmt.Println("【New block number】", queueBestEntry.block.Header().Number)
				fmt.Println("【Profit】", queueBestEntry.blockValue, "Wei")
				fmt.Println("【New block tx cnt】", len(queueBestEntry.block.Transactions()))
				fmt.Println("【New block hash】", queueBestEntry.block.Hash().Hex())
				fmt.Println("【New block gasused】", queueBestEntry.block.GasUsed())
			} else {
				fmt.Println("No block be generated!!!!!!!!!!")
			}

			select {
			case queueSignal <- struct{}{}:
			default:
			}
		}
	}

	// resubmits block builder requests every builderBlockResubmitInterval
	// Brian Add: builderBlockResubmitInterval -> BlockResubmitIntervalDefault = 500 * time.Millisecond
	prefetchRunRetryLoop(ctx /*BlockResubmitIntervalDefault*/, 500*time.Millisecond, func() {
		fmt.Println("\n\033[32m"+time.Now().Format("【2006-01-02 15:04:05.000】")+"\033[0m", "\033[32m【Retrying BuildBlock】 \033[0m", //Brian Modify
			"slot", attrs.Slot,
			"parent", attrs.HeadHash,
			"resubmit-interval", BlockResubmitIntervalDefault.String())
		err := miner.PrefetchBuildBlock(attrs, blockHook)
		if err != nil {
			fmt.Println("\033[31mFailed to build block\033[0m", "err", err) //Brian Modify
		}
	})
}

// runRetryLoop calls retry periodically with the provided interval respecting context cancellation
func prefetchRunRetryLoop(ctx context.Context, interval time.Duration, retry func()) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			fmt.Println("\033[32m【Run Loop Done】\n\033[0m") //Brian Add
			return
		case <-t.C:
			start := time.Now() //Brian Add
			retry()
			fmt.Println("\033[32m【retry() Done】\033[0m", "Time:", time.Since(start)) //Brian Add
			//time.Sleep(6 * time.Second) // Brian Add
		}
	}
}

// // SubscribeToPayloadAttributesEvents subscribes to payload attributes events to validate fields such as prevrandao and withdrawals
// func PrefetchSubscribeToPayloadAttributesEvents(payloadAttrC chan types.BuilderPayloadAttributes) {
// 	payloadAttributesResp := new(PayloadAttributesEvent)

// 	eventsURL := fmt.Sprintf("%s/eth/v1/events?topics=payload_attributes", endpoint)
// 	log.Info("subscribing to payload_attributes events")

// 	for {
// 		client := sse.NewClient(eventsURL)
// 		err := client.SubscribeRawWithContext(b.ctx, func(msg *sse.Event) {
// 			err := json.Unmarshal(msg.Data, payloadAttributesResp)
// 			if err != nil {
// 				log.Error("could not unmarshal payload_attributes event", "err", err)
// 			} else {
// 				// convert capella.Withdrawal to types.Withdrawal
// 				var withdrawals []*types.Withdrawal
// 				for _, w := range payloadAttributesResp.Data.PayloadAttributes.Withdrawals {
// 					withdrawals = append(withdrawals, &types.Withdrawal{
// 						Index:     uint64(w.Index),
// 						Validator: uint64(w.ValidatorIndex),
// 						Address:   common.Address(w.Address),
// 						Amount:    uint64(w.Amount),
// 					})
// 				}

// 				data := types.BuilderPayloadAttributes{
// 					Slot:                  payloadAttributesResp.Data.ProposalSlot,
// 					HeadHash:              payloadAttributesResp.Data.ParentBlockHash,
// 					Timestamp:             hexutil.Uint64(payloadAttributesResp.Data.PayloadAttributes.Timestamp),
// 					Random:                payloadAttributesResp.Data.PayloadAttributes.PrevRandao,
// 					SuggestedFeeRecipient: payloadAttributesResp.Data.PayloadAttributes.SuggestedFeeRecipient,
// 					Withdrawals:           withdrawals,
// 					ParentBeaconBlockRoot: payloadAttributesResp.Data.PayloadAttributes.ParentBeaconBlockRoot,
// 				}
// 				payloadAttrC <- data
// 			}
// 		})
// 		if err != nil {
// 			log.Error("failed to subscribe to payload_attributes events", "err", err)
// 			time.Sleep(1 * time.Second)
// 		}
// 		log.Warn("beaconclient SubscribeRaw ended, reconnecting")
// 	}
// }
