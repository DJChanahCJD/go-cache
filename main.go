package main

import (
	"flag"
	"fmt"
	"geecache"
	"log"
	"net/http"
)

// 模拟数据库存储
// 使用map存储键值对,模拟数据库的数据存储
// key为学生姓名,value为分数
var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

// 创建缓存组实例
// 返回一个新的缓存组实例,包含:
// 1. 组名为"scores"
// 2. 缓存大小为2KB(2<<10)
// 3. 回调函数用于在缓存未命中时从数据库获取数据
func createGroup() *geecache.Group {
	return geecache.NewGroup("scores", 2<<10, geecache.GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			// 从db中查找数据
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			// 未找到数据则返回错误
			return nil, fmt.Errorf("%s not exist", key)
		}))
}

// 启动缓存服务器
// addr: 当前节点地址
// addrs: 所有节点地址
// gee: 缓存组实例
func startCacheServer(addr string, addrs []string, gee *geecache.Group) {
	// 创建HTTP服务器实例
	peers := geecache.NewHTTPPool(addr)
	// 注册所有节点信息
	peers.Set(addrs...)
	// 将节点注册到缓存组
	gee.RegisterPeers(peers)
	log.Println("geecache is running at", addr)
	// 启动HTTP服务器,去除地址中的"http://"前缀
	log.Fatal(http.ListenAndServe(addr[7:], peers))
}

// 启动API服务器
// apiAddr: API服务器地址
// gee: 缓存组实例
func startAPIServer(apiAddr string, gee *geecache.Group) {
	// 注册处理/api路由的处理函数
	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// 获取查询参数中的key
			key := r.URL.Query().Get("key")
			// 从缓存中获取数据
			view, err := gee.Get(key)
			if err != nil {
				// 发生错误时返回500状态码
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// 设置响应头为二进制流
			w.Header().Set("Content-Type", "application/octet-stream")
			// 写入响应数据
			w.Write(view.ByteSlice())
		}))
	log.Println("fontend server is running at", apiAddr)
	// 启动HTTP服务器,去除地址中的"http://"前缀
	log.Fatal(http.ListenAndServe(apiAddr[7:], nil))
}

// 主函数
func main() {
	// 解析命令行参数
	// port: 缓存服务器端口号,默认8001
	// api: 是否启动API服务器,默认false
	var port int
	var api bool
	flag.IntVar(&port, "port", 8001, "Geecache server port")
	flag.BoolVar(&api, "api", false, "Start a api server?")
	flag.Parse()

	// 设置服务器地址
	// apiAddr: API服务器地址
	// addrMap: 缓存节点地址映射表
	apiAddr := "http://localhost:9999"
	addrMap := map[int]string{
		8001: "http://localhost:8001",
		8002: "http://localhost:8002",
		8003: "http://localhost:8003",
	}

	// 收集所有节点地址
	var addrs []string
	for _, v := range addrMap {
		addrs = append(addrs, v)
	}

	// 启动服务
	// 1. 创建缓存组实例
	gee := createGroup()
	// 2. 如果需要,启动API服务器(使用goroutine避免阻塞)
	if api {
		go startAPIServer(apiAddr, gee)
	}
	// 3. 启动缓存服务器
	startCacheServer(addrMap[port], []string(addrs), gee)
}