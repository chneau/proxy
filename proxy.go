package proxy

import (
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/chneau/limiter"
	"github.com/chneau/proxy/pkg/list"
)

// Information ...
type Information struct {
	Transparent bool
	Works       int
	Fails       int
}

// Manager ...
type Manager struct {
	Proxies      []string
	LastFetch    time.Time
	ProxyRefresh time.Duration

	Fetchers       []func() ([]string, error)
	Timeout        time.Duration
	MaxConcurrency int
	Filter         float64
	URLTest        string

	GoodProxies  map[string]*http.Client
	ProxiesScore map[string]*Information
	Mtx          sync.Mutex
}

// GetProxies ...
func (m *Manager) GetProxies() ([]string, error) {
	if m.LastFetch.Add(m.ProxyRefresh).After(time.Now()) {
		return m.Proxies, nil
	}
	proxiesToTest := map[string]string{}
	mtx := sync.Mutex{}
	wg := sync.WaitGroup{}
	var err error
	wg.Add(len(m.Fetchers))
	for _, fetcher := range m.Fetchers {
		go func(fetcher func() ([]string, error)) {
			defer wg.Done()
			proxies, e := fetcher()
			if e != nil {
				err = e
				return
			}
			mtx.Lock()
			defer mtx.Unlock()
			for i := range proxies {
				proxiesToTest[proxies[i]] = proxies[i]
			}
		}(fetcher)
	}
	wg.Wait()
	if err != nil {
		return nil, err
	}
	urls := []string{}
	for _, proxy := range proxiesToTest {
		urls = append(urls, proxy)
		if _, exist := m.ProxiesScore[proxy]; !exist {
			m.ProxiesScore[proxy] = &Information{}
		}
	}
	m.LastFetch = time.Now()
	m.Proxies = urls
	return urls, nil
}

// GetIP ...
func GetIP(url string) net.IP {
	resp, err := http.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	return net.ParseIP(string(b))
}

// TestProxies will populate goodproxies with proxies passing the test
func (m *Manager) TestProxies() error {
	proxies, err := m.GetProxies()
	if err != nil {
		return err
	}
	ip := GetIP(m.URLTest)
	dial := (&net.Dialer{Timeout: m.Timeout, KeepAlive: m.Timeout}).Dial
	filter := time.Duration(float64(m.Timeout) * m.Filter)
	limit := limiter.New(m.MaxConcurrency)
	for i := range proxies {
		i := i
		if m.IsProxyBad(proxies[i]) {
			m.Mtx.Lock()
			delete(m.GoodProxies, proxies[i]) // without scoring
			m.Mtx.Unlock()
			continue
		}
		limit.Execute(func() {
			parsedURL, err := url.Parse(proxies[i])
			if err != nil {
				log.Println("Could not parse URL", proxies[i])
			}
			client := &http.Client{
				Timeout: m.Timeout,
				Transport: &http.Transport{
					DisableKeepAlives: true,
					Proxy:             http.ProxyURL(parsedURL),
					Dial:              dial,
				},
			}
			start := time.Now()
			resp, err := client.Get(m.URLTest)
			if err != nil {
				m.RemoveProxy(proxies[i])
				return
			}
			defer resp.Body.Close()
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				m.RemoveProxy(proxies[i])
				return
			}
			returnedIP := net.ParseIP(string(b))
			if returnedIP == nil {
				m.RemoveProxy(proxies[i])
				return
			}
			if ip.Equal(returnedIP) {
				m.AddBadProxy(proxies[i])
				m.RemoveProxy(proxies[i])
				return
			}
			duration := time.Since(start)
			if duration > filter {
				m.RemoveProxy(proxies[i])
				return
			}
			m.AddProxy(proxies[i], client)
		})
	}
	limit.Wait()
	return nil
}

// GetRandomClient ...
func (m *Manager) GetRandomClient() (*http.Client, string) {
	m.Mtx.Lock()
	defer m.Mtx.Unlock()
	l := len(m.GoodProxies)
	if l == 0 {
		return nil, ""
	}
	i := rand.Intn(l)
	for key := range m.GoodProxies {
		if i == 0 {
			return m.GoodProxies[key], key
		}
		i--
	}
	return nil, ""
}

// AddBadProxy adds a proxy to the map of bat proxies
func (m *Manager) AddBadProxy(key string) {
	m.Mtx.Lock()
	defer m.Mtx.Unlock()
	m.ProxiesScore[key].Transparent = true
}

// IsProxyBad ...
func (m *Manager) IsProxyBad(key string) bool {
	m.Mtx.Lock()
	defer m.Mtx.Unlock()
	v, exist := m.ProxiesScore[key]
	if !exist {
		return true
	}
	if v.Transparent {
		return true
	}
	if v.Works == 0 && v.Fails > 3 {
		return true
	}
	total := v.Fails + v.Works
	if total < 2 {
		return false
	}
	if rand.Intn(total+1) < v.Fails { // 1,0 == 50, 1,1 = 0.33 | 0,1 = 0
		return true
	}
	return false
}

// AddProxy adds a proxy to the map of good proxies
func (m *Manager) AddProxy(key string, client *http.Client) {
	m.Mtx.Lock()
	defer m.Mtx.Unlock()
	m.GoodProxies[key] = client
}

// RemoveProxy removes a proxy from the map of good proxies
func (m *Manager) RemoveProxy(key string) {
	m.Mtx.Lock()
	defer m.Mtx.Unlock()
	m.ProxiesScore[key].Fails = m.ProxiesScore[key].Fails + 1
	delete(m.GoodProxies, key)
}

// GratzProxy congratz a proxy for working
func (m *Manager) GratzProxy(key string) {
	m.Mtx.Lock()
	defer m.Mtx.Unlock()
	m.ProxiesScore[key].Works++
}

// NewDefaultManager ...
func NewDefaultManager() *Manager {
	manager := &Manager{
		Fetchers: []func() ([]string, error){
			list.ProxiesFromClarketm,
			list.ProxiesFromFate0,
			list.ProxiesFromSunny9577,
			list.ProxiesFromDailyFreeProxy,
			list.ProxiesFromDailyProxy,
			list.ProxiesFromSmallSeoTools,
		},
		Timeout:        time.Second * 3,
		ProxyRefresh:   time.Minute * 10,
		Filter:         0.6,
		MaxConcurrency: 500,
		URLTest:        "http://api.ipify.org/",
		GoodProxies:    map[string]*http.Client{},
		ProxiesScore:   map[string]*Information{},
	}
	return manager
}
