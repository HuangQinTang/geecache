package geecache

import pb "geecache/geecachepb"

// PeerPicker 根据传入的 key 选择相应节点
type PeerPicker interface {
	// PickPeer 根据具体的 key，选择节点，返回节点对应的 HTTP 客户端
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter 对应 PeerPicker 中的节点(http客户端), 从对应 Group 查找缓存值。
type PeerGetter interface {
	Get(in *pb.Request, out *pb.Response) error
}
