<div align="center" id="top">
<img alt="logo" width="80" height="80" style="border-radius: 20px" src="icon.png"/>
</div>
<h1 align="center">DODOEX Price API v2</h1>
<div align="center">
  <img alt="Go Version Badge" src="https://img.shields.io/badge/Go-1.21-blue"/>
  &#xa0;
  <a href="https://app.codacy.com/gh/DODOEX/token-price-proxy?utm_source=github.com&utm_medium=referral&utm_content=DODOEX/token-price-proxy&utm_campaign=Badge_Grade"><img alt="Codacy Badge" src="https://api.codacy.com/project/badge/Grade/de013bb362a3436c9d1872bce5ab3c04"/></a>
  &#xa0;
  <img alt="License Badge" src="https://camo.githubusercontent.com/cd7febaf01f1e0dfe434776bfbea02b20b1e4afeed9b8e3ed09f20f3e138a545/68747470733a2f2f696d672e736869656c64732e696f2f6769746875622f6c6963656e73652f444f444f45582f776562332d7270632d70726f78792e737667"/>
  &#xa0;
  <img alt="Release Badge" src="https://img.shields.io/github/release/DODOEX/token-price-proxy"/>
</div>
<div align="center">
  <a href="README_ZH.md">中文</a>
  &#xa0;|&#xa0;
  <a href="../README.md">English</a>
</div>
</br>

## 项目简介

DODOEX Price API v2 旨在通过聚合多个价格平台来获取 Token 价格。当前对接了多个数据平台和 Dodoex Route 平台，支持当前价格和历史价格的查询。

## 功能特性

- **多平台价格获取与处理**: 支持多个主流平台（CoinGecko、GeckoTerminal、DefiLlama、Dodoex Route、CoinGecko OnChain）进行价格查询。通过 Redis 实现价格缓存和队列处理，用户可根据配置禁用特定数据源。
- **Redis 集成与请求管理**: 利用 Redis 队列与 Lua 脚本管理价格请求，确保唯一性与顺序处理，并通过去重和批量处理提升系统操作效率。
- **结果订阅与异步推送**: 采用 Redis 的发布/订阅（Pub/Sub）模式，将处理后的价格结果推送给请求方，支持异步处理大批量请求。
- **节流与错误管理**: 引入节流机制，防止频繁查询消耗资源，并在获取价格失败时记录日志，保障系统稳定性。
- **灵活配置与自定义选项**: 支持通过 Koanf 库加载配置，允许自定义价格源、节流设置和 Redis 参数，以满足不同需求。
- **价格数据源的手动配置与自动选择**: 用户可手动配置价格数据源，或选择使用上次成功获取价格的数据源，以提高查询效率。
- **高效的 Redis 队列与数据更新**: 通过 Redis 队列管理价格请求，借助 Lua 脚本确保唯一性和有序处理，提高数据更新效率。
- **接口限流与访问控制**: 使用 Redis Lua 脚本实现接口限流，并支持通过 API Key 进行访问控制，增强接口安全性。
- **关联 Token 查询**: 支持关联 Token 查询，确保冷门链上的 Token 价格也能准确获取，扩大价格覆盖范围。
- **定时更新 Token 列表与 Redis 队列**: 定期更新 CoinGecko 的 Token 列表和 Redis 队列中的请求，确保系统数据的最新与准确。

## 安装与运行

### 运行初始化 SQL 脚本

在启动项目之前，推荐执行初始化 SQL 脚本以设置数据库的初始状态。可以通过以下命令来执行：

```sh
psql -U <username> -d <database_name> -f /sql/init.sql
```

### 克隆项目

```sh
git clone https://github.com/DODOEX/token-price-proxy
cd token-price-proxy
```

### 构建项目

```sh
go build ./cmd/main.go
```

### 运行项目

```sh
go run ./cmd/main.go --migrate --seed
```

`--migrate` 和 `--seed` 参数是可选的，用于数据库迁移和数据初始化。

