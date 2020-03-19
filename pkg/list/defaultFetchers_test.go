package list

import (
	"sync"
	"testing"
)

func TestAll(t *testing.T) {
	proxies := map[string]func() []string{
		"ProxiesFromClarketm":       ProxiesFromClarketm,
		"ProxiesFromDailyFreeProxy": ProxiesFromDailyFreeProxy,
		"ProxiesFromDailyProxy":     ProxiesFromDailyProxy,
		"ProxiesFromFate0":          ProxiesFromFate0,
		"ProxiesFromSmallSeoTools":  ProxiesFromSmallSeoTools,
		"ProxiesFromSunny9577":      ProxiesFromSunny9577,
	}
	wg := sync.WaitGroup{}
	wg.Add(len(proxies))
	for name := range proxies {
		name := name
		go func() {
			defer wg.Done()
			prx := proxies[name]
			found := prx()
			if len(found) == 0 {
				t.Error(name, len(found))
			}
		}()
	}
	wg.Wait()
}
