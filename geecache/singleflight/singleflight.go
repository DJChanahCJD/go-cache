package singleflight

import "sync"

// call 表示一个正在进行中或已结束的请求
type call struct {
    wg  sync.WaitGroup    // 用于防止重入的锁
    val interface{}       // 请求结果值
    err error            // 请求过程中的错误
}

// Group 管理一组相同 key 的请求
type Group struct {
    mu sync.Mutex       // 保护 m 不被并发读写
    m  map[string]*call // key 到相应请求的映射
}

// Do 执行一个函数，确保同一时间对同一个 key 只会执行一次
// 参数:
//   - key: 请求的键
//   - fn: 要执行的函数
// 返回:
//   - interface{}: 函数执行结果
//   - error: 可能的错误
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
    g.mu.Lock()
    // 延迟初始化
    if g.m == nil {
        g.m = make(map[string]*call)
    }
    // 如果请求已经在进行中，等待请求完成
    if c, ok := g.m[key]; ok {
        g.mu.Unlock()
        c.wg.Wait()         // 等待请求完成
        return c.val, c.err // 返回请求结果
    }
    // 创建新的请求
    c := new(call)
    c.wg.Add(1)            // 发起请求前加锁
    g.m[key] = c           		// 记录请求
    g.mu.Unlock()

    // 执行请求
    c.val, c.err = fn()
    c.wg.Done()            // 请求完成，释放锁

    // 删除已完成的请求
    g.mu.Lock()
    delete(g.m, key)
    g.mu.Unlock()

    return c.val, c.err
}