为了对中文 README.md 进行修改，以下是如何加入新增配置的示例：

---

## 配置说明

### 默认配置

项目默认使用 `/config/default.yaml` 配置文件。你可以通过环境变量以 `token_price_proxy` 为前缀覆盖配置，使用 `_` 替换 `.`，并将包含空格的值拆分成切片。

### 支持的配置项

#### 价格配置

```yaml
price:
  processTime: 10ms
  processTimeOut: 5s
```

- **`processTime`**: 拉取任务间隔时间。在此示例中，任务每 10 毫秒拉取一次。
- **`processTimeOut`**: 任务超时时间。在此示例中，若任务在 5 秒内未完成，则会被超时处理。

#### 禁止数据源配置

```yaml
prohibitedSources:
  current:
    geckoterminal: 1
    defillama: 1
  historical:
    coingecko: 1
```

- **`prohibitedSources`**: 该部分允许配置哪些数据源应被禁止。
  - **`current`**: 禁止用于当前价格查询的数据源。
  - **`historical`**: 禁止用于历史价格查询的数据源。

#### Postgres 配置

在构建项目后，需要配置 Postgres 链接信息：

```yaml
db:
  gorm:
    disable-foreign-key-constraint-when-migrating: true
  postgres:
    dsn: "your-postgres-dsn"
```

#### Redis 配置

需要对 Redis 进行如下配置：

```yaml
redis:
  url: "your-redis-url"
```

#### API Key 配置

配置第三方 API Key：

```yaml
apiKey:
  coingecko: "your-coingecko-key"
  coingeckoOnChain: "your-coingeckoOnChain-key"
  geckoterminal: "your-geckoterminal-key"
```

### 环境变量配置

以下是支持的环境变量配置项，详情见对应说明：

- `CHAIN_MAPPING`: 配置服务价格查询支持的链和对应的 network 名称，推荐默认配置如下：
  ```json
  {
    "mainnet": "1",
    "ethereum": "1",
    "ethereum-mainnet": "1",
    "bsc": "56",
    "polygon": "137",
    "heco": "128",
    "arbitrum": "42161",
    "moonriver": "1285",
    "okex-chain": "66",
    "boba": "288",
    "aurora": "1313161554",
    "rinkeby": "4",
    "kovan": "69",
    "avalanche": "43114",
    "avax": "43114",
    "optimism": "10",
    "cronos": "25",
    "arb-rinkeby": "421611",
    "goerli": "5",
    "gor": "5",
    "kcc": "321",
    "kcs": "321",
    "base-goerli": "84531",
    "basegor": "84531",
    "conflux": "1030",
    "cfx": "1030",
    "scroll-alpha": "534353",
    "scr-alpha": "534353",
    "base": "8453",
    "base-mainnet": "8453",
    "scroll-sepolia": "534351",
    "scr-sepolia": "534353",
    "linea": "59144",
    "scr": "534352",
    "scroll": "534352",
    "scroll-mainnet": "534352",
    "manta": "169",
    "mantle": "5000",
    "merlin": "4200",
    "merlin-chain": "4200",
    "merlin-testnet": "686868",
    "okchain": "66",
    "tokb": "195",
    "x1": "196",
    "sepolia": "11155111",
    "dodochain-testnet": "53457",
    "bitlayer": "200901",
    "btr": "200901",
    "zircuit-testnet": "48899",
    "arbitrum-sepolia": "421614",
    "arb-sep": "421614",
    "zircuit-mainnet": "48900",
    "okb": "196"
  }
  ```
