package nacos

import (
	"github.com/spf13/viper"
)

func init() {
	viper.SupportedRemoteProviders = append(viper.SupportedRemoteProviders, "nacos")
	viper.RemoteConfig = &nacosProvider{delegate: viper.RemoteConfig}
}
