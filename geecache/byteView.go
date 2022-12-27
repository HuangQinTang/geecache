package geecache

// ByteView 不可变字节视图，缓存值。选择 byte 类型是为了能够支持任意的数据类型的存储，例如字符串、图片等
type ByteView struct {
	b []byte //存储真实的缓存值，只读，可通过ByteSlice()返回该值拷贝，防止缓存值被外部程序修改
}

// Len 返回占用的内存大小。实现 lru.Value 接口。
func (v ByteView) Len() int {
	return len(v.b)
}

// ByteSlice 返回ByteView.b的拷贝，与ByteView.b不存在引用关系
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

// String 返回ByteView.b转换后的字符串
func (v ByteView) String() string {
	return string(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
