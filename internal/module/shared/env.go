package shared

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

var (
	nameToIDMapping           = make(map[string]string)      // 全局变量，存储链名称到链ID的映射
	idToIDMapping             = make(map[string]string)      // 全局变量，存储链ID到链ID的映射
	idToNameMapping           = make(map[string]string)      // 全局变量，存储链链ID到名称的映射
	usdtAddresses             = make(map[string]USDTAddress) // 全局变量，存储链ID到USDT地址及小数位的映射
	DodoexRouteUrl            = "https://api.dodoex.io/route-service/v2/backend/swap"
	AllowApiKeyNil            = true
	AllowApiKeyNilRateLimiter = 1000
	AllowedTokens             = []string{"*USD*", "DAI", "*DODO", "JOJO", "*BTC*", "*ETH*", "*MATIC*", "*BNB*", "*AVAX", "*NEAR", "*XRP", "TON*", "*ARB", "ENS"} // 设置默认值
	//拒绝的链ID
	RefuseChainIdMap = map[string]int{
		"128": 1,
	}
	mu sync.RWMutex // 读写锁保护映射的并发访问
)

// USDTAddress 存储 USDT 地址及其小数位数
type USDTAddress struct {
	Address string `json:"address"`
	Decimal int    `json:"decimal"`
}

// LoadEnv 加载 .env 文件中的环境变量，并更新链映射
func LoadEnv() (struct{}, error) {
	UpdateChainMapping()
	LoadAllowApiKey()
	LoadUSDTAddresses()
	LoadDodoexRouteUrl()
	LoadAllowedTokens()
	LoadRefuseChainId()
	return struct{}{}, nil
}
func LoadAllowApiKey() {
	AllowApiKeyNil := os.Getenv("ALLOW_API_KEY") != "false"
	fmt.Printf("AllowApiKeyNil is %t\n", AllowApiKeyNil)
	AllowApiKeyNilRateLimiterEnv := os.Getenv("ALLOW_API_KEY_DEFAULT_RATE_LIMITER")
	if AllowApiKeyNilRateLimiterEnv != "" {
		AllowApiKeyNilRateLimiter, _ = strconv.Atoi(AllowApiKeyNilRateLimiterEnv)
	}
	fmt.Printf("AllowApiKeyNilRateLimiter is %d\n", AllowApiKeyNilRateLimiter)
}

// UpdateChainMapping 解析环境变量 CHAIN_MAPPING 并更新全局映射
func UpdateChainMapping() {
	chainMappingStr := os.Getenv("CHAIN_MAPPING")
	if chainMappingStr == "" {
		fmt.Println("环境变量 CHAIN_MAPPING 未设置")
		return
	}

	newNameToIDMapping := make(map[string]string)
	newIDToIDMapping := make(map[string]string)

	// 解析 JSON 字符串
	err := json.Unmarshal([]byte(chainMappingStr), &newNameToIDMapping)
	if err != nil {
		fmt.Println("解析 CHAIN_MAPPING 时出错:", err)
		return
	}

	// 构建 ID 映射
	for name, id := range newNameToIDMapping {
		newIDToIDMapping[id] = id
		idToNameMapping[id] = name
	}

	// 使用写锁保护更新操作
	mu.Lock()
	nameToIDMapping = newNameToIDMapping
	idToIDMapping = newIDToIDMapping
	mu.Unlock()
	fmt.Printf("环境变量 CHAIN_MAPPING 设置成功，长度为 %d \n", len(nameToIDMapping))
}

