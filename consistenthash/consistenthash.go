package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

// Hash hash函数，[]byte转uint32
type Hash func(data []byte) uint32

// Map 一致性哈希算法主数据结构
type Map struct {
	hash     Hash           //Hash 函数，默认为 crc32.ChecksumIEEE 算法
	replicas int            //虚拟节点倍数
	keys     []int          // 哈希环
	hashMap  map[int]string //虚拟节点与真实节点的映射表,键是虚拟节点的哈希值，值是真实节点的名称
}

// New 构造 Map 允许传入虚拟节点倍数及自定义的哈希函数
func New(replicas int, fn Hash) *Map {
	m := &Map{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil { //默认 crc32.ChecksumIEEE 算法
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// Add 添加真实节点/机器
func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		//创建 m.replicas 个虚拟节点
		for i := 0; i < m.replicas; i++ {
			//虚拟节点的名称是：strconv.Itoa(i) + key，即通过添加编号的方式区分不同虚拟节点。
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			//添加到环上
			m.keys = append(m.keys, hash)
			//在 hashMap 中增加虚拟节点和真实节点的映射关系。
			m.hashMap[hash] = key
		}
	}
	//环上的哈希值排序
	sort.Ints(m.keys)
}

// Get 选择节点, 环上与key hash后最相近的节点
func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}

	// 计算 key 的哈希值
	hash := int(m.hash([]byte(key)))
	// 顺时针找到第一个匹配的虚拟节点的下标,找不到将存储在 m.keys 最后一个元素
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	// 返回虚拟节点在hashMap映射到的真实节点，因为是环，所以要取余
	return m.hashMap[m.keys[idx%len(m.keys)]]
}
