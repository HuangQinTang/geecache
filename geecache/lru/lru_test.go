package lru

import (
	"fmt"
	"reflect"
	"testing"
)

type String string

func (d String) Len() int {
	return len(d)
}

// 测试 Get 方法
func TestGet(t *testing.T) {
	lru := New(int64(0), nil)
	lru.Add("key1", String("1234"))
	if v, ok := lru.Get("key1"); !ok || string(v.(String)) != "1234" {
		t.Fatalf("cache hit key1=1234 failed")
	}
	if _, ok := lru.Get("key2"); ok {
		t.Fatalf("cache miss key2 failed")
	}
}

// 测试当使用内存超过了设定值时，是否会触发“无用”节点的移除：
func TestRemoveoldest(t *testing.T) {
	k1, k2, k3 := "key1", "key2", "k3"
	v1, v2, v3 := "value1", "value2", "v3"
	//计算key1和key2占用的内存
	memory := len(k1 + k2 + v1 + v2)
	//设置内存上限为key1和key2之和
	lru := New(int64(memory), nil)
	lru.Add(k1, String(v1))
	lru.Add(k2, String(v2))
	//当写入key3，内存不够，预期key1应该被淘汰
	lru.Add(k3, String(v3))

	if _, ok := lru.Get("key1"); ok || lru.Len() != 2 { //判断缓存中是否只剩2个键值，是成功淘汰
		t.Fatalf("Removeoldest key1 failed")
	}
}

// 测试回调函数能否被调用
func TestOnEvicted(t *testing.T) {
	keys := make([]string, 0)
	//淘汰元素时执行，将被淘汰的元素存放进keys
	callback := func(key string, value Value) {
		keys = append(keys, key)
	}
	lru := New(int64(10), callback) //int64(10)，刚好可以存放k3+k4，key1和k2预期应该被淘汰
	lru.Add("key1", String("123456"))
	lru.Add("k2", String("k2"))
	lru.Add("k3", String("k3"))
	lru.Add("k4", String("k4"))

	expect := []string{"key1", "k2"} //预期应该淘汰的key

	if !reflect.DeepEqual(expect, keys) { //判断淘汰的key是否与预期的一致
		t.Fatalf("Call OnEvicted failed, expect keys equals to %s", expect)
	}
}

func TestHaha(t *testing.T) {
	haha := []byte{1, 2, 3}
	fmt.Println(haha)
	c := cloneBytes(haha)
	fmt.Println(c)
	c[0] = 4
	fmt.Println(haha)
	fmt.Println(c)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
