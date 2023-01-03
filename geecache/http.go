package geecache

import (
	"context"
	"errors"
	"fmt"
	"geecache/consistenthash"
	"geecache/discovery"
	pb "geecache/geecachepb"
	"google.golang.org/protobuf/proto"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

const (
	defaultBasePath = "/_geecache/"
)

var defaultReplicas = 50 //默认副本数

// HTTPPool 承载节点间 HTTP 通信的服务端
type HTTPPool struct {
	self        string                 // 地址，IP+端口
	basePath    string                 // 节点间通讯地址的前缀，默认是 /_geecache/，因为一个主机上还可能承载其他的服务，加一段 Path 是一个好习惯
	mu          sync.Mutex             // 保护 peers 和 httpGetters
	peers       *consistenthash.Map    // 类型是一致性哈希算法的 consistenthash.Map
	peersEtcd   map[string]string      // map[远程节点etcd key] 远程节点addr:ip
	httpGetters map[string]*httpGetter // 映射远程节点与对应的客户端 httpGetter ,每一个远程节点对应一个客户端
	register    *discovery.Register    // etcd注册服务
	replicas    int                    // 哈希环副本数
}

func NewHTTPPool(self string, register *discovery.Register) *HTTPPool {
	return &HTTPPool{
		self:        self,
		basePath:    defaultBasePath,
		register:    register,
		httpGetters: make(map[string]*httpGetter),
		peersEtcd:   make(map[string]string),
	}
}

// Work 从etcd中获取集群节点信息并监听集群变化 维护哈希环与该节点的http客户端
func (p *HTTPPool) Work() error {
	// 设置哈希环真实节点对应的副本数
	if err := p.setReplicas(); err != nil {
		return err
	}

	// 创建一致性哈希环
	p.peers = consistenthash.New(p.replicas, nil)

	// 从etcd中获取当前集群节点信息，添加进哈希环，并监听节点变动
	if err := p.addNowNodesToPeers(); err != nil {
		return err
	}

	// 监听整个集群变动，并根据节点注册信息维护 p.peers
	p.watchCluster()
	return nil
}

// WatchCluster 监听集群节点变化，并重新维护哈希环
func (p *HTTPPool) watchCluster() {
	discovery.EtcdService.WatchPrefix(context.Background(), discovery.ClusterPrefix, p.addBackFun(), p.delBackFun())
}

// initReplicas 设置哈希环真实节点对应的副本数
func (p *HTTPPool) setReplicas() error {
	p.replicas = defaultReplicas //默认副本数

	// 创建哈希环节点时，从etcd中获取真实节点的副本数
	replicasNum, err := discovery.EtcdService.GetKey(discovery.ConsistentHashReplicasNum)
	if err != nil {
		return errors.New("etcd 查询失败：" + err.Error())
	}
	replicasNumInt, err := strconv.Atoi(replicasNum)
	if err != nil {
		return errors.New("etcd " + discovery.ConsistentHashReplicasNum + "配置转换格式出错：" + err.Error())
	}
	if replicasNumInt > 0 {
		p.replicas = replicasNumInt
	}

	// 监听配置变化，并更具变化做出响应操作
	updateFun := func(ctx context.Context, keyInfo discovery.WatchInfo) { // 更新时
		replicas, putErr := strconv.Atoi(keyInfo.Value)
		if putErr != nil {
			fmt.Println("etcd " + discovery.ConsistentHashReplicasNum + "配置转换格式出错：" + err.Error())
			return
		}
		if replicas > 0 {
			p.replicas = replicas
		}
		p.peers.SetReplicas(p.replicas)
	}
	delFun := func(ctx context.Context, keyInfo discovery.WatchInfo) { // 删除时
		p.replicas = defaultReplicas
		p.peers.SetReplicas(p.replicas)
	}
	_ = discovery.EtcdService.WatchKey(context.Background(), discovery.ConsistentHashReplicasNum, updateFun, delFun)
	return nil
}

func (p *HTTPPool) addNowNodesToPeers() error {
	// 获取当前集群节点信息
	nodesInfo, err := p.register.GetNowNodes()
	if err != nil {
		return err
	}

	for addr, key := range nodesInfo {
		p.add(addr, key)
	}
	return nil
}

// Add 添加节点
func (p *HTTPPool) add(addr, etcdKey string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// hash环添加节点
	p.peers.Add(addr)
	// 建立节点与该节点客户端映射关系
	p.httpGetters[addr] = &httpGetter{baseURL: "http://" + addr + p.basePath}

	// 该key未监听过，创建监听
	if _, ok := p.peersEtcd[etcdKey]; !ok {
		p.peersEtcd[etcdKey] = addr
		// 开始监听
		discovery.EtcdService.WatchKey(context.Background(), etcdKey, p.addBackFun(), p.delBackFun())
	}
}

// addBackFun 有新节点加入集群时/或者原节点addr变动 执行的回调
func (p *HTTPPool) addBackFun() discovery.KeyEventFun {
	return func(ctx context.Context, keyInfo discovery.WatchInfo) { //keyInfo key为/gee_cache/序号 value为节点addr
		p.mu.Lock()
		defer p.mu.Unlock()
		etcdKey := keyInfo.Key
		addr := keyInfo.Value

		//删除旧节点信息
		oldAddr := p.peersEtcd[etcdKey]
		p.peers.Del(oldAddr)

		//添加新节点信息
		p.peers.Add(addr)
		p.httpGetters[addr] = &httpGetter{baseURL: "http://" + addr + p.basePath}
		p.peersEtcd[etcdKey] = addr
		fmt.Printf("集群新增节点 %s => %s\n", etcdKey, addr)
	}
}

// delBackFun 集群中有节点移除时 执行的回调
func (p *HTTPPool) delBackFun() discovery.KeyEventFun {
	return func(ctx context.Context, keyInfo discovery.WatchInfo) { //keyInfo key为/gee_cache/序号 value为""
		etcdKey := keyInfo.Key
		addr := p.peersEtcd[etcdKey]
		p.del(addr)
		fmt.Printf("集群移除节点 %s => %s\n", etcdKey, addr)
	}
}

// Del 删除节点
func (p *HTTPPool) del(addr string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.peers.Del(addr)
	delete(p.httpGetters, addr)
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

	// 使用Protobuf 序列化
	body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream") //字节流
	w.Write(body)
}

// PickPeer 根据具体的 key，选择节点，返回节点对应的 HTTP 客户端
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if peer := p.peers.Get(key); peer != "" && peer != p.self { //节点不能是自己
		p.Log("Pick peer %s", peer)
		return p.httpGetters[peer], true
	}
	return nil, false
}

var _ PeerPicker = (*HTTPPool)(nil)

// httpGetter 缓存服务http客户端，实现 PeerGetter 接口
type httpGetter struct {
	baseURL string //表示将要访问的远程节点的地址，例如 http://example.com/_geecache/
}

func (h *httpGetter) Get(in *pb.Request, out *pb.Response) error {
	u := fmt.Sprintf(
		"%v%v/%v",
		h.baseURL,
		url.QueryEscape(in.GetGroup()),
		url.QueryEscape(in.GetKey()),
	)
	res, err := http.Get(u)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: %v", res.Status)
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}

	if err = proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}
	return nil
}

// 判断 httpGetter 是否实现 PeerGetter 接口
var _ PeerGetter = (*httpGetter)(nil)
