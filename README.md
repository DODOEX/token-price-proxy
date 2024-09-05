<div align="center" id="top">
<img alt="logo" width="80" height="80" style="border-radius: 20px" src="docs/icon.png"/>
</div>
<h1 align="center">Token Price Proxy</h1>
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
  <a href="docs/README_ZH.md">中文</a>
  &#xa0;|&#xa0;
  <a href="README.md">English</a>
</div>
</br>

## Project Overview

Token Price Proxy aims to aggregate multiple price platforms to obtain Token prices. It currently integrates multiple data platforms and the Dodoex Route platform, supporting both current and historical price queries.

## Features

- **Multi-platform Price Retrieval and Processing**: Supports price retrieval from multiple major platforms (CoinGecko, GeckoTerminal, DefiLlama, Dodoex Route, CoinGecko OnChain). Prices are cached and queued using Redis, and users can disable specific data sources based on configuration.
- **Redis Integration and Request Management**: Manages price requests using Redis queues and Lua scripts, ensuring uniqueness and sequential processing, and improving overall system efficiency through deduplication and batch processing.
- **Result Subscription and Asynchronous Push**: Uses Redis's Pub/Sub model to push processed price results to the requester, supporting asynchronous processing of large batches of requests.
- **Throttling and Error Management**: Implements throttling to prevent excessive resource consumption from frequent queries, and logs errors in detail when price retrieval fails, ensuring system stability.
- **Flexible Configuration and Custom Options**: Supports loading configurations through the Koanf library, allowing users to customize price sources, throttling settings, and Redis-related parameters to meet different application needs.
- **Manual Configuration and Automatic Selection of Price Data Sources**: Users can manually configure price data sources or choose to use the last successful source automatically to improve query efficiency.
- **Efficient Redis Queue Management and Data Updates**: Manages price requests through Redis queues, ensuring uniqueness and sequential processing via Lua scripts, greatly improving data update efficiency.
- **API Rate Limiting and Access Control**: Implements API rate limiting using Redis Lua scripts, ensuring stable operation under high concurrency. Supports access control via API keys to enhance interface security.
- **Token Query Association**: Supports associated Token queries, ensuring accurate price retrieval for tokens on less active chains, expanding the coverage of price retrieval.
- **Scheduled Token List and Redis Queue Updates**: Periodically updates CoinGecko's token list and requests in the Redis queue, ensuring that data in the system remains up-to-date and accurate.

## Installation and Running

### Run Initialization SQL Script

Before starting the project, it is recommended to run the initialization SQL script to set up the initial state of the database. You can execute the script using the following command:

```sh
psql -U <username> -d <database_name> -f /sql/init.sql
```

### Clone the Project

```sh
git clone https://github.com/DODOEX/token-price-proxy
cd token-price-proxy
```

### Build the Project

```sh
go build ./cmd/main.go
```

### Run the Project

```sh
go run ./cmd/main.go --migrate --seed
```

The `--migrate` and `--seed` parameters are optional, used for database migration and data seeding.

Here is the corresponding English README section with the new configuration details:

---

## Configuration Instructions

### Default Configuration

The project uses the `/config/default.yaml` configuration file by default. You can override configurations through environment variables prefixed with `token_price_proxy`, replacing `.` with `_`, and splitting values with spaces into a slice.

### Supported Configuration Items

#### Price Configuration

```yaml
price:
  processTime: 10ms
  processTimeOut: 5s
```

- **`processTime`**: The interval between task fetches. In this example, tasks are fetched every 10 milliseconds.
- **`processTimeOut`**: The timeout duration for a task. In this example, if a task is not completed within 5 seconds, it will be timed out.

#### Prohibited Data Sources Configuration

```yaml
prohibitedSources:
  current:
    geckoterminal: 1
    defillama: 1
  historical:
    coingecko: 1
```

- **`prohibitedSources`**: This section allows you to configure which data sources should be prohibited.
  - **`current`**: Data sources to be prohibited for current price queries.
  - **`historical`**: Data sources to be prohibited for historical price queries.

#### Postgres Configuration

After building the project, configure the Postgres connection information:

```yaml
db:
  gorm:
    disable-foreign-key-constraint-when-migrating: true
  postgres:
    dsn: "your-postgres-dsn"
```

#### Redis Configuration

Configure Redis as follows:

```yaml
redis:
  url: "your-redis-url"
```

#### API Key Configuration

Configure third-party API keys:

```yaml
apiKey:
  coingecko: "your-coingecko-key"
  coingeckoOnChain: "your-coingeckoOnChain-key"
  geckoterminal: "your-geckoterminal-key"
```

### Environment Variable Configuration

Below are the supported environment variables. Refer to the corresponding instructions for details:

- `CHAIN_MAPPING`: Configure the chains and their corresponding network names supported for price queries. The recommended default configuration is as follows:
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
- `ALLOW_API_KEY`: Allows access to the API without configuring an API key, enabled by default.
- `USDT_ADDRESSES`: Configure the default USDT price addresses for each chain, used by Dodoex Route for price queries. The recommended default configuration is as follows:
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
- `DODOEX_ROUTE_URL`: Configure the Dodoex Route request interface URL, defaulting to `https://api.dodoex.io/route-service/v2/backend/swap`.
- `GECKO_CHAIN_ALLOWED_TOKENS`: Configure the tokens allowed for querying, with the default being `*USD* DAI *DODO JOJO *BTC* *ETH* *MATIC* *BNB* *AVAX *NEAR *XRP TON* *ARB ENS`.
- `REFUSE_CHAIN_IDS`: Configure chain IDs that should be refused for querying, where the returned price will be `nil`. The default configuration is `chainId 128`.

## Usage Guide

