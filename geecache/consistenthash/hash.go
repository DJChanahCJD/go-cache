package consistentHash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

// Hash maps bytes to uint32
type Hash func(data []byte) uint32

// Map constains all hashed keys
type Map struct {
	hash     Hash           // 哈希函数
	replicas int            // 虚拟节点倍数
	keys     []int          // 哈希环（排序后的）
	hashMap  map[int]string // 虚拟节点到真实节点的映射
}

// New creates a Map instance
func New(replicas int, fn Hash) *Map {
	m := &Map{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE	//	默认使用crc32算法
	}
	return m
}

// Add adds some keys to the hash.
func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		// 对每个真实节点，根据副本数创建多个虚拟节点
		for i := 0; i < m.replicas; i++ {
			// 通过添加编号生成虚拟节点的名称，并计算哈希值
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			// 将虚拟节点添加到哈希环
			m.keys = append(m.keys, hash)
			// 建立虚拟节点到真实节点的映射
			m.hashMap[hash] = key
		}
	}
	// 对哈希环上的节点进行排序
	sort.Ints(m.keys)
}

// Get gets the closest item in the hash to the provided key.
func (m *Map) Get(key string) string {
	// 如果哈希环为空，返回空字符串
	if len(m.keys) == 0 {
		return ""
	}

	// 计算key的哈希值
	hash := int(m.hash([]byte(key)))
	// Binary search for appropriate replica.
	// 顺时针二分查找，找到第一个匹配的虚拟节点的下标
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})

	// 通过虚拟节点找到真实节点
	return m.hashMap[m.keys[idx%len(m.keys)]]
}