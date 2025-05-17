package geecache

import pb "geecache/geecachepb"
// PeerPicker
// 节点选择接口
type PeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter
// 节点获取接口
type PeerGetter interface {
	Get(in *pb.Request, out *pb.Response) (error)
}