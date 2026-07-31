package session

import (
	"sync"
	"testing"
)

func TestConfigSwapUnderLoad(t *testing.T) {
	s := &Session{}
	c0 := &Configuration{}
	c0.Proxy.Phishing = "a.example"
	s.SwapConfig(c0)

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 8; i++ { // readers
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = s.Config().Proxy.Phishing
				}
			}
		}()
	}
	for i := 0; i < 100; i++ { // writers
		c := &Configuration{}
		c.Proxy.Phishing = "b.example"
		s.SwapConfig(c)
	}
	close(stop)
	wg.Wait()
	if s.Config().Proxy.Phishing == "" {
		t.Fatal("config lost after swaps")
	}
}
