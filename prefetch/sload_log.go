package prefetch

import (
	"fmt"
	"runtime"

	"github.com/ethereum/go-ethereum/common"
)

type Log struct {
	List []LogData
	Map  map[string][]interface{}
}

type LogData struct {
	Type string
	Data interface{}
}

func (log *Log) Write(t string, d interface{}) {
	if !DO_LOG || LOCK || getGoroutineID() != MAIN_THREAD_ID {
		return
	}
	log.List = append(log.List, LogData{Type: t, Data: d})
	if _, ok := log.Map[t]; !ok {
		log.Map[t] = make([]interface{}, 0)
	}
	log.Map[t] = append(log.Map[t], d)
}

func (log *Log) Init() {
	MAIN_THREAD_ID = getGoroutineID()
	DO_LOG = true
}

var DO_LOG = false //是否需要log，不需要就不记录
var LOCK = true
var LOG = Log{List: make([]LogData, 0), Map: make(map[string][]interface{})}

// var TOUCH_ADDR_LOG = Log{List: make([]LogData, 0), Map: make(map[string][]interface{})}
var MAIN_THREAD_ID int

func PrintLogLinear(log Log) {
	for _, l := range log.List {
		fmt.Println(l.Type, l.Data)
	}
}

// 用于确定主线程
func getGoroutineID() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	var id int
	fmt.Sscanf(string(buf[:n]), "goroutine %d ", &id)
	return id
}

// using to get touch address
// TODO 这里获取Address key和上面获取SLOAD 调用log一样写的规范一点，可以随时开关通道
type TouchLog struct {
	WhichTx common.Hash
	Address common.Address
	Key     common.Hash
	Value   common.Hash
}

var TOUCH_ADDR_CH chan TouchLog
var CURRENT_TX common.Hash
