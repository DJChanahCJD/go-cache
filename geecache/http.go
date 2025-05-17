package geecache

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"geecache/consistentHash"
	pb "geecache/geecachepb"

	"google.golang.org/protobuf/proto"
)

const (
	defaultBasePath = "/_geecache/"
	defaultReplicas = 50			//	虚拟节点倍数
)

type HTTPPool struct {
	self     string 					//	记录自己的地址，包括主机名/IP和端口
	basePath string 					//	节点间通讯地址的前缀，默认是 /_geecache/
	mu       sync.Mutex
	peers    *consistentHash.Map 		//	一致性哈希算法 Map，通过 key 选择节点
	httpGetters map[string]*httpGetter 	//	根据具体的key，创建httpGetter，获取远程节点
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

// Log info with server name
func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

// ServeHTTP handle all http requests
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 判断访问路径的前缀是否是 basePath，不是返回错误
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		log.Fatal("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	p.Log("%s %s", r.Method, r.URL.Path) //	记录请求方法和路径

	// /<basepath>/<groupname>/<key> required
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2) //	将请求路径/groupname/key 分割为两部分
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	groupName := parts[0] //	groupname为第一部分
	key := parts[1]       //	key为第二部分

	group := GetGroup(groupName) //  通过groupname得到group实例
	if group == nil {
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}

	// 获取缓存值
	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()}) //  将value序列化为protobuf格式
	if err!= nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}


	w.Header().Set("Content-Type", "application/octet-stream") //	返回原始二进制流
	w.Write(body)
}

// Set updates the pool's list of peers.
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()                                           // 加锁保护并发访问
	defer p.mu.Unlock()                                   // 函数结束时解锁
	p.peers = consistentHash.New(defaultReplicas, nil)    // 创建一致性哈希算法的Map实例
	p.peers.Add(peers...)                                 // 添加节点到一致性哈希环上
	p.httpGetters = make(map[string]*httpGetter, len(peers))  // 为每个节点创建一个HTTP客户端
	for _, peer := range peers {
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath}  // 为每个节点创建一个HTTP客户端，并设置baseURL（节点地址+路径）
	}
}

// PickPeer picks a peer according to key
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// 根据key选择节点，如果节点存在且不是自己，则返回对应的HTTP客户端
	if peer := p.peers.Get(key); peer!= "" && peer != p.self {
		p.Log("Pick peer %s", peer)
		return p.httpGetters[peer], true
	}
	// 如果节点不存在或者是自己，则返回nil和false
	return nil, false
}

var _ PeerPicker = (*HTTPPool)(nil)	//	确保HTTPPool实现了PeerPicker接口

// 客户端代码
// 实现PeerGetter接口
type httpGetter struct {
	baseURL string
}

// Get 从远程节点获取缓存值
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
    defer res.Body.Close()    //  关闭响应体

    if res.StatusCode != http.StatusOK {
        return fmt.Errorf("server returned: %v", res.Status)
    }

    bytes, err := io.ReadAll(res.Body)    //  读取响应体
    if err != nil {
        return fmt.Errorf("reading response body: %v", err)
    }
    if err = proto.Unmarshal(bytes, out); err != nil {
        return fmt.Errorf("decoding response body: %v", err)
    }

    return nil
}

var _ PeerGetter = (*httpGetter)(nil)	//	确保httpGetter实现了PeerGetter接口

