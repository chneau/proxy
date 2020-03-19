package proxy

import (
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Manager struct {
	proxiesToTest chan string
	timeout       time.Duration
	URLTest       string
	ip            net.IP
	tested        map[string]int
	mtxTest       sync.Mutex
}

func (m *Manager) AddProxies(proxies ...string) {
	go func() {
		for i := range proxies {
			m.proxiesToTest <- proxies[i]
		}
	}()
}

func (m *Manager) modifyTest(str string, delta int) {
	m.mtxTest.Lock()
	defer m.mtxTest.Unlock()
	m.tested[str] = m.tested[str] + delta
}

func (m *Manager) readTest(str string) int {
	m.mtxTest.Lock()
	defer m.mtxTest.Unlock()
	return m.tested[str]
}

func (m *Manager) autoTest() {
	client := &http.Client{
		Timeout: m.timeout,
		Transport: &http.Transport{
			DisableKeepAlives: true,
			Dial:              (&net.Dialer{Timeout: m.timeout, KeepAlive: m.timeout}).Dial,
		},
	}
	for p := range m.proxiesToTest {
		if m.readTest(p) <= -10 {
			continue
		}
		client.Transport.(*http.Transport).Proxy = http.ProxyURL(strToURL(p))
		start := time.Now()
		resp, err := client.Get(m.URLTest)
		if err != nil {
			m.modifyTest(p, -1)
			m.AddProxies(p)
			continue
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			m.modifyTest(p, -1)
			m.AddProxies(p)
			continue
		}
		returnedIP := net.ParseIP(string(b))
		if returnedIP == nil {
			m.modifyTest(p, -1)
			m.AddProxies(p)
			continue
		}
		if m.ip.Equal(returnedIP) {
			m.modifyTest(p, -10)
			continue
		}
		m.modifyTest(p, 1)
		duration := time.Since(start)
		log.Println(p, returnedIP, duration)
	}
}

func NewDefaultManager() *Manager {
	m := &Manager{
		proxiesToTest: make(chan string, 1000),
		timeout:       time.Second * 3,
		URLTest:       "http://api.ipify.org/",
		ip:            getIP("http://api.ipify.org/"),
		tested:        map[string]int{},
	}
	for i := 0; i < 100; i++ {
		go m.autoTest()
	}
	return m
}

func strToURL(proxy string) *url.URL {
	parsedURL, _ := url.Parse(proxy)
	return parsedURL
}

func getIP(url string) net.IP {
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
