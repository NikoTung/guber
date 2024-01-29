package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"time"

	"github.com/IrineSistiana/mosdns/v5/mlog"
	"go.uber.org/zap"
)

type NacosClient struct {
	NacosConfig *Nacos
	AccessToken string
	TokenTtl    int64
	LastLogin   int64
	Cache       map[string][]string
	Env         string
}

// {"accessToken":"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJuYWNvcyIsImV4cCI6MTYwNTYyOTE2Nn0.2TogGhhr11_vLEjqKko1HJHUJEmsPuCxkur-CfNojDo","tokenTtl":18000,"globalAdmin":true}
type loginResponse struct {
	AccessToken string `json:"accessToken"`
	TokenTtl    int64  `json:"tokenTtl"`
	GlobalAdmin bool   `json:"globalAdmin"`
}

// {"name":"DEFAULT_GROUP@@ownbank-deposit-service","groupName":"DEFAULT_GROUP","clusters":"","cacheMillis":10000,"hosts":[{"ip":"192.168.56.199","port":11663,"weight":1.0,"healthy":true,"enabled":true,"ephemeral":true,"clusterName":"DEFAULT","serviceName":"DEFAULT_GROUP@@ownbank-deposit-service","metadata":{"dubbo.metadata-service.url-params":"{\"connections\":\"1\",\"version\":\"1.0.0\",\"dubbo\":\"2.0.2\",\"release\":\"3.1.3\",\"side\":\"provider\",\"port\":\"11663\",\"protocol\":\"dubbo\"}","X_ROUTER_TAG":"stable","dubbo.endpoints":"[{\"port\":11663,\"protocol\":\"dubbo\"}]","dubbo.metadata.revision":"9fcdb0a156559f3f0a0bf83623a20fbd","dubbo.metadata.storage-type":"local","timestamp":"1700551351464"},"instanceHeartBeatInterval":5000,"instanceHeartBeatTimeOut":15000,"ipDeleteTimeout":30000}],"lastRefTime":1705474386546,"checksum":"","allIPs":false,"reachProtectionThreshold":false,"valid":true}
type nacosService struct {
	Name  string      `json:"name"`
	Hosts []nacosHost `json:"hosts"`
}

// hosts
type nacosHost struct {
	Ip       string            `json:"ip"`
	Port     int32             `json:"port"`
	Healthy  bool              `json:"healthy"`
	Metadata map[string]string `json:"metadata"`
}

func NewNacosClient(nacosConfig *Nacos, env string) *NacosClient {
	return &NacosClient{
		NacosConfig: nacosConfig,
		AccessToken: "",
		TokenTtl:    0,
		LastLogin:   0,
		Cache:       make(map[string][]string),
		Env:         env,
	}
}

func (n *NacosClient) Run(g *Guber) error {
	err := n.Login()
	if err != nil {
		return err
	}
	n.LastLogin = time.Now().Unix()
	ticker := time.NewTicker(1000 * time.Second)

	go func() {
		err := g.GetSafeClose().WaitClosed()
		if err != nil {
			g.Logger().Fatal("nacos exited", zap.Error(err))
		} else {
			g.Logger().Info("nacos exited")
		}
		ticker.Stop()
	}()

	go func() {
		for range ticker.C {
			mlog.L().Info("refresh nacos access token")
			if (time.Now().Unix() - n.LastLogin) >= n.TokenTtl {
				mlog.L().Debug("ticker")
				n.Login()
			}
		}
	}()

	return nil
}

// curl -X POST '127.0.0.1:8848/nacos/v1/auth/login' -d 'username=nacos&password=nacos'
// login
func (n *NacosClient) Login() error {
	params := url.Values{
		"username": {n.NacosConfig.Username},
		"password": {n.NacosConfig.Password},
	}

	resp, err := http.PostForm(n.NacosConfig.Addr+"/nacos/v1/auth/login", params)
	if err != nil {
		mlog.L().Error("nacos login failed", zap.String("addr", n.NacosConfig.Addr), zap.Error(err))
		return err
	}

	defer resp.Body.Close()
	rs := resp.StatusCode == 200
	if !rs {
		mlog.L().Error("nacos login response none 200 status code", zap.String("addr", n.NacosConfig.Addr), zap.Int("status", resp.StatusCode))
		return errors.New("nacos login response none 200 status code")
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		mlog.L().Error("nacos login response read failed", zap.String("addr", n.NacosConfig.Addr), zap.Error(err))
		return err
	}

	var loginResponse loginResponse
	err = json.Unmarshal(b, &loginResponse)
	if err != nil {
		mlog.L().Error("nacos login response json unmarshal failed", zap.String("addr", n.NacosConfig.Addr), zap.Error(err))
		return err
	}

	n.AccessToken = loginResponse.AccessToken
	n.TokenTtl = loginResponse.TokenTtl
	n.LastLogin = time.Now().Unix()

	return nil

}

// curl -X GET 'http://127.0.0.1:8848/nacos/v1/ns/instance/list?accessToken=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJuYWNvcyIsImV4cCI6MTcwNTQ5MjM0MX0.jXXwYRrQxWjETD1fLxOb2i0heXg7xz_vIyEcy2Y&serviceName=nacos.test.1'
func (n *NacosClient) GetNacosService(app string, keep []Meta) (string, []string, error) {
	params := url.Values{
		"accessToken": {n.AccessToken},
		"serviceName": {app},
	}

	resp, err := http.Get(n.NacosConfig.Addr + "/nacos/v1/ns/instance/list?" + params.Encode())
	if err != nil {
		mlog.L().Error("nacos get service failed", zap.String("app", app), zap.Error(err))
		return "", nil, err
	}

	defer resp.Body.Close()
	rs := resp.StatusCode == 200
	if !rs {
		mlog.L().Error("nacos get service response none 200 status code", zap.String("app", app), zap.Int("status", resp.StatusCode))
		return "", nil, errors.New("nacos get service response none 200 status code")
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		mlog.L().Error("nacos get service response read failed", zap.String("app", app), zap.Error(err))
		return "", nil, err
	}

	var nacosService nacosService
	err = json.Unmarshal(b, &nacosService)
	if err != nil {
		mlog.L().Error("nacos get service response json unmarshal failed", zap.String("app", app), zap.Error(err))
		return "", nil, err
	}

	if len(nacosService.Hosts) == 0 {
		mlog.L().Error("nacos get service response hosts is empty", zap.String("app", app))
		return "", nil, errors.New("nacos get service response hosts is empty")
	}

	var hosts []string

	for _, host := range nacosService.Hosts {
		if !host.Healthy {
			continue
		}

		if len(keep) == 0 {
			hosts = append(hosts, host.Ip)
			continue
		} else {
			for _, k := range keep {
				if v, ok := host.Metadata[k.Key]; ok && (k.Val == "" || v == k.Val) {
					hosts = append(hosts, host.Ip)
					break
				}
			}
		}
	}

	name := fmt.Sprintf("%s.%s", app, n.Env)

	//check if hosts is in cache
	if hs, ok := n.Cache[name]; ok {
		if len(hs) == len(hosts) && reflect.DeepEqual(hs, hosts) {
			return "", nil, errors.New("nacos get service response hosts is same")
		}
	}

	n.Cache[name] = hosts

	mlog.L().Info("nacos get service success", zap.String("app", name), zap.Strings("hosts", hosts), zap.String("addr", n.NacosConfig.Addr))

	return name, hosts, nil
}
