package list

import (
	"net/url"
	"sync"
	"testing"
)

func TestAll(t *testing.T) {
	proxies := map[string]func() ([]*url.URL, error){
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
			found, err := prx()
			if len(found) == 0 || err != nil {
				t.Error(name, len(found), err)
			}
		}()
	}
	wg.Wait()
}
