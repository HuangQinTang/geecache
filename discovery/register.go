package discovery

import (
	"context"
	"errors"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"strconv"
	"strings"
)

type Register struct {
	Addr    string             //当前节点ip+端口
	CurKey  string             //当前节点在etcd中的key
	cancel  context.CancelFunc //关闭注册(续约)用到的chan
	leaseId clientv3.LeaseID   //租约id
}

func NewRegister(addr string) *Register {
	return &Register{Addr: addr}
}

// Register 在etcd中注册服务, ttl单位秒，服务租约时间，该方法会维护租约时效
func (r *Register) Register(ttl int64) error {
	// 创建租约
	leaseResp, err := EtcdService.cli.Grant(context.TODO(), ttl)
	if err != nil {
		return err
	}
	r.leaseId = leaseResp.ID

	// 刷新租约，刷新间隔为 1/3 ttl时间
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	leaseInfoChan, err := EtcdService.cli.KeepAlive(ctx, r.leaseId) //该方法会创建一个g在租约过期前定期刷新
	if err != nil {
		return err
	}

	// leaseInfoChan租约续约成功后 clientv3会把响应信息塞入管道
	// 管道满后，clientv3会在控制台打印warm信息，太烦了，这里避免管道溢出，读取抛弃
	go func() {
		for {
			_ = <-leaseInfoChan
		}
	}()

	// 获取当前集群中的节点信息 将对应的etcd key推入nodeIndex
	nodes, err := r.GetNowNodes()
	if err != nil {
		return err
	}
	nodeIndex := make([]int, 0, len(nodes)+1) //集群已经存在的节点序号
	for _, etcdKey := range nodes {
		keySlice := strings.Split(etcdKey, "/")
		index, _ := strconv.Atoi(keySlice[3])
		nodeIndex = append(nodeIndex, index)
	}

	r.CurKey = "" //新加入的节点key
	// 节点在etcd中的key为 /gee_cache/序号 序号根据节点的增加依次递增
	// 如果当前节点数与最后一个节点的序号对不上，说明中间存在挂掉的节点
	// 如果存在挂掉的节点，那么新加入的节点补上挂点的节点序号，这样是为了一致性hash算法稳定
	if len(nodeIndex) > 0 && len(nodeIndex) != nodeIndex[len(nodeIndex)-1] {
		for i, j := 0, 1; i < len(nodeIndex); i, j = i+1, j+1 {
			if nodeIndex[i] != j { //序号不是顺序递增，新加入的节点用这个序号，填补顺序
				r.CurKey = ClusterPrefix + fmt.Sprintf("%d", j)
			}
		}
	}

	if r.CurKey == "" { //不存在挂掉的节点
		r.CurKey = ClusterPrefix + fmt.Sprintf("%d", len(nodeIndex)+1)
	}

	// 往etcd中注册新节点
	_, err = EtcdService.cli.Put(context.TODO(), r.CurKey, r.Addr, clientv3.WithLease(r.leaseId)) //写入 /gee_cache/递增数字 绑定租约

	fmt.Printf("key %s，value %s，租约%x\n", r.CurKey, r.Addr, r.leaseId)
	return nil
}

// GetNowNodes 获取集群注册成功的节点信息 map[节点地址]节点在etcd中的key
func (r *Register) GetNowNodes() (map[string]string, error) {
	// 获取etcd中当前存在的节点信息 get --prefix /gee_cache/nodes/
	clusterInfo, err := EtcdService.cli.Get(context.TODO(), ClusterPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	nodes := make(map[string]string, clusterInfo.Count)
	for _, kv := range clusterInfo.Kvs {
		// 校验格式
		keyStr := string(kv.Key)
		keySlice := strings.Split(keyStr, "/")
		if len(keySlice) != 4 {
			return nil, errors.New("etcd 配置出错, 出错的key为：" + keyStr + " 正确格式应为：" + ClusterPrefix + "数字")
		}

		_, err = strconv.Atoi(keySlice[3])
		if err != nil {
			return nil, errors.New("etcd 配置出错, 出错的key为：" + keyStr + " 正确格式应为：" + ClusterPrefix + "数字")
		}
		nodes[string(kv.Value)] = keyStr
	}
	return nodes, nil
}

// RemoveRegister 删除当前节点注册信息
func (r *Register) RemoveRegister() error {
	// 取消续约
	r.cancel()
	// 删除租约
	if _, err := EtcdService.cli.Revoke(context.TODO(), r.leaseId); err != nil {
		return err
	}
	// 删除etcd中的节点信息
	if _, err := EtcdService.cli.Delete(context.TODO(), r.CurKey); err != nil {
		return err
	}
	return nil
}