- `ALLOW_API_KEY`: 允许不配置 API Key 进行接口访问，默认允许。
- `USDT_ADDRESSES`: 配置每条链默认的 USDT 价格地址，供 Dodoex Route 询价使用。推荐默认配置如下：
  ```json
  {
    "1": {
      "address": "0xdac17f958d2ee523a2206206994597c13d831ec7",
      "decimal": 6
    },
    "56": {
      "address": "0x55d398326f99059ff775485246999027b3197955",
      "decimal": 18
    },
    "42161": {
      "address": "0xfd086bc7cd5c481dcc9c85ebe478a1c0b69fcbb9",
      "decimal": 6
    },
    "10": {
      "address": "0x94b008aa00579c1307b0ef2c499ad98a8ce58e58",
      "decimal": 6
    },
    "137": {
      "address": "0xc2132d05d31c914a87c6611c10748aeb04b58e8f",
      "decimal": 6
    },
    "288": {
      "address": "0x5DE1677344D3Cb0D7D465c10b72A8f60699C062d",
      "decimal": 6
    },
    "43114": {
      "address": "0x9702230a8ea53601f5cd2dc00fdbc13d4df4a8c7",
      "decimal": 6
    },
    "1285": {
      "address": "0xb44a9b6905af7c801311e8f4e76932ee959c663c",
      "decimal": 6
    },
    "1030": {
      "address": "0xfe97e85d13abd9c1c33384e796f10b73905637ce",
      "decimal": 18
    }
  }
  ```
- `DODOEX_ROUTE_URL`: 配置 Dodoex Route 请求接口地址，默认为 `https://api.dodoex.io/route-service/v2/backend/swap`。
- `GECKO_CHAIN_ALLOWED_TOKENS`: 配置允许查询的 Token 名称，默认为 `*USD* DAI *DODO JOJO *BTC* *ETH* *MATIC* *BNB* *AVAX *NEAR *XRP TON* *ARB ENS`。
- `REFUSE_CHAIN_IDS`: 配置拒绝查询的链，返回价格为 `nil`，默认配置为 `chainId 128`。

## 使用介绍

DODOEX Price API v2 提供了一系列 RESTful API 接口，用于获取 Token 的当前价格、历史价格、币种列表的管理，以及应用 Token 的管理。以下是这些接口的详细说明和使用示例。

### 获取当前价格

**GET /api/v1/price**

获取某个 Token 的当前价格。

#### 请求示例

```bash
curl -X GET "http://localhost:8080/api/v1/price/current?network=ethereum&address=0x6b175474e89094c44da98b954eedeac495271d0f&symbol=DAI&isCache=true&excludeRoute=true"
```

### 请求参数

- `network`: 必填，网络名称，如 `ethereum`。
- `address`: 必填，Token 的合约地址。
- `symbol`: 可选，Token 的符号，如 `DAI`。
- `isCache`: 可选，是否使用缓存，默认为 `true`。
- `excludeRoute`: 可选，是否排除 Route，默认为 `true`。

### 响应示例

```bash
{
  "code": 0,
  "data": "1.01",
  "message": "请求成功"
}
```

### 获取历史价格

**GET /api/v1/price/historical**

获取某个 Token 在指定日期的价格。

#### 请求示例

```bash
curl -X GET "http://localhost:8080/api/v1/price/historical?network=ethereum&address=0x6b175474e89094c44da98b954eedeac495271d0f&symbol=DAI&date=2023-01-01"
```

#### 参数说明

- `network`: 必填，网络名称，如 `ethereum`。
- `address`: 必填，Token 的合约地址。
- `symbol`: 可选，Token 的符号，如 `DAI`。
- `date`: 必填，日期，可以是 `YYYY-MM-DD` 格式或 UNIX 时间戳。

#### 响应示例

```json
{
  "code": 0,
  "data": "1.01",
  "message": "请求成功"
}
```

### 获取批量当前价格

**POST /api/v1/price/current/batch**

获取多个 Token 的当前价格。

#### 请求示例

```bash
curl -X POST "http://localhost:8080/api/v1/price/current/batch" \
-H "Content-Type: application/json" \
-d '{
  "addresses": ["0x6b175474e89094c44da98b954eedeac495271d0f"],
  "networks": ["ethereum"],
  "symbols": ["DAI"],
  "isCache": true,
  "excludeRoute": true
}'
```

#### 参数说明

