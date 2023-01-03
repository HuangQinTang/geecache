### 简单的分布式kv内存数据库

根据该教程实现，[极客兔兔-用Go从零实现分布式缓存GeeCache](https://geektutu.com/post/geecache.html)，在此基础上引入etcd实现服务发现。

#### 实现功能

- 可以为key定义数据源，然后提供HTTP Api 获取key对应的value值
- 同个key没有缓存时，窜行获取数据源，防止同时缓存穿透
- 一致性hash算法确保同个key访问到同个节点
- lru缓存淘汰
- 节点间http通讯，数据格式为 protobuf

#### 配置

- 需要有etcd服务，默认连接 http://127.0.0.1:2379
- main.go文件ip变量值为当前机器ip，用于服务注册，节点间应可以互相访问该ip
- etcd中可配置key `/gee_cache/consistent_hash_replicas_num`，值为哈希环副本数，值越大key分布相对均匀，默认500

#### 启动

```shell
# window
# 根目录
go build -o server.exe
# -port 缓存服务端口 -api http服务端口 -etcd etcd地址，默认 http://127.0.0.1:2379
./server.exe -port=8001
./server.exe -port=8002
./server.exe -port=8003 -api=9999 

# 测试
curl "http://localhost:9999/api?key=Tom"

# 同时etcd中会新增一个前缀为 /gee_cache/nodes/ 的key
etcdctl get --prefix /gee_cache/nodes/
```

#### 目录结构
- /consistenthash 一致性hash算法实现，用于让同个key命中同一节点
- /discovery 服务发现实现逻辑，将节点地址注册进etcd，并获取集群其他节点信息
- /geecache 缓存管理对象，每个节点的http客户端管理对象
- /singleflight 防止同时缓存穿透