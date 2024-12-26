package metric

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// 实现GetSize接口的都可以被监听
type Storage interface {
	GetSize() map[string]common.StorageSize
}

type MemoryData struct {
	Name string //内存名称
	//Max_MB  uint64                   //最大容量MB
	Storage Storage                   //监控变量的指针
	Record  map[string][]MemoryRecord //一个内存监测对象里面的子对象，比如hashdb里面有cleans dirties等
}

type MemoryRecord struct {
	Current_Size common.StorageSize //当前的内存占用
	Timestamp    int64              //记录时刻时间戳(nano second)
}

type MemoryMonitor struct {
	MemoryMap map[string]MemoryData //监视器监视的内存对象
	is_stop   bool                  //终止信号
}

// 新建内存监控器对象
func NewMemoryMonitor() *MemoryMonitor {
	return &MemoryMonitor{
		MemoryMap: make(map[string]MemoryData),
		is_stop:   true,
	}
}

// 注册要监控的内存Storage接口
func (mm *MemoryMonitor) MemoryRegister(name string /*limit uint64,*/, storage Storage) {
	if _, ok := mm.MemoryMap[name]; ok {
		panic("The registered monitoring object already exists")
	}
	mm.MemoryMap[name] = MemoryData{
		Name: name,
		//Max_MB:  limit,
		Storage: storage,
		Record:  make(map[string][]MemoryRecord),
	}
}

// 记录一次数据
func (mm *MemoryMonitor) Tick() {
	//遍历监视器监控的所有Memory
	for _, memory := range mm.MemoryMap {
		//调用存储对象实体获取子存储实体的空间占用
		size_map := memory.Storage.GetSize() //调用被监控的Memory的GetSize()函数
		timestamp := time.Now().UnixNano()   //获取时间戳
		for label, size := range size_map {  //遍历存储子实体
			//添加记录条目
			memory.Record[label] = append(memory.Record[label], MemoryRecord{
				Current_Size: size,      //调用被监控的Memory的Size()函数
				Timestamp:    timestamp, //记录时间
			})
			//fmt.Println(label, size)
		}
	}
}

// 开始监控
func (mm *MemoryMonitor) Start(gap int64) {
	mm.is_stop = false
	go func() {
		//没有收到终止信号就一直循环
		for !mm.is_stop {
			mm.Tick()
			time.Sleep(time.Duration(gap) * time.Nanosecond)
		}
	}()
}

// 停止监控
func (mm *MemoryMonitor) Stop() {
	mm.is_stop = true
}

// 获取指定存储实体的子实体的记录数据（如HashDB获取cleansSize）
func (mm *MemoryMonitor) GetLastRecord(memory_name string, sub_memory_name string) MemoryRecord {
	length := len(mm.MemoryMap[memory_name].Record[sub_memory_name])
	if length == 0 {
		return MemoryRecord{}
	}
	return mm.MemoryMap[memory_name].Record[sub_memory_name][length-1]
}

/*
获取内存占用量的工具
需要实现对应内存对象如hashdb等的GetSize()方法
会新开一个线程并行监听
TODO:目前仅仅实现对于hashdb的监听，而且方法也不知道对不对，主要是没弄懂fastcache的原理，会自动FIFO维护还是怎么样
TODO:目前所有监测对象都写在内存中，时间长可能内存溢出，后期考虑输出到文件或者封装成接口输出
*/
