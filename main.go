package main

import (
	"flag"
	"fmt"
	"geecache/discovery"
	"geecache/geecache"
	"log"
	"net/http"
	"os"
)

// 本机可用于与其他节点互通的ip
var ip = "127.0.0.1"

// 根目录 go build -o server.exe
// ./server.exe -port=8001
// ./server.exe -port=8002
// ./server.exe -port=8003 -api=9999
// curl "http://localhost:9999/api?key=Tom"
func main() {
	var port string     //geecache 服务端口
	var api string      //geecache http服务端口
	var etcdAddr string //etcd地址
	flag.StringVar(&port, "port", "", "Geecache server port")
	flag.StringVar(&api, "api", "", "http api port")
	flag.StringVar(&etcdAddr, "etcd", "http://127.0.0.1:2379", "etcd addr eg: http://127.0.0.1:2379")
	flag.Parse()
	//port = "8888"
	//api = "9999"

	if port == "" {
		fmt.Println("-port，输入服务端口")
		os.Exit(-1)
	}
	addr := ip + ":" + port

	// 初始化etcd客户端
	err := discovery.InitEtcdService([]string{etcdAddr}, 3)
	if err != nil {
		log.Fatal(err.Error())
	}

	// 服务注册
	register := discovery.NewRegister(addr)
	if err = register.Register(3); err != nil {
		log.Fatal(err.Error())
	}

	// 通过etcd获取集群中其他节点信息，为每个节点创建http客户端 存放在 HTTPPool
	peers := geecache.NewHTTPPool(addr, register)
	if err = peers.Work(); err != nil {
		log.Fatal(err.Error())
	}

	// 创建命名空间，以及为该命名空间准备数据源
	gee := geecache.NewGroup("scores", 2<<10, scoresDb())
	gee.RegisterPeers(peers) //当key对应的缓存不在本地节点，通过 peers(httpPool) 计算key拿到对应的http客户端请求远程节点缓存

	// 启动http服务
	if api != "" {
		go startAPIServer(api, gee)
	}

	// 启动缓存服务
	log.Println("Geecache server is running at port:", port)
	log.Fatal(http.ListenAndServe(":"+port, peers))
}

// startAPIServer 用来启动一个 API 服务
func startAPIServer(port string, gee *geecache.Group) {
	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			key := r.URL.Query().Get("key")
			view, err := gee.Get(key)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(view.ByteSlice())

		}))
	log.Println("fontend server is running at port:", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// 定义获取数据源的回调，这里写死从变量db获取
func scoresDb() geecache.GetterFunc {
	// map的kv 对应缓存key和value
	db := map[string]string{
		"Tom":  "630",
		"Jack": "589",
		"Sam":  "567",
		"Tang": "999",
		"Lbj":  "23",
		"Liu":  "55",
	}
	return func(key string) ([]byte, error) {
		log.Println("[SlowDB] search key", key)
		if v, ok := db[key]; ok {
			return []byte(v), nil
		}
		return nil, fmt.Errorf("%s not exist", key)
	}
}
