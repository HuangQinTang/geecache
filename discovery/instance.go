package discovery

const (
	ClusterPrefix             = "/gee_cache/nodes/"                       //ectd中集群地址信息，/gee_cache/nodes/序号 => ip:port ，序号根据节点数量依次递增
	ConsistentHashReplicasNum = "/gee_cache/consistent_hash_replicas_num" //一致性哈希环一个节点的副本数
)
