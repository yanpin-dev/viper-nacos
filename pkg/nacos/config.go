package nacos

import (
	"bytes"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"io"
	"net/url"
	"strconv"
	"strings"
)

const (
	ProviderName = "nacos"

	KeyDataId = "DataId"
	KeyGroup  = "Group"
)

var supportedProviders = []string{ProviderName}

type configFactory interface {
	Get(rp viper.RemoteProvider) (io.Reader, error)
	Watch(rp viper.RemoteProvider) (io.Reader, error)
	WatchChannel(rp viper.RemoteProvider) (<-chan *viper.RemoteResponse, chan bool)
}
type configManager interface {
	Get(key string) ([]byte, error)
	Watch(key string, stop chan bool) <-chan *viper.RemoteResponse
}

func getConfigManager(rp viper.RemoteProvider) (configManager, error) {
	switch rp.Provider() {
	case ProviderName:
		return newNacosConfigManager(rp)
	default:
		return nil, errors.New("this configuration manager is not supported: " + rp.Provider())
	}
}

type nacosConfigManager struct {
	client config_client.IConfigClient
	dataId string
	group  string
}

type nacosConfig struct {
	constant.ServerConfig

	namespace string
	dataId    string
	group     string
}

func newNacosConfigManager(rp viper.RemoteProvider) (*nacosConfigManager, error) {
	cfg, err := extractNacosConfig(rp)
	if err != nil {
		return nil, err
	}
	sc := []constant.ServerConfig{
		cfg.ServerConfig,
	}
	cc := constant.ClientConfig{
		NamespaceId:         cfg.namespace, //namespace id
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogDir:              "/tmp/nacos/log",
		CacheDir:            "/tmp/nacos/cache",
		RotateTime:          "1h",
		MaxAge:              3,
		LogLevel:            "debug",
	}

	client, err := clients.NewConfigClient(
		vo.NacosClientParam{
			ClientConfig:  &cc,
			ServerConfigs: sc,
		},
	)

	if err != nil {
		return nil, err
	}

	return &nacosConfigManager{
		client: client,
		dataId: cfg.dataId,
		group:  cfg.group,
	}, nil
}

func extractNacosConfig(rp viper.RemoteProvider) (*nacosConfig, error) {
	url, err := url.Parse(rp.Endpoint())
	if err != nil {
		return nil, err
	}

	var port int
	if url.Port() == "" {
		switch url.Scheme {
		case "http":
			port = 80
		case "https":
			port = 443
		default:
			return nil, errors.New("port is not defined.")
		}
	} else {
		port, err = strconv.Atoi(url.Port())
		if err != nil {
			return nil, err
		}
	}

	ns := url.Query().Get("namespace")
	group := url.Query().Get("group")
	dataId := url.Query().Get("dataId")

	return &nacosConfig{
		ServerConfig: constant.ServerConfig{
			Scheme:      url.Scheme,
			ContextPath: url.Path,
			IpAddr:      url.Hostname(),
			Port:        uint64(port),
		},
		namespace: ns,
		dataId:    dataId,
		group:     group,
	}, nil
}

func (m *nacosConfigManager) Get(key string) ([]byte, error) {

	content, err := m.client.GetConfig(vo.ConfigParam{
		DataId: m.dataId,
		Group:  m.group,
	})

	if err != nil {
		return nil, err
	}

	return []byte(content), nil
}

func (m *nacosConfigManager) Watch(key string, stop chan bool) <-chan *viper.RemoteResponse {
	c := make(chan *viper.RemoteResponse)
	m.client.ListenConfig(vo.ConfigParam{
		DataId: key,
		Group:  m.group,
		OnChange: func(namespace, group, dataId, data string) {
			c <- &viper.RemoteResponse{
				Value: []byte(data),
				Error: nil,
			}
		},
	})

	go func(client config_client.IConfigClient, dataId, group string) {
		for {
			select {
			case <-stop:
				client.CancelListenConfig(vo.ConfigParam{
					DataId: dataId,
					Group:  group,
				})
				return
			}
		}
	}(m.client, key, m.group)

	return c
}

func (m *nacosConfigManager) extractKey(key string) (string, string) {
	dataId, group := "", ""
	items := strings.Split(key, "&")
	for _, item := range items {
		param := strings.SplitN(item, "=", 2)
		if strings.ToLower(KeyDataId) == strings.ToLower(param[0]) {
			dataId = param[1]
			continue
		}
		if strings.ToLower(KeyGroup) == strings.ToLower(param[0]) {
			group = param[1]
			continue
		}
	}
	return dataId, group

}

type nacosProvider struct {
	delegate configFactory
}

func (n *nacosProvider) Get(rp viper.RemoteProvider) (io.Reader, error) {
	if !stringInSlice(rp.Provider(), supportedProviders) {
		return n.delegate.Get(rp)
	}
	cm, err := getConfigManager(rp)
	if err != nil {
		return nil, err
	}
	var data []byte
	data, err = cm.Get(rp.Path())
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (n *nacosProvider) Watch(rp viper.RemoteProvider) (io.Reader, error) {
	if !stringInSlice(rp.Provider(), supportedProviders) {
		return n.delegate.Watch(rp)
	}
	return n.Get(rp)
}
func (n *nacosProvider) WatchChannel(rp viper.RemoteProvider) (<-chan *viper.RemoteResponse, chan bool) {
	if !stringInSlice(rp.Provider(), supportedProviders) {
		return n.delegate.WatchChannel(rp)
	}
	cm, err := getConfigManager(rp)
	if err != nil {
		panic("failed to get nacos config manager")
	}
	c := make(chan *viper.RemoteResponse)

	quit := make(chan bool)
	cm.Watch(rp.Path(), quit)
	return c, quit
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
