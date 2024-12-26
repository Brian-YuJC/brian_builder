package metric

import (
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	//存放命中位置和字符串的映射
	ID_NAME_MAP = make([]string, 20)

	//命中位置编号
	STATEDB_DIRTY             = 0
	STATEDB_PENDING           = 1
	STATEDB_ORIGION           = 2
	STATEDB_DESTRUCT          = 3
	STATEDB_SNAPSHOT          = 4
	TRIE_VALUENODE            = 5
	TRIE_SHORTNODE            = 6
	TRIE_FULLNODE             = 7
	TRIE_HASHNODE             = 8
	HASHDB_CLEANS             = 9
	HASHDB_DIRTIES            = 10
	PEBBLE_DB                 = 11
	SNAPSHOT_DISKLAYER_CACHE  = 12
	SNAPSHOT_DISKLAYER_DISK   = 13
	SNAPSHOT_DIFFLAYER        = 14
	SNAPSHOT_PARENT_DIFFLAYER = 15
	SNAPSHOT_BLOOM_ERROR      = 16
	ERROR                     = 19

	//全局的命中检测器
	GLOBAL_HIT_MONITOR *HitMonitor
)

func init() {
	//初始化映射表
	ID_NAME_MAP[0] = "StateDB_Dirty"
	ID_NAME_MAP[1] = "StateDB_Pending"
	ID_NAME_MAP[2] = "StateDB_Origion"
	ID_NAME_MAP[3] = "StateDB_Destruct"
	ID_NAME_MAP[4] = "StateDB_Snapshot"
	ID_NAME_MAP[5] = "Trie_ValueNode"
	ID_NAME_MAP[6] = "Trie_ShortNode"
	ID_NAME_MAP[7] = "Trie_FullNode"
	ID_NAME_MAP[8] = "Trie_HashNode"
	ID_NAME_MAP[9] = "HashDB_Cleans"
	ID_NAME_MAP[10] = "HashDB_Dirties"
	ID_NAME_MAP[11] = "Pebble_DB"
	ID_NAME_MAP[12] = "Snapshot_DiskLayer_Cache"
	ID_NAME_MAP[13] = "Snapshot_DiskLayer_Disk"
	ID_NAME_MAP[14] = "Snapshot_DiffLayer"
	ID_NAME_MAP[15] = "Snapshot_Parent_DiffLayer"
	ID_NAME_MAP[16] = "Snapshot_DiffLayer(bloom filter error)" //布隆过滤器出错，提示在difflayer可以命中但是最后没有命中只能从disklayer读取
	ID_NAME_MAP[19] = "Sload_Error"

	//初始化全局命中率监控
	GLOBAL_HIT_MONITOR = NewHitMonitor()
}

// 命中记录的溯源记录
type TraceRecord struct {
	Type     int
	CostTime int64
}

// 命中记录
type HitRecord struct {
	StartTimestamp int64 //开始调用sload的时间
	TraceList      []TraceRecord
}

// 命中监视器
type HitMonitor struct {
	mu            sync.Mutex
	HitRecordList []*HitRecord
	is_stop       bool
}

// 新建命中率监视器
func NewHitMonitor() *HitMonitor {
	return &HitMonitor{
		HitRecordList: make([]*HitRecord, 0),
		is_stop:       true,
	}
}

// 新建一个HitRecord
func NewHitRecord() *HitRecord {
	return &HitRecord{
		StartTimestamp: time.Now().UnixNano(), //记录开始时间
	}
}

// 命中记录
func (hr *HitRecord) Hit(Type int) {
	hr.TraceList = append(hr.TraceList, TraceRecord{
		Type:     Type,
		CostTime: time.Now().UnixNano() - hr.StartTimestamp, //维护命中时间=当前时间-发起记录时间
	})
}

// 添加一条记录到Hit Monitor
func (hm *HitMonitor) Record(hr *HitRecord) {
	if hm.is_stop { //如果当前停止监视则直接返回不记录结果
		return
	}
	//加锁，因为builder并行运行会并发的写HitRecordList
	hm.mu.Lock()
	hm.HitRecordList = append(hm.HitRecordList, hr)
	hm.mu.Unlock()
}

// 打印输出结果
func (hm *HitMonitor) OutputRecord(output_path string) {
	//打开输出文件
	output, _ := os.Create(output_path)

	hm.mu.Lock()
	for _, hit_record := range hm.HitRecordList {
		for _, hit_trace := range hit_record.TraceList {
			fmt.Fprint(output, hit_trace.GetName(), ":", hit_trace.CostTime, " ") //往文件输出
		}
		fmt.Fprintln(output)
		//fmt.Println()
	}
	hm.mu.Unlock()
}

// 通过命中位置编号获取对应名称字符串
func (tr *TraceRecord) GetName() string {
	return ID_NAME_MAP[tr.Type]
}

// 开始监控
func (hm *HitMonitor) Start() {
	hm.is_stop = false
}

// 停止监控
func (hm *HitMonitor) Stop() {
	hm.is_stop = true
}

// 清空监视器
func (hm *HitMonitor) Clear() {
	hm.mu.Lock()
	hm.is_stop = true
	hm.HitRecordList = make([]*HitRecord, 0)
	hm.mu.Unlock()
}

//TODO 写注释OK，检查Hit位置对不对，有些打印出来是空值检查一下，检查一下多线程是否安全，
//TODO 有时间记录是负数？ 已经解决：解决方法time.Now().Nanosecond() 换成 time.Now().UnixNano()
