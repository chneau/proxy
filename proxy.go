package proxy

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/chneau/proxy/pkg/proxylist"
)

type Manager struct {
	// testing
	proxiesTest   chan string
	TimeoutTest   time.Duration
	URLTest       string
	ip            net.IP
	proxiesTested map[string]int
	mtxTest       sync.Mutex

	// good to use
	mtxGood            sync.Mutex
	proxiesGood        map[string][]time.Time
	proxiesGoodStrikes map[string]int
	StrikeLimit        int
	Requests           int
	TimeWindow         time.Duration
	TimeoutGood        time.Duration
}

func (m *Manager) addTimeToGoodProxy(proxy string) {
	m.proxiesGood[proxy] = append(m.proxiesGood[proxy], time.Now())
	if len(m.proxiesGood[proxy]) > m.Requests {
		m.proxiesGood[proxy] = m.proxiesGood[proxy][1:]
	}
}

func (m *Manager) PunishProxy(proxy string) {
	m.mtxGood.Lock()
	defer m.mtxGood.Unlock()
	for i := 0; i < m.Requests; i++ {
		m.addTimeToGoodProxy(proxy)
	}
	m.proxiesGoodStrikes[proxy]++
	if m.proxiesGoodStrikes[proxy] >= m.StrikeLimit {
		delete(m.proxiesGoodStrikes, proxy)
		delete(m.proxiesGood, proxy)
		m.AddProxies(proxy)
	}
}

func (m *Manager) GetGoodProxy() (string, *http.Client) {
	m.mtxGood.Lock()
	defer m.mtxGood.Unlock()
	proxy := ""
	for k, v := range m.proxiesGood {
		if len(v) < m.Requests {
			proxy = k
			break
		}
		if time.Since(v[0]) > m.TimeWindow {
			proxy = k
			break
		}
	}
	if proxy == "" {
		return "", nil
	}
	m.addTimeToGoodProxy(proxy)
	return proxy, clientFromString(m.TimeoutGood, proxy)
}

func (m *Manager) addProxies(proxies ...string) {
	for i := range proxies {
		m.proxiesTest <- proxies[i]
	}
}

func (m *Manager) AddProxies(proxies ...string) {
	go m.addProxies(proxies...)
}

func (m *Manager) removeProxyGood(str string) {
	m.mtxGood.Lock()
	defer m.mtxGood.Unlock()
	delete(m.proxiesGood, str)
}

func (m *Manager) addProxyGood(str string) {
	m.mtxGood.Lock()
	defer m.mtxGood.Unlock()
	if _, exist := m.proxiesGood[str]; !exist {
		m.proxiesGood[str] = []time.Time{}
	}
}

func (m *Manager) modifyTest(str string, delta int) {
	m.mtxTest.Lock()
	defer m.mtxTest.Unlock()
	m.proxiesTested[str] = m.proxiesTested[str] + delta
}

func (m *Manager) readTest(str string) int {
	m.mtxTest.Lock()
	defer m.mtxTest.Unlock()
	return m.proxiesTested[str]
}

func (m *Manager) test(client *http.Client, p string) {
	if m.readTest(p) <= -10 {
		return
	}
	client.Transport.(*http.Transport).Proxy = http.ProxyURL(strToURL(p))
	resp, err := client.Get(m.URLTest)
	if err != nil {
		m.modifyTest(p, -1)
		m.AddProxies(p)
		return
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		m.modifyTest(p, -1)
		m.AddProxies(p)
		return
	}
	returnedIP := net.ParseIP(string(b))
	if returnedIP == nil {
		m.modifyTest(p, -1)
		m.AddProxies(p)
		return
	}
	if m.ip.Equal(returnedIP) {
		m.modifyTest(p, -10)
		return
	}
	m.modifyTest(p, 1)
	m.addProxyGood(p)
}

func (m *Manager) autoProxyTester() {
	client := clientFromString(m.TimeoutTest, "")
	for p := range m.proxiesTest {
		m.test(client, p)
	}
}

func (m *Manager) WithAutoProxyTester(concurrentTest int) *Manager {
	m.ip = getIP(m.URLTest)
	for i := 0; i < concurrentTest; i++ {
		go m.autoProxyTester()
	}
	return m
}

func (m *Manager) WithAutoRefresh(every time.Duration) *Manager {
	go func() {
		for {
			m.AddProxies(proxylist.ProxiesFromClarketm()...)
			m.AddProxies(proxylist.ProxiesFromDailyFreeProxy()...)
			m.AddProxies(proxylist.ProxiesFromDailyProxy()...)
			m.AddProxies(proxylist.ProxiesFromFate0()...)
			m.AddProxies(proxylist.ProxiesFromSmallSeoTools()...)
			m.AddProxies(proxylist.ProxiesFromSunny9577()...)
			time.Sleep(every)
		}
	}()
	return m
}

func NewDefaultManager() *Manager {
	m := &Manager{
		proxiesTest:   make(chan string, 1000),
		URLTest:       "http://api.ipify.org/",
		proxiesTested: map[string]int{},
		proxiesGood:   map[string][]time.Time{},
		StrikeLimit:   5,
		TimeoutTest:   time.Second * 3,
		TimeoutGood:   time.Second * 4,
		TimeWindow:    time.Second * 12,
		Requests:      16,
	}
	return m
}

func strToURL(proxy string) *url.URL {
	parsedURL, _ := url.Parse(proxy)
	return parsedURL
}

func getIP(urlIP string) net.IP {
	resp, err := http.Get(urlIP)
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

func clientFromString(timeout time.Duration, proxy string) *http.Client {
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DisableKeepAlives: true,
			Proxy:             http.ProxyURL(strToURL(proxy)),
			Dial:              (&net.Dialer{Timeout: timeout, KeepAlive: timeout}).Dial,
		},
	}
	return client
}
