package main

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type value struct {
	mu    sync.Mutex
	value int
}

// デッドロックを引き起こす並行処理
func Test_Deadlock(t *testing.T) {
	var wg sync.WaitGroup
	printSum := func(v1, v2 *value) {
		defer wg.Done()
		v1.mu.Lock()
		defer v1.mu.Unlock()

		time.Sleep(2*time.Second)
		v2.mu.Lock()
		defer v2.mu.Unlock()

		fmt.Printf("sum=%v\n", v1.value + v2.value)
	}

	var a, b value
	wg.Add(2)
	go printSum(&a, &b)
	go printSum(&b, &a)
	wg.Wait()
}

// ライブロックを引き起こす並行処理
// 対面で歩いていてよける方向が同じでお見合いになる事象をシミュレート
func Test_Livelock(t *testing.T) {
	cadence := sync.NewCond(&sync.Mutex{})
	go func() {
		for range time.Tick(1*time.Microsecond) {
			cadence.Broadcast()
		}
	}()

	// 人間歩く歩調をシミュレート
	takeStep := func() {
		cadence.L.Lock()
		cadence.Wait()
		cadence.L.Unlock()
	}

	// 方向を指定して避けようとする
	tryDir := func(dirName string, dir *int32, out *bytes.Buffer) bool {
		fmt.Fprintf(out, " %v", dirName)
		atomic.AddInt32(dir, 1)
		takeStep()
		if atomic.LoadInt32(dir) == 1 {
			fmt.Fprint(out, ". Sucess!")
			return true
		}
		takeStep()
		// その方向に避けようとしてる人が他にもいれば歩くのを諦める
		atomic.AddInt32(dir, -1)
		return false
	}

	var left, right int32
	// 左によける
	tryLeft := func(out *bytes.Buffer) bool { return tryDir("left", &left, out) }
	// 右によける
	tryRight := func(out *bytes.Buffer) bool { return tryDir("right", &right, out) }

	walk := func(walking * sync.WaitGroup, name string) {
		var out bytes.Buffer
		defer func() { fmt.Println(out.String()) }()
		defer walking.Done()
		fmt.Fprintf(&out, "%v is trying to scoot:", name)
		for i := 0; i < 5; i++ {
			// まず左に避けようとして、失敗したら右に避けようとする
			if tryLeft(&out) || tryRight(&out) {
				return
			}
		}
		fmt.Fprintf(&out, "\n%v tosses her hands up in exasperation!", name)
	}

	var peopleInHallway sync.WaitGroup
	peopleInHallway.Add(2)
	go walk(&peopleInHallway, "Alice")
	go walk(&peopleInHallway, "Barbara")
	peopleInHallway.Wait()
}

// リソース枯渇
func Test_ResourceExhaustion(t *testing.T) {
	var wg sync.WaitGroup
	var sharedLock sync.Mutex
	const runtime = 1*time.Second

	// 貪欲なワーカー
	// こまめにロックするよりまとめて3nsをロックすることによって他のワーカーの処理を妨げている
	greedyWorker := func() {
		defer wg.Done()

		var count int
		for begin := time.Now(); time.Since(begin) <= runtime; {
			sharedLock.Lock()
			time.Sleep(3*time.Nanosecond)
			sharedLock.Unlock()
			count++
		}

		fmt.Printf("Greedy worker was able to execute %v work loops\n", count)
	}

	// 行儀が良いワーカー
	// 必要な時に1ns毎でロックすることによって他のワーカーの処理を妨げない
	// 貪欲なワーカーのせいで処理能力が落ちている
	politeWorker := func() {
		defer wg.Done()

		var count int
		for begin := time.Now(); time.Since(begin) <= runtime; {
			sharedLock.Lock()
			time.Sleep(1*time.Nanosecond)
			sharedLock.Unlock()

			sharedLock.Lock()
			time.Sleep(1*time.Nanosecond)
			sharedLock.Unlock()

			sharedLock.Lock()
			time.Sleep(1*time.Nanosecond)
			sharedLock.Unlock()

			count++
		}

		fmt.Printf("Polite worker was able to execute %v work loops.\n", count)
	}

	wg.Add(2)
	go greedyWorker()
	go politeWorker()

	wg.Wait()
}