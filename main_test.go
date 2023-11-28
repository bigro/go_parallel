package main

import (
	"bytes"
	"fmt"
	"runtime"
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

		time.Sleep(2 * time.Second)
		v2.mu.Lock()
		defer v2.mu.Unlock()

		fmt.Printf("sum=%v\n", v1.value+v2.value)
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
		for range time.Tick(1 * time.Microsecond) {
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

	walk := func(walking *sync.WaitGroup, name string) {
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
	const runtime = 1 * time.Second

	// 貪欲なワーカー
	// こまめにロックするよりまとめて3nsをロックすることによって他のワーカーの処理を妨げている
	greedyWorker := func() {
		defer wg.Done()

		var count int
		for begin := time.Now(); time.Since(begin) <= runtime; {
			sharedLock.Lock()
			time.Sleep(3 * time.Nanosecond)
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
			time.Sleep(1 * time.Nanosecond)
			sharedLock.Unlock()

			sharedLock.Lock()
			time.Sleep(1 * time.Nanosecond)
			sharedLock.Unlock()

			sharedLock.Lock()
			time.Sleep(1 * time.Nanosecond)
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

// ゴルーチンが終了する前に親ゴルーチンが終了してまうパターン
func Test_HelloGoroutine(t *testing.T) {
	seyHello := func() {
		time.Sleep(500 * time.Millisecond)
		fmt.Println("hello")
	}

	go seyHello()
}

// 合流ポイントを作成してゴルーチンが終わるまで親ゴルーチンが終了しない
func Test_HelloGoroutine2(t *testing.T) {
	var wg sync.WaitGroup
	seyHello := func() {
		defer wg.Done()
		time.Sleep(500 * time.Millisecond)
		fmt.Println("hello")
	}

	wg.Add(1)
	go seyHello()
	wg.Wait()
}

// ゴルーチンが親プロセスと同じアドレス空間を利用してることを証明するロジック
// salutation変数の元の参照に対して代入されているので結果がwelcomeとなる
func Test_HelloGoroutine3(t *testing.T) {
	var wg sync.WaitGroup
	salutation := "hello"
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(500 * time.Millisecond)
		salutation = "welcome"
	}()
	wg.Wait()

	fmt.Println(salutation)
}

// ゴルーチンが開始する前にループが完了して「good by」が3回出力される
// 先にループが終了してもsalutationがスコープ外にならないのは、Goのランタイムがヒープにメモリを移してるから
func Test_HelloGoroutine4(t *testing.T) {
	var wg sync.WaitGroup
	for _, salutation := range []string{"hello", "greetings", "good by"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fmt.Println(salutation)
		}()
	}
	wg.Wait()
}

// salutationのコピーをクロージャに渡すことで意図通りの挙動になる
func Test_HelloGoroutine5(t *testing.T) {
	var wg sync.WaitGroup
	for _, salutation := range []string{"hello", "greetings", "good by"} {
		wg.Add(1)
		go func(salutation string) {
			defer wg.Done()
			fmt.Println(salutation)
		}(salutation)
	}
	wg.Wait()
}

// ゴルーチンで使用されるメモリを計測
func Test_HelloGoroutine6(t *testing.T) {
	// メモリの計測
	memConsumed := func() uint64 {
		runtime.GC()
		var s runtime.MemStats
		runtime.ReadMemStats(&s)
		return s.Sys
	}

	var c <-chan interface{}
	var wg sync.WaitGroup
	// 終わらないクロージャ
	noop := func() { wg.Done(); <-c }

	// 10の4乗分のゴルーチンを立ち上げる
	const numGoroutines = 1e4
	wg.Add(numGoroutines)
	before := memConsumed()
	for i := numGoroutines; i > 0; i-- {
		go noop()
	}
	wg.Wait()
	after := memConsumed()
	fmt.Printf("%.3fkb", float64(after-before)/numGoroutines/1000)
}

// ゴルーチンのコンテキストスイッチにかかるコストを計測
func BenchmarkContextSwitch(b *testing.B) {
	var wg sync.WaitGroup
	begin := make(chan struct{})
	c := make(chan struct{})

	var token struct{}
	sender := func() {
		defer wg.Done()
		// ゴルーチン生成を計測に含まないようにここから計測
		<-begin
		for i := 0; i < b.N; i++ {
			c <- token
		}
	}
	receiver := func() {
		defer wg.Done()
		<-begin
		for i := 0; i < b.N; i++ {
			<-c
		}
	}

	wg.Add(2)
	// ゴルーチンを生成
	go sender()
	go receiver()
	// タイマーを開始
	b.StartTimer()
	// コンテキストスイッチが発生する処理を開始
	close(begin)
	wg.Wait()
}

func Test_WaitGroup(t *testing.T) {
	var wg sync.WaitGroup

	// WaitGroupに渡された整数分、DoneされるのをWaitメソッドで待つ
	// ゴルーチン内でAddメソッドを呼ぶとWaitまで辿り着くまでにゴルーチンが起動されないことがあるので、必ずゴルーチン外でAddする
	wg.Add(1)
	go func() {
		// メソッドが終了したらDoneする
		defer wg.Done()
		fmt.Println("1st goroutine sleeping...")
		time.Sleep(1)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("2nd goroutine sleeping...")
		time.Sleep(2)
	}()

	// Addで追加された整数分、Doneされるまでブロックする
	wg.Wait()
	fmt.Println("All goroutines complete")
}

func Test_WaitGroupLoop(t *testing.T) {
	hello := func(wg *sync.WaitGroup, id int)  {
		defer wg.Done()
		fmt.Printf("Hello from %v!\n", id)
	}

	const numGreeters = 5
	var wg sync.WaitGroup
	wg.Add(numGreeters)
	for i := 0; i < numGreeters; i++ {
		go hello(&wg, i+1)
	}
	wg.Wait()
}