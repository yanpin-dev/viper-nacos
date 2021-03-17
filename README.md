# viper-nacos
## Introduce
Integrate nacos to viper

## Usage:
```golang
package main

import (
	"fmt"
	"github.com/spf13/viper"
	_ "github.com/yanpin-dev/viper-nacos/pkg/nacos"
	"os"
)

// Run Example:
// go run main.go http://sample.nacos.io:8848/nacos?namespace=test&dataId=test.yaml&group=DEFAULT_GROUP
func main() {
	if len(os.Args) != 2 {
		fmt.Println("should pass one parameter")
		os.Exit(1)
	}
	endpoint := os.Args[1]
	v := viper.New()
	v.SetConfigType("yaml")
	v.AddRemoteProvider("nacos", endpoint, "")
	err := v.ReadRemoteConfig()
	if err != nil {
		fmt.Printf("failed to read remote config: %s\n", err.Error())
		os.Exit(-1)
	}

	a := v.Get("test")
	fmt.Printf("%v\n", a)
}

```