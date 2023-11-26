package main

import "sync"

var memmoryAccsess sync.Mutex

func increment() {
	go func()  {
		memmoryAccsess.Lock()
		data++
		memmoryAccsess.Unlock()
	}()
}
