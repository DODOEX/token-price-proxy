package shared

import (
	"log"
	"strings"
	"time"

	"github.com/DODOEX/token-price-proxy/utils/config"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

func unmarshalChains(k *koanf.Koanf) []config.Chain {
	var chains []config.Chain
	err := k.Unmarshal("loadbalances", &chains)
	if err != nil {
		log.Fatalf("Unmarshal chains error: %v", err)
	}
	k.Set("chains", chains)
	return chains
}

func NewKoanfInstance() *koanf.Koanf {
	// 创建一个新的 koanf 实例
	k := koanf.New(".")

	// 定义你的默认值
	defaultValues := map[string]interface{}{
		"app.name":                "token-price-proxy",
		"app.host:":               ":8080",
		"app.idle-timeout":        50 * time.Second,
		"app.print-routes":        false,
		"app.prefork":             false,
		"app.production":          false,
		"redis.keeplive-interval": 30 * time.Second,
		"redis.retry-count":       3,
		"amqp.keeplive-interval":  30 * time.Second,
		"amqp.retry-count":        3,
	}

	// 使用 confmap provider 加载默认值。
	if err := k.Load(confmap.Provider(defaultValues, "."), nil); err != nil {
		log.Fatalf("error loading default values: %v", err)
	}

	// 加载本地配置文件
	if err := k.Load(file.Provider("config/default.yaml"), yaml.Parser()); err != nil {
		log.Panicf("Error loading defautl config: %v", err)
	}
	k.Print()
	log.Println("Load local config!")

	// 加载环境变量并合并到已加载的配置中。
	if err := k.Load(env.ProviderWithValue("token_price_proxy_", ".", func(s string, v string) (string, interface{}) {
		// 去掉 token_price_proxy_ 前缀,并将 _ 替换为 .
		key := strings.Replace(strings.TrimPrefix(s, "token_price_proxy_"), "_", ".", -1)

		// 如果值中包含空格，将值拆分成一个切片
		if strings.Contains(v, " ") {
			return key, strings.Split(v, " ")
		}

		// 否则，返回原始字符串
		return key, v
	}), nil); err != nil {
		log.Panicf("Error loading env: %v", err)
	}

	unmarshalChains(k)
	return k // 返回 koanf 实例
}
