package lru

import "container/list"

// Cache LRU缓存，非并发安全实现
// LRU 缓存淘汰策略，最近最少使用。LRU 认为，如果数据最近被访问过，那么将来被访问的概率也会更高。
// 维护一个队列，如果某条记录被访问了，则移动到队尾，那么队首则是最近最少访问的数据，淘汰该条记录即可。
// 队首队尾是相对的，ll是双向链表，这里我们规定队列ll的队尾存放最少访问的数据，队首存放最频繁访问的数据
type Cache struct {
	maxBytes  int64                         //允许使用的最大内存
	nbytes    int64                         //当前已使用的内存
	ll        *list.List                    //队列
	cache     map[string]*list.Element      //键是字符串，值是双向链表中对应节点的指针
	OnEvicted func(key string, value Value) //某条记录被移除时的回调函数，可以为 nil
}

// entry kv键值缓存
type entry struct {
	key   string
	value Value
}

// Value 返回值所占用的内存大小
type Value interface {
	Len() int
}

// New 构造Cache
func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: onEvicted,
	}
}

// Get 查找缓存中key对应的value值
func (c *Cache) Get(key string) (value Value, ok bool) {
	// 1.从字典中找到对应的双向链表的节点
	if ele, ok := c.cache[key]; ok {
		// 2.将该节点移动到队首。那么ll队尾为最少访问的节点
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		return kv.value, true
	}
	return
}

// Add 添加kv缓存
func (c *Cache) Add(key string, value Value) {
	if ele, ok := c.cache[key]; ok { //如果键存在，则更新对应节点的值，并将该节点移到队首
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else { //不存则新增，首先队首添加新节点, 并字典中添加 key 和节点的映射关系。
		ele = c.ll.PushFront(&entry{key, value})
		c.cache[key] = ele
		c.nbytes += int64(len(key)) + int64(value.Len())
	}
	//当前使用内存超出最大内存，惰性删除
	for c.maxBytes != 0 && c.maxBytes < c.nbytes {
		c.RemoveOldest()
	}
}

// RemoveOldest 缓存淘汰,移除最近最少访问的节点（ll队尾）
func (c *Cache) RemoveOldest() {
	// 1.获取队尾节点
	ele := c.ll.Back()
	if ele != nil {
		// 2.移除该节点
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		// 3.删除map中该节点的映射关系
		delete(c.cache, kv.key)
		// 4.重新计算Cache占用内存
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		// 5.执行回调事件
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

// Len 获取缓存中的键值数
func (c *Cache) Len() int {
	return c.ll.Len()
}