- `addresses`: 必填，Token 的合约地址数组。
- `networks`: 必填，网络名称数组，与 `addresses` 对应。
- `symbols`: 可选，Token 的符号数组，与 `addresses` 对应。
- `isCache`: 可选，是否使用缓存，默认为 `true`。
- `excludeRoute`: 可选，是否排除 Route，默认为 `true`。

#### 响应示例

```json
{
  "code": 0,
  "data": [
    {
      "price": 1.01,
      "symbol": "DAI",
      "network": "ethereum",
      "chainId": "1",
      "address": "0x6b175474e89094c44da98b954eedeac495271d0f",
      "date": "1725289355",
      "serial": 0
    }
  ],
  "message": "请求成功"
}
```

### 获取批量历史价格

**POST /api/v1/price/historical/batch**

获取多个 Token 在多个日期的历史价格。

#### 请求示例

```bash
curl -X POST "http://localhost:8080/api/v1/price/historical/batch" \
-H "Content-Type: application/json" \
-d '{
  "addresses": ["0x6b175474e89094c44da98b954eedeac495271d0f"],
  "networks": ["ethereum"],
  "symbols": ["DAI"],
  "dates": ["2023-01-01"]
}'
```

#### 参数说明

- `addresses`: 必填，Token 的合约地址数组。
- `networks`: 必填，网络名称数组，与 `addresses` 对应。
- `symbols`: 可选，Token 的符号数组，与 `addresses` 对应。
- `dates`: 必填，日期数组，可以是 `YYYY-MM-DD` 格式或 UNIX 时间戳。

#### 响应示例

```json
{
  "code": 0,
  "data": [
    {
      "chainId": "1",
      "address": "0x6b175474e89094c44da98b954eedeac495271d0f",
      "price": "1.02",
      "symbol": "DAI",
      "network": "ethereum",
      "date": "2023-01-01",
      "serial": 0
    }
  ],
  "message": "请求成功"
}
```

### 添加币种

**POST /coins/add**

添加新的币种到系统中。

#### 请求示例

```bash
curl -X POST "http://localhost:8080/coins/add" \
-H "Content-Type: application/json" \
-d '{
  "chain_id": "1",
  "address": "0x1234567890abcdef1234567890abcdef12345678",
  "id": "1_0x1234567890abcdef1234567890abcdef12345678"
}'
```

#### 响应示例

```json
{
  "code": 200,
  "message": "添加币种成功"
}
```

### 更新币种

**POST /coins/update/{id}**

更新已存在的币种信息。

#### 请求示例

```bash
curl -X POST "http://localhost:8080/coins/update/1_0x1234567890abcdef1234567890abcdef12345678" \
-H "Content-Type: application/json" \
-d '{
  "chain_id": "1",
  "address": "0x1234567890abcdef1234567890abcdef12345678",
}'
```

#### 响应示例

```json
{
  "code": 200,
  "message": "更新币种成功"
}
```

### 删除币种

**POST /coins/delete/{id}**

删除指定的币种。

#### 请求示例

```bash
curl -X POST "http://localhost:8080/coins/delete/1_0x1234567890abcdef1234567890abcdef12345678"
```

#### 响应示例

```json
{
  "code": 200,
  "message": "删除币种成功"
}
```

### 刷新所有币种缓存

**GET /coins/refresh**

刷新所有币种的缓存。

#### 请求示例

```bash
curl -X GET "http://localhost:8080/coins/refresh"
```

#### 响应示例

```json
{
  "code": 200,
  "message": "刷新缓存成功"
}
```

### 删除 Redis 缓存键

**POST /redis/delete/{key}**

删除指定的 Redis 缓存键。

#### 请求示例

```bash
curl -X POST "http://localhost:8080/redis/delete/{key}"
```

#### 响应示例

```json
{
  "code": 200,
  "message": "删除redis成功"
}
```

---

以上内容展示了 DODOEX Price API v2 的核心功能及其对应的 RESTful API 接口。使用这些接口，用户可以高效地管理币种数据，获取 Token 的实时和历史价格，并管理应用 Token 和缓存等相关信息。

