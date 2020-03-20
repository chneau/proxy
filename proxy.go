package proxy

import (
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
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
	ProxiesBanned map[string]bool
	MtxTest       sync.Mutex
	FuncTest      func(*http.Client) bool

	// good to use
	MtxGood            sync.Mutex
	ProxiesGood        map[string][]time.Time
	ProxiesGoodStrikes map[string]int
	StrikeLimit        int
	Requests           int
	Wait               time.Duration
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
	punishment := 1
	if score := -(m.ProxiesGoodStrikes[proxy] / 10); score > punishment {
		punishment = score
	}
	m.ProxiesGoodStrikes[proxy] += punishment
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
	proxy := ""
	for _, x := range m.sortedGoodProxies() {
		k := x.Proxy
		v := m.ProxiesGood[k]
		if len(v) < m.Requests {
			proxy = k
			break
		}
		if time.Since(v[len(v)-1]) < m.Wait {
			continue
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

func (m *Manager) removeProxyGood(proxy string) {
	m.MtxGood.Lock()
	defer m.MtxGood.Unlock()
	delete(m.ProxiesGood, proxy)
}

func (m *Manager) addProxyGood(proxy string) {
	m.MtxGood.Lock()
	defer m.MtxGood.Unlock()
	if _, exist := m.ProxiesGood[proxy]; !exist {
		m.ProxiesGood[proxy] = []time.Time{}
	}
	if _, exist := m.ProxiesGoodStrikes[proxy]; !exist {
		m.ProxiesGoodStrikes[proxy] = 0
	}
}

func (m *Manager) modifyTest(proxy string, delta int) {
	m.MtxTest.Lock()
	defer m.MtxTest.Unlock()
	m.ProxiesTested[proxy] = m.ProxiesTested[proxy] + delta
}

func (m *Manager) readTest(proxy string) int {
	m.MtxTest.Lock()
	defer m.MtxTest.Unlock()
	return m.ProxiesTested[proxy]
}

func (m *Manager) ban(proxy string) {
	m.MtxTest.Lock()
	defer m.MtxTest.Unlock()
	m.ProxiesBanned[proxy] = true
}

func (m *Manager) isBanned(proxy string) bool {
	m.MtxTest.Lock()
	defer m.MtxTest.Unlock()
	return m.ProxiesBanned[proxy]
}

func (m *Manager) test(client *http.Client, proxy string) {
	if m.isBanned(proxy) {
		return
	}
	if m.readTest(proxy) <= -10 {
		m.modifyTest(proxy, -10)
		score := m.readTest(proxy)
		if score <= -50 {
			m.modifyTest(proxy, -score)
			m.AddProxies(proxy)
		}
		return
	}
	client.Transport.(*FakeTransport).Proxy = http.ProxyURL(strToURL(proxy))
	resp, err := client.Get(m.URLTest)
	if err != nil {
		m.modifyTest(proxy, -1)
		m.AddProxies(proxy)
		return
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		m.modifyTest(proxy, -1)
		m.AddProxies(proxy)
		return
	}
	returnedIP := net.ParseIP(string(b))
	if returnedIP == nil {
		m.modifyTest(proxy, -1)
		m.AddProxies(proxy)
		return
	}
	if m.IP.Equal(returnedIP) {
		m.modifyTest(proxy, -10)
		m.ban(proxy)
		return
	}
	if !m.FuncTest(client) {
		m.modifyTest(proxy, -3)
		return
	}
	m.modifyTest(proxy, 1)
	m.addProxyGood(proxy)
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
		ProxiesBanned:      map[string]bool{},
		FuncTest:           func(*http.Client) bool { return true },
		StrikeLimit:        10,
		Wait:               time.Millisecond * 200,
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
	var transport http.RoundTripper = &FakeTransport{
		Transport: &http.Transport{
			DisableKeepAlives: true,
			Proxy:             http.ProxyURL(strToURL(proxy)),
			Dial:              (&net.Dialer{Timeout: timeout, KeepAlive: timeout}).Dial,
		},
	}
	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
	return client
}

var _ http.RoundTripper = (*FakeTransport)(nil)

type FakeTransport struct {
	*http.Transport
}

func (ft *FakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-Forwarded-For", strconv.Itoa(rand.Intn(256))+"."+strconv.Itoa(rand.Intn(256))+"."+strconv.Itoa(rand.Intn(256))+"."+strconv.Itoa(rand.Intn(256)))
	return ft.Transport.RoundTrip(req)
}
