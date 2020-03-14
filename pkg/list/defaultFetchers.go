package list

import (
	"bufio"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/PuerkitoBio/goquery"
	"github.com/chneau/limiter"
)

// ProxiesFromDailyFreeProxy returns proxies from https://www.dailyfreeproxy.com/.
func ProxiesFromDailyFreeProxy() ([]*url.URL, error) {
	doc, err := goquery.NewDocument("https://www.dailyfreeproxy.com/")
	if err != nil {
		return nil, err
	}
	urls := []*url.URL{}
	limit := limiter.New(4)
	doc.Find("h3 > a").Each(func(i int, s *goquery.Selection) {
		next := s.AttrOr("href", "")
		if next == "" {
			return
		}
		limit.Execute(func() {
			resp, err := http.Get(next)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			bb, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return
			}
			str := string(bb)
			regex := regexp.MustCompile(`\d+\.\d+\.\d+\.\d+:\d+`)
			proxies := regex.FindAllString(str, -1)
			for _, p := range proxies {
				urls = append(urls, &url.URL{Scheme: "http", Host: p})
			}
		})
	})
	limit.Wait()
	return urls, nil
}

// ProxiesFromSmallSeoTools returns proxies from https://smallseotools.com/free-proxy-list/.
func ProxiesFromSmallSeoTools() ([]*url.URL, error) {
	resp, err := http.Get("https://smallseotools.com/free-proxy-list/")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bb, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	str := string(bb)
	regex := regexp.MustCompile(`\d+\.\d+\.\d+\.\d+:\d+`)
	proxies := regex.FindAllString(str, -1)
	urls := make([]*url.URL, len(proxies))
	for i := range proxies {
		urls[i] = &url.URL{Scheme: "http", Host: proxies[i]}
	}
	return urls, nil
}

// ProxiesFromDailyProxy returns proxies from https://proxy-daily.com/.
func ProxiesFromDailyProxy() ([]*url.URL, error) {
	resp, err := http.Get("https://proxy-daily.com/")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bb, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	str := string(bb)
	regex := regexp.MustCompile(`\d+\.\d+\.\d+\.\d+:\d+`)
	proxies := regex.FindAllString(str, -1)
	urls := make([]*url.URL, len(proxies))
	for i := range proxies {
		urls[i] = &url.URL{Scheme: "http", Host: proxies[i]}
	}
	return urls, nil
}

// ProxiesFromClarketm returns proxies from clarketm/proxy-list.
func ProxiesFromClarketm() ([]*url.URL, error) {
	resp, err := http.Get("https://raw.githubusercontent.com/clarketm/proxy-list/master/proxy-list.txt")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bb, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	str := string(bb)
	regex := regexp.MustCompile(`\d+\.\d+\.\d+\.\d+:\d+`)
	proxies := regex.FindAllString(str, -1)
	urls := make([]*url.URL, len(proxies))
	for i := range proxies {
		urls[i] = &url.URL{Scheme: "http", Host: proxies[i]}
	}
	return urls, nil
}

// ProxiesFromFate0 returns proxies from fate0/proxylist.
func ProxiesFromFate0() ([]*url.URL, error) {
	resp, err := http.Get("https://raw.githubusercontent.com/fate0/proxylist/master/proxy.list")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	proxy := &struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}{}
	urls := []*url.URL{}
	for scanner.Scan() {
		json.Unmarshal(scanner.Bytes(), proxy)
		urls = append(urls, &url.URL{Scheme: "http", Host: proxy.Host + ":" + strconv.Itoa(proxy.Port)})
	}
	return urls, nil
}

type proxy struct {
	IP        string `json:"ip"`
	Port      string `json:"port"`
	Country   string `json:"country"`
	Anonymity string `json:"anonymity"`
	Type      string `json:"type"`
}

// ProxiesFromSunny9577 returns proxies from sunny9577/proxy-scraper.
func ProxiesFromSunny9577() ([]*url.URL, error) {
	resp, err := http.Get("https://raw.githubusercontent.com/sunny9577/proxy-scraper/master/proxies.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	proxies := &struct {
		LastUpdated string  `json:"lastUpdated"`
		Proxynova   []proxy `json:"proxynova"`
		Usproxy     []proxy `json:"usproxy"`
		Hidemyname  []proxy `json:"hidemyname"`
	}{}
	json.NewDecoder(resp.Body).Decode(proxies)
	urls := []*url.URL{}
	for _, proxy := range proxies.Proxynova {
		urls = append(urls, &url.URL{Scheme: "http", Host: proxy.IP + ":" + proxy.Port})
	}
	for _, proxy := range proxies.Usproxy {
		urls = append(urls, &url.URL{Scheme: "http", Host: proxy.IP + ":" + proxy.Port})
	}
	for _, proxy := range proxies.Hidemyname {
		urls = append(urls, &url.URL{Scheme: "http", Host: proxy.IP + ":" + proxy.Port})
	}
	return urls, nil
}

// TODO: check if other exist here https://github.com/topics/proxy-list?o=desc&s=updated
