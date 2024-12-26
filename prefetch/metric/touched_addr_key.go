package metric

import (
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
)

var ADDR_KEY_LOG_PATH string = "./prefetch_list.log"
var FILE *os.File
var err error

// func init() {
// 	FILE, err = os.OpenFile(ADDR_KEY_LOG_PATH, os.O_CREATE|os.O_WRONLY, 0644)
// 	if err != nil {
// 		fmt.Println(err)
// 	}
// }

func InitOutputAddrKeyLog() {
	FILE, err = os.OpenFile(ADDR_KEY_LOG_PATH, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println(err)
	}
}

func OutputAddrKeyLog(addr common.Address, key common.Hash) {
	fmt.Fprintln(FILE, addr.Hex(), key.Hex())
}

/*
一个工具获取某个运行环境下的所有touch到的addr和key
用于最优prefetch测量
*/
