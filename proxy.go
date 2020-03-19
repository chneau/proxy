package proxy

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/chneau/proxy/pkg/proxylist"
)

type Manager struct {
	// testing
	ProxiesTest   chan string
	TimeoutTest   time.Duration
	URLTest       string
	IP            net.IP
	ProxiesTested map[string]int
	MtxTest       sync.Mutex
	FuncTest      func(*http.Client) bool

	// good to use
	MtxGood            sync.Mutex
	ProxiesGood        map[string][]time.Time
	ProxiesGoodStrikes map[string]int
	StrikeLimit        int
	Requests           int
	TimeWindow         time.Duration
	TimeoutGood        time.Duration
}

func (m *Manager) addTimeToGoodProxy(proxy string) {
	m.ProxiesGood[proxy] = append(m.ProxiesGood[proxy], time.Now())
	if len(m.ProxiesGood[proxy]) > m.Requests {
		m.ProxiesGood[proxy] = m.ProxiesGood[proxy][1:]
	}
}

func (m *Manager) ForgiveProxy(proxy string) {
	m.MtxGood.Lock()
	defer m.MtxGood.Unlock()
	m.ProxiesGoodStrikes[proxy]--
	m.ProxiesGoodStrikes[proxy]--
}

func (m *Manager) PunishProxy(proxy string) {
	m.MtxGood.Lock()
	defer m.MtxGood.Unlock()
	m.ProxiesGoodStrikes[proxy]++
	if m.ProxiesGoodStrikes[proxy] >= m.StrikeLimit {
		delete(m.ProxiesGoodStrikes, proxy)
		delete(m.ProxiesGood, proxy)
	}
}

type proxies struct {
	Proxy  string
	Strike int
}

func (m *Manager) sortedGoodProxies() []proxies {
	x := []proxies{}
	for k, v := range m.ProxiesGoodStrikes {
		x = append(x, proxies{Proxy: k, Strike: v})
	}
	sort.Slice(x, func(i, j int) bool {
		return x[i].Strike < x[j].Strike
	})
	return x
}

func (m *Manager) GetGoodProxy() (string, *http.Client) {
	m.MtxGood.Lock()
	defer m.MtxGood.Unlock()
	m.sortedGoodProxies()
	proxy := ""
	for _, x := range m.sortedGoodProxies() {
		k := x.Proxy
		v := m.ProxiesGood[k]
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
		m.ProxiesTest <- proxies[i]
	}
}

func (m *Manager) AddProxies(proxies ...string) {
	go m.addProxies(proxies...)
}

func (m *Manager) removeProxyGood(str string) {
	m.MtxGood.Lock()
	defer m.MtxGood.Unlock()
	delete(m.ProxiesGood, str)
}

func (m *Manager) addProxyGood(str string) {
	m.MtxGood.Lock()
	defer m.MtxGood.Unlock()
	if _, exist := m.ProxiesGood[str]; !exist {
		m.ProxiesGood[str] = []time.Time{}
	}
	if _, exist := m.ProxiesGoodStrikes[str]; !exist {
		m.ProxiesGoodStrikes[str] = 0
	}
}

func (m *Manager) modifyTest(str string, delta int) {
	m.MtxTest.Lock()
	defer m.MtxTest.Unlock()
	m.ProxiesTested[str] = m.ProxiesTested[str] + delta
}

func (m *Manager) readTest(str string) int {
	m.MtxTest.Lock()
	defer m.MtxTest.Unlock()
	return m.ProxiesTested[str]
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
	if m.IP.Equal(returnedIP) {
		m.modifyTest(p, -10)
		return
	}
	if !m.FuncTest(client) {
		m.modifyTest(p, -3)
		return
	}
	m.modifyTest(p, 1)
	m.addProxyGood(p)
}

func (m *Manager) autoProxyTester() {
	client := clientFromString(m.TimeoutTest, "")
	for p := range m.ProxiesTest {
		m.test(client, p)
	}
}

func (m *Manager) WithFuncTest(fn func(*http.Client) bool) *Manager {
	m.FuncTest = fn
	return m
}

func (m *Manager) WithAutoProxyTester(concurrentTest int) *Manager {
	m.IP = getIP(m.URLTest)
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
		ProxiesTest:        make(chan string, 1000),
		URLTest:            "http://api.ipify.org/",
		ProxiesTested:      map[string]int{},
		ProxiesGood:        map[string][]time.Time{},
		ProxiesGoodStrikes: map[string]int{},
		FuncTest:           func(*http.Client) bool { return true },
		StrikeLimit:        10,
		TimeoutTest:        time.Second * 3,
		TimeoutGood:        time.Second * 4,
		TimeWindow:         time.Second * 12,
		Requests:           16,
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
