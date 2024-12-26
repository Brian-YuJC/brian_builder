package prefetch

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

// 读取需要prefetch的Address-slot
func ReadPrefetchList() map[common.Address][]common.Hash {
	//path := "../output/addr_hash.log"
	path := "./prefetch_list.log"
	file, err := os.Open(path)
	if err != nil {
		fmt.Println(err)
	}
	prefetch_map := make(map[common.Address][]common.Hash)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		s := strings.Split(strings.Split(line, "\n")[0], " ")
		var addr common.Address
		var hash common.Hash
		byte1, err := hex.DecodeString(s[0][2:])
		check(err)
		addr.SetBytes(byte1)
		byte2, err := hex.DecodeString(s[1][2:])
		check(err)
		hash.SetBytes(byte2)
		//fmt.Fprintln(os.Stderr, addr, hash)
		if _, ok := prefetch_map[addr]; !ok {
			prefetch_map[addr] = make([]common.Hash, 0)
		}
		prefetch_map[addr] = append(prefetch_map[addr], hash)
	}
	return prefetch_map
}

// 这是prefetch到statedb的缓存中
// TODO：是否真有prefetch待确认
// 这个prefetch完后好像运行效率反而降低了一倍 TODO：看看为什么（是因为在StateDB层prefetch开了太多无关的stateobject吗）
func StateDB_Prefetch(statedb *state.StateDB, prefetch_map map[common.Address][]common.Hash) {
	for addr, v := range prefetch_map {
		for _, hash := range v {
			statedb.GetState(addr, hash) //这里直接就是直接调用实现SLOAD的核心函数GetState进行prefetch
		}
	}
}

// 这是直接prefetch到triedb backend的hashdb 的缓存中
// TODO：是否真有prefetch待确认
// 这里prefetch的东西太底层了，我们现在只有key的hash，这里传入的hash应该是要通过查Trie得到value的hash而不是key的hash？？？？？？
// 也可以prefetch在Trie上检索value时的某几个经过的node（可以进行最细粒度的prefetch（单独prefetch路径上的node））
func TrieDB_Prefetch(triedb *triedb.Database, prefetch_map map[common.Address][]common.Hash) {
	backendDB := triedb.GetBackend()
	if hashdb, ok := backendDB.(*hashdb.Database); ok { // 这里获取实现triedb的数据库，hashdb或者pathdb(这里断言成hashdb)
		for _, v := range prefetch_map {
			for _, hash := range v {
				_, err := hashdb.Node(hash)
				check(err)
			}
		}
	} else {
		panic("Cannot transfer to *hashdb.Database type")
	}
}

// 基于Trie的prefetch
// TODO：是否真有prefetch待确认
func Trie_Prefetch(Trie state.Trie, prefetch_map map[common.Address][]common.Hash) {
	if state_trie, ok := Trie.(*trie.StateTrie); ok {
		for _, v := range prefetch_map {
			for _, key := range v {
				_, err := state_trie.GetStorage(common.Address{}, key.Bytes()) //感觉这里是不是要开一颗addr对应的storagetrie先
				//应该调用这个才对不然root都不一样GetStorage用的是statetrie的root不是storagetrie的
				//func (t *Trie) get(origNode node, key []byte, pos int) (value []byte, newnode node, didResolve bool, err error) {
				//fmt.Println(data) //debug TODO：目前这里data为空
				check(err)
			}
		}
	} else {
		panic("Cannot transfer to *hashdb.Database type")
	}
}

func PrefetchStorage() {
	//TODO
	//实现 builder prefetch 模块
	//实现 选取策略算法
	fmt.Println("hello")
}

//目前看来builder进行打包时会优先选择snapshot进行SLOAD操作，所以其实并不用Trie的prefetch，而是snapshot的prefetch
//这里的方法都是针对Trie进行的prefetch
