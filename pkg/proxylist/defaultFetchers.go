package proxylist

import (
	"bufio"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"

	"github.com/PuerkitoBio/goquery"
	"github.com/chneau/limiter"
)

// ProxiesFromDailyFreeProxy returns proxies from https://www.dailyfreeproxy.com/.
func ProxiesFromDailyFreeProxy() []string {
	res, err := http.Get("https://www.dailyfreeproxy.com/")
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil
	}
	urls := []string{}
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
				urls = append(urls, "http://"+p)
			}
		})
	})
	limit.Wait()
	return urls
}

// ProxiesFromSmallSeoTools returns proxies from https://smallseotools.com/free-proxy-list/.
func ProxiesFromSmallSeoTools() []string {
	resp, err := http.Get("https://smallseotools.com/free-proxy-list/")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	bb, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	str := string(bb)
	regex := regexp.MustCompile(`\d+\.\d+\.\d+\.\d+:\d+`)
	proxies := regex.FindAllString(str, -1)
	urls := make([]string, len(proxies))
	for i := range proxies {
		urls[i] = "http://" + proxies[i]
	}
	return urls
}

// ProxiesFromDailyProxy returns proxies from https://proxy-daily.com/.
func ProxiesFromDailyProxy() []string {
	resp, err := http.Get("https://proxy-daily.com/")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	bb, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	str := string(bb)
	regex := regexp.MustCompile(`\d+\.\d+\.\d+\.\d+:\d+`)
	proxies := regex.FindAllString(str, -1)
	urls := make([]string, len(proxies))
	for i := range proxies {
		urls[i] = "http://" + proxies[i]
	}
	return urls
}

// ProxiesFromClarketm returns proxies from clarketm/proxy-list.
func ProxiesFromClarketm() []string {
	resp, err := http.Get("https://raw.githubusercontent.com/clarketm/proxy-list/master/proxy-list.txt")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	bb, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	str := string(bb)
	regex := regexp.MustCompile(`\d+\.\d+\.\d+\.\d+:\d+`)
	proxies := regex.FindAllString(str, -1)
	urls := make([]string, len(proxies))
	for i := range proxies {
		urls[i] = "http://" + proxies[i]
	}
	return urls
}

// ProxiesFromFate0 returns proxies from fate0/proxylist.
func ProxiesFromFate0() []string {
	resp, err := http.Get("https://raw.githubusercontent.com/fate0/proxylist/master/proxy.list")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	proxy := &struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}{}
	urls := []string{}
	for scanner.Scan() {
		err = json.Unmarshal(scanner.Bytes(), proxy)
		if err != nil {
			return nil
		}
		urls = append(urls, "http://"+proxy.Host+":"+strconv.Itoa(proxy.Port))
	}
	return urls
}

type proxy struct {
	IP   string `json:"ip"`
	Port string `json:"port"`
}

// ProxiesFromSunny9577 returns proxies from sunny9577/proxy-scraper.
func ProxiesFromSunny9577() []string {
	resp, err := http.Get("https://raw.githubusercontent.com/sunny9577/proxy-scraper/master/proxies.json")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	proxies := &struct {
		LastUpdated string  `json:"lastUpdated"`
		Proxynova   []proxy `json:"proxynova"`
		Usproxy     []proxy `json:"usproxy"`
		Hidemyname  []proxy `json:"hidemyname"`
	}{}
	err = json.NewDecoder(resp.Body).Decode(proxies)
	if err != nil {
		return nil
	}
	urls := []string{}
	for _, proxy := range proxies.Proxynova {
		urls = append(urls, "http://"+proxy.IP+":"+proxy.Port)
	}
	for _, proxy := range proxies.Usproxy {
		urls = append(urls, "http://"+proxy.IP+":"+proxy.Port)
	}
	for _, proxy := range proxies.Hidemyname {
		urls = append(urls, "http://"+proxy.IP+":"+proxy.Port)
	}
	return urls
}

// TODO: check if other exist here https://github.com/topics/proxy-list?o=desc&s=updated