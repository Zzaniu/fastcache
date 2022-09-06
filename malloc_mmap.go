//go:build !appengine && !windows
// +build !appengine,!windows

package fastcache

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

const chunksPerAlloc = 1024

var (
	freeChunks     []*[chunkSize]byte
	freeChunksLock sync.Mutex
)

// 申请内存
func getChunk() []byte {
	freeChunksLock.Lock()
	// 应该是初始化的时候, 去申请堆外内存
	if len(freeChunks) == 0 {
		// Allocate offheap memory, so GOGC won't take into account cache size.
		// This should reduce free memory waste.
		// Linux 申请堆外内存, 返回 []byte, error
		// 好像没有给出释放的函数, 所以这块内存会一直存在, 直到程序停止运行
		data, err := unix.Mmap(-1, 0, chunkSize*chunksPerAlloc, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_ANON|unix.MAP_PRIVATE)
		if err != nil {
			panic(fmt.Errorf("cannot allocate %d bytes via mmap: %s", chunkSize*chunksPerAlloc, err))
		}
		for len(data) > 0 {
			// 将申请的内存 []byte 转换为 [chunkSize]byte 数组, 然后取到首地址, 加入 freeChunks
			p := (*[chunkSize]byte)(unsafe.Pointer(&data[0]))
			freeChunks = append(freeChunks, p)
			// 把 data 置空, 不让 data 再指向那一块内存
			data = data[chunkSize:]
		}
	}
	n := len(freeChunks) - 1
	// p 为 *[chunkSize]byte
	p := freeChunks[n]
	// 释放内存
	freeChunks[n] = nil
	// 把分配的截取掉
	freeChunks = freeChunks[:n]
	freeChunksLock.Unlock()
	// 返回为一个切片
	return p[:]
}

// putChunk 归还内存
func putChunk(chunk []byte) {
	if chunk == nil {
		return
	}
	// 这里截取一下, 因为外面还要把引用置为 nil, 只有这样, 外面的才能和这块内存失去链接
	chunk = chunk[:chunkSize]
	// 将这个 []byte 转换为 [chunkSize]byte
	p := (*[chunkSize]byte)(unsafe.Pointer(&chunk[0]))

	freeChunksLock.Lock()
	// 归还到 freeChunks
	freeChunks = append(freeChunks, p)
	freeChunksLock.Unlock()
}