DODOEX Price API v2 offers a range of RESTful API endpoints to retrieve current token prices, historical prices, manage token lists, and manage application tokens. Below is a detailed explanation of these endpoints and usage examples.

### Retrieve Current Price

**GET /api/v1/price**

Retrieve the current price of a specific token.

#### Request Example

```bash
curl -X GET "http://localhost:8080/api/v1/price/current?network=ethereum&address=0x6b175474e89094c44da98b954eedeac495271d0f&symbol=DAI&isCache=true&excludeRoute=true"
```

### Request Parameters

- `network`: Required, the network name, such as `ethereum`.
- `address`: Required, the contract address of the token.
- `symbol`: Optional, the symbol of the token, such as `DAI`.
- `isCache`: Optional, whether to use the cache, default is `true`.
- `excludeRoute`: Optional, whether to exclude Route, default is `true`.

### Response Example

```bash
{
  "code": 0,
  "data": "1.01",
  "message": "Request successful"
}
```

### Retrieve Historical Price

**GET /api/v1/price/historical**

Retrieve the price of a specific token on a given date.

#### Request Example

```bash
curl -X GET "http://localhost:8080/api/v1/price/historical?network=ethereum&address=0x6b175474e89094c44da98b954eedeac495271d0f&symbol=DAI&date=2023-01-01"
```

#### Parameter Description

- `network`: Required, the network name, such as `ethereum`.
- `address`: Required, the contract address of the token.
- `symbol`: Optional, the symbol of the token, such as `DAI`.
- `date`: Required, the date, which can be in `YYYY-MM-DD` format or a UNIX timestamp.

#### Response Example

```json
{
  "code": 0,
  "data": "1.01",
  "message": "Request successful"
}
```

### Retrieve Batch Current Prices

**POST /api/v1/price/current/batch**

Retrieve the current prices of multiple tokens.

#### Request Example

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

#### Parameter Description

- `addresses`: Required, an array of token contract addresses.
- `networks`: Required, an array of network names corresponding to `addresses`.
- `symbols`: Optional, an array of token symbols corresponding to `addresses`.
- `isCache`: Optional, whether to use the cache, default is `true`.
- `excludeRoute`: Optional, whether to exclude Route, default is `true`.

#### Response Example

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
  "message": "Request successful"
}
```

### Retrieve Batch Historical Prices

**POST /api/v1/price/historical/batch**

Retrieve historical prices for multiple tokens across multiple dates.

#### Request Example

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

#### Parameter Description

- `addresses`: Required, an array of token contract addresses.
- `networks`: Required, an array of network names corresponding to `addresses`.
- `symbols`: Optional, an array of token symbols corresponding to `addresses`.
- `dates`: Required, an array of dates, which can be in `YYYY-MM-DD` format or UNIX timestamps.

#### Response Example

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
  "message": "Request successful"
}
```

### Add Token

**POST /coins/add**

Add a new token to the system.

#### Request Example

```bash
curl -X POST "http://localhost:8080/coins/add" \
-H "Content-Type: application/json" \
-d '{
  "chain_id": "1",
  "address": "0x1234567890abcdef1234567890abcdef12345678",
  "id": "1_0x1234567890abcdef1234567890abcdef12345678"
}'
```

#### Response Example

```json
{
  "code": 200,
  "message": "Token added successfully"
}
```

### Update Token

**POST /coins/update/{id}**

Update the information of an existing token.

#### Request Example

```bash
curl -X POST "http://localhost:8080/coins/update/1_0x1234567890abcdef1234567890abcdef12345678" \
-H "Content-Type: application/json" \
-d '{
  "chain_id": "1",
  "address": "0x1234567890abcdef1234567890abcdef12345678",
}'
```

#### Response Example

```json
{
  "code": 200,
  "message": "Token updated successfully"
}
```

### Delete Token

**POST /coins/delete/{id}**

Delete a specific token.

#### Request Example

```bash
curl -X POST "http://localhost:8080/coins/delete/1_0x1234567890abcdef1234567890abcdef12345678"
```

#### Response Example

```json
{
  "code": 200,
  "message": "Token deleted successfully"
}
```

### Refresh All Tokens Cache

**GET /coins/refresh**

Refresh the cache for all tokens.

#### Request Example

```bash
curl -X GET "http://localhost:8080/coins/refresh"
```

#### Response Example

```json
{
  "code": 200,
  "message": "Cache refreshed successfully"
}
```

### Delete Redis Cache Key

**POST /redis/delete/{key}**

Delete a specific Redis cache key.

#### Request Example

```bash
curl -X POST "http://localhost:8080/redis/delete/{key}"
```

#### Response Example

```json
{
  "code": 200,
  "message": "Redis key deleted successfully"
}
```

---

The above content showcases the core features of DODOEX Price API v2 and its corresponding RESTful API endpoints. By using these endpoints, users can efficiently manage token data, retrieve real-time and historical token prices, and manage application tokens and related cache information.

## Additional Considerations

When building and running the project, ensure that the following key items are properly configured:

- **Postgres Configuration**: Make sure to correctly configure the Postgres database connection information to ensure smooth database operations.
- **Redis Configuration**: Redis is a core component of this project, used for managing queues, caching, and more. Ensure that Redis is correctly configured.
  Continuing with the English translation:
- **API Key Configuration**: Since the project relies on multiple third-party data sources, make sure to configure the corresponding API keys to ensure proper data retrieval.
- **Environment Variable Configuration**: Correctly set environment variables such as `CHAIN_MAPPING`, `USDT_ADDRESSES`, etc., to ensure the system can correctly process requests across different blockchain networks.

Below is a common example of environment variable configurations when running the project:

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

Make sure to adjust these configuration items according to your actual situation to ensure the stability and security of the project operation.
