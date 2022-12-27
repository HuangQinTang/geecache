package geecache

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

const defaultBasePath = "/_geecache/"

// HTTPPool 承载节点间 HTTP 通信
type HTTPPool struct {
	self     string //地址，IP+端口
	basePath string //节点间通讯地址的前缀，默认是 /_geecache/，因为一个主机上还可能承载其他的服务，加一段 Path 是一个好习惯
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

// Log 打印日志
func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

// ServeHTTP 实现 http.Handler
// 我们约定访问路径格式为 /<basepath>/<groupname>/<key>，通过 groupname 得到 group 实例，再使用 group.Get(key) 获取缓存数据。
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		//panic("HTTPPool serving unexpected path: " + r.URL.Path)
		http.Error(w, "HTTPPool serving unexpected path: "+r.URL.Path, http.StatusNotFound)
		return
	}
	p.Log("%s %s", r.Method, r.URL.Path)
	// /<basepath>/<groupname>/<key> required
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2) //只获取groupname和key部分
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	groupName := parts[0]
	key := parts[1]

	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}

	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream") //字节流
	w.Write(view.ByteSlice())
}