// LoadUSDTAddresses 解析环境变量 USDT_ADDRESSES 并更新全局映射
func LoadUSDTAddresses() {
	usdtAddressesStr := os.Getenv("USDT_ADDRESSES")
	if usdtAddressesStr == "" {
		fmt.Println("环境变量 USDT_ADDRESSES 未设置")
		return
	}

	newUsdtAddresses := make(map[string]USDTAddress)

	// 解析 JSON 字符串
	err := json.Unmarshal([]byte(usdtAddressesStr), &newUsdtAddresses)
	if err != nil {
		fmt.Println("解析 USDT_ADDRESSES 时出错:", err)
		return
	}

	// 使用写锁保护更新操作
	mu.Lock()
	usdtAddresses = newUsdtAddresses
	mu.Unlock()
	fmt.Printf("环境变量 USDT_ADDRESSES 设置成功，长度为 %d \n", len(usdtAddresses))
}

func LoadDodoexRouteUrl() {
	newDodoexRouteUrl := os.Getenv("DODOEX_ROUTE_URL")
	if newDodoexRouteUrl == "" {
		fmt.Println("环境变量 DODOEX_ROUTE_URL 未设置")
		return
	}
	DodoexRouteUrl = newDodoexRouteUrl
	fmt.Printf("环境变量 DODOEX_ROUTE_URL 设置成功 %s \n", DodoexRouteUrl)
}

// GetChainID 根据链名称或链ID字符串返回相应的链ID
func GetChainID(chainNameOrID string) (string, error) {
	// 使用读锁保护读取操作
	mu.RLock()
	defer mu.RUnlock()

	// 检查输入是否为有效的链名称
	if chainID, exists := nameToIDMapping[chainNameOrID]; exists {
		return chainID, nil
	}

	// 检查输入是否为有效的链ID（必须存在于环境变量定义的映射中）
	if chainID, exists := idToIDMapping[chainNameOrID]; exists {
		return chainID, nil
	}

	return "", fmt.Errorf("无效的链名称或ID: %s", chainNameOrID)
}

// GetChainName 根据链ID返回相应的链名称
func GetChainName(chainID string) (string, error) {
	// 使用读锁保护读取操作
	mu.RLock()
	defer mu.RUnlock()

	if chainName, exists := idToNameMapping[chainID]; exists {
		return chainName, nil
	}

	return "", fmt.Errorf("无效的链ID: %s", chainID)
}

// GetUSDTAddress 根据链ID返回相应的USDT地址及其小数位数
func GetUSDTAddress(chainID string) (USDTAddress, error) {
	// 使用读锁保护读取操作
	mu.RLock()
	defer mu.RUnlock()

	if usdtAddress, exists := usdtAddresses[chainID]; exists {
		return usdtAddress, nil
	}

	return USDTAddress{}, fmt.Errorf("无效的链ID: %s", chainID)
}

// LoadAllowedTokens 加载环境变量中的允许 token 列表
func LoadAllowedTokens() {
	tokensStr := os.Getenv("GECKO_CHAIN_ALLOWED_TOKENS")
	if tokensStr == "" {
		fmt.Println("环境变量 GECKO_CHAIN_ALLOWED_TOKENS 未设置")
		return
	}

	// 分割字符串并去除多余空格
	tokens := strings.Fields(tokensStr)

	// 使用写锁保护更新操作
	mu.Lock()
	AllowedTokens = tokens
	mu.Unlock()
	fmt.Printf("环境变量 GECKO_CHAIN_ALLOWED_TOKENS 设置成功，长度为 %d \n", len(AllowedTokens))
}

func LoadRefuseChainId() {
	chainIds := os.Getenv("REFUSE_CHAIN_IDS")
	if chainIds == "" {
		fmt.Println("环境变量 REFUSE_CHAIN_IDS 未设置")
		return
	}

	// 分割字符串并去除多余空格
	chainIdList := strings.Fields(chainIds)

	// 使用写锁保护更新操作
	mu.Lock()
	RefuseChainIdMap := make(map[string]int)
	for _, key := range chainIdList {
		RefuseChainIdMap[key] = 1
	}
	mu.Unlock()
	fmt.Printf("环境变量 REFUSE_CHAIN_IDS 设置成功，长度为 %d \n", len(RefuseChainIdMap))
}