## 其他注意事项

在构建和运行项目时，请确保正确配置以下关键项：

- **Postgres 配置**: 需要正确设置 Postgres 数据库的连接信息，以确保数据库操作正常。
- **Redis 配置**: Redis 是本项目的核心组件之一，用于管理队列、缓存等功能，请确保 Redis 服务配置正确。
- **API Key 配置**: 由于本项目依赖多个第三方数据源，务必配置相应的 API Key，以确保数据能够正确获取。
- **环境变量配置**: 正确设置环境变量，如 `CHAIN_MAPPING`、`USDT_ADDRESSES` 等，以确保系统能够正确处理不同区块链网络的请求。

以下是项目运行时常见的环境变量配置示例：

```bash
export CHAIN_MAPPING='{
	"mainnet": "1",
	"ethereum": "1",
	"ethereum-mainnet": "1",
	"bsc": "56",
	"polygon": "137",
	"heco": "128",
	"arbitrum": "42161",
	"moonriver": "1285",
	"okex-chain": "66",
	"boba": "288",
	"aurora": "1313161554",
	"rinkeby": "4",
	"kovan": "69",
	"avalanche": "43114",
	"avax": "43114",
	"optimism": "10",
	"cronos": "25",
	"arb-rinkeby": "421611",
	"goerli": "5",
	"gor": "5",
	"kcc": "321",
	"kcs": "321",
	"base-goerli": "84531",
	"basegor": "84531",
	"conflux": "1030",
	"cfx": "1030",
	"scroll-alpha": "534353",
	"scr-alpha": "534353",
	"base": "8453",
	"base-mainnet": "8453",
	"scroll-sepolia": "534351",
	"scr-sepolia": "534353",
	"linea": "59144",
	"scr": "534352",
	"scroll": "534352",
	"scroll-mainnet": "534352",
	"manta": "169",
	"mantle": "5000",
	"merlin": "4200",
	"merlin-chain": "4200",
	"merlin-testnet": "686868",
	"okchain": "66",
	"tokb": "195",
	"x1": "196",
	"sepolia": "11155111",
	"dodochain-testnet": "53457",
	"bitlayer": "200901",
	"btr": "200901",
	"zircuit-testnet": "48899",
	"arbitrum-sepolia": "421614",
	"arb-sep": "421614",
	"zircuit-mainnet": "48900",
    "okb": "196"
}'
export USDT_ADDRESSES='{"1":{"address":"0xdac17f958d2ee523a2206206994597c13d831ec7","decimal":6},"56":{"address":"0x55d398326f99059ff775485246999027b3197955","decimal":18},"42161":{"address":"0xfd086bc7cd5c481dcc9c85ebe478a1c0b69fcbb9","decimal":6},"10":{"address":"0x94b008aa00579c1307b0ef2c499ad98a8ce58e58","decimal":6},"137":{"address":"0xc2132d05d31c914a87c6611c10748aeb04b58e8f","decimal":6},"288":{"address":"0x5DE1677344D3Cb0D7D465c10b72A8f60699C062d","decimal":6},"43114":{"address":"0x9702230a8ea53601f5cd2dc00fdbc13d4df4a8c7","decimal":6},"1285":{"address":"0xb44a9b6905af7c801311e8f4e76932ee959c663c","decimal":6},"1030":{"address":"0xfe97e85d13abd9c1c33384e796f10b73905637ce","decimal":18}}'
export DODOEX_ROUTE_URL="https://api.dodoex.io/route-service/v2/backend/swap"
export GECKO_CHAIN_ALLOWED_TOKENS="*USD* DAI *DODO JOJO *BTC* *ETH* *MATIC* *BNB* *AVAX *NEAR *XRP TON* *ARB ENS"
export REFUSE_CHAIN_IDS="128"
```

务必根据实际情况调整这些配置项，以确保项目运行的稳定性和安全性。
