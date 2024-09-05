-- app_tokens 表
CREATE TABLE app_tokens (
    name       TEXT NOT NULL,
    token      TEXT NOT NULL,
    rate       REAL NOT NULL,
    id         BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

-- 添加索引
CREATE INDEX idx_app_tokens_token ON app_tokens (token);
CREATE INDEX idx_app_tokens_deleted_at ON app_tokens (deleted_at);

-- coin_historical_prices 表
CREATE TABLE coin_historical_prices (
    coin_id    VARCHAR(255) NOT NULL,
    date       BIGINT NOT NULL,
    day_date   VARCHAR(255) NOT NULL,
    price      VARCHAR(255) NOT NULL,
    source     VARCHAR(255) DEFAULT ''::character varying NOT NULL,
    query_info JSON,
    id         BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT unique_coin_id_day_date UNIQUE (coin_id, day_date)
);

-- 添加索引
CREATE INDEX idx_coin_historical_prices_coin_id_day_date ON coin_historical_prices (coin_id, day_date);
CREATE INDEX idx_coin_historical_prices_day_date ON coin_historical_prices (day_date);

-- coins 表
CREATE TABLE coins (
    id                    VARCHAR(255) NOT NULL PRIMARY KEY,
    address               VARCHAR(255) NOT NULL,
    chain_id              VARCHAR(255) NOT NULL,
    symbol                VARCHAR(255),
    name                  VARCHAR(255),
    coingecko_coin_id     VARCHAR(255),
    coingecko_platforms   JSON,
    geckoterminal_network VARCHAR(255),
    extra                 JSON,
    decimals              BIGINT,
    total_supply          VARCHAR(255),
    label                 VARCHAR(255) DEFAULT ''::character varying NOT NULL,
    pool_name             VARCHAR(255) DEFAULT ''::character varying,
    base_token_address    VARCHAR(255) DEFAULT ''::character varying,
    quote_token_address   VARCHAR(255) DEFAULT ''::character varying,
    pool_created_at       TIMESTAMPTZ,
    pool_attributes       JSON,
    last_price_source     VARCHAR(255),
    price_source          VARCHAR(255),
    return_coins_id       VARCHAR(255),
    created_at            TIMESTAMPTZ,
    updated_at            TIMESTAMPTZ,
    deleted_at            TIMESTAMPTZ
);

-- 添加索引
CREATE INDEX idx_coins_address ON coins (address);
CREATE INDEX idx_coins_chain_id ON coins (chain_id);
CREATE INDEX idx_coins_symbol ON coins (symbol);
CREATE INDEX idx_coins_deleted_at ON coins (deleted_at);
CREATE INDEX idx_coins_return_coins_id ON coins (return_coins_id);


-- base network
INSERT INTO coins (id, address, chain_id,symbol,name, return_coins_id, created_at, updated_at)
VALUES
    ('8453_0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '8453','','' ,'8453_0x4200000000000000000000000000000000000006', NOW(), NOW());

-- linea network
INSERT INTO coins (id, address, chain_id,symbol,name, return_coins_id, created_at, updated_at)
VALUES
    ('59144_0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '59144','','', '59144_0xe5d7c2a44ffddf6b295a15c148167daaaf5cf34f', NOW(), NOW());

-- scr network
INSERT INTO coins (id, address, chain_id,symbol,name, return_coins_id, created_at, updated_at)
VALUES
    ('534352_0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '534352','','', '534352_0x5300000000000000000000000000000000000004', NOW(), NOW());

-- manta network
INSERT INTO coins (id, address, chain_id,symbol,name, return_coins_id, created_at, updated_at)
VALUES
    ('169_0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '169','','', '169_0x0dc808adce2099a9f62aa87d9670745aba741746', NOW(), NOW());

-- mantle network
INSERT INTO coins (id, address, chain_id,symbol,name, return_coins_id, created_at, updated_at)
VALUES
    ('5000_0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '5000','','', '5000_0x78c1b0c915c4faa5fffa6cabf0219da63d7f4cb8', NOW(), NOW());

-- merlin network (0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee)
INSERT INTO coins (id, address, chain_id,symbol,name, return_coins_id, created_at, updated_at)
VALUES
    ('4200_0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '4200','','', '4200_0xf6d226f9dc15d9bb51182815b320d3fbe324e1ba', NOW(), NOW());

-- merlin network (0xb5d8b1e73c79483d7750c5b8df8db45a0d24e2cf)
INSERT INTO coins (id, address, chain_id,symbol,name, return_coins_id, created_at, updated_at)
VALUES
    ('4200_0xb5d8b1e73c79483d7750c5b8df8db45a0d24e2cf', '0xb5d8b1e73c79483d7750c5b8df8db45a0d24e2cf', '4200','STONE','stakeStone Ether', 'eth_0x7122985656e38bdc0302db86685bb972b145bd3c', NOW(), NOW());

-- arbitrum network
-- INSERT INTO coins (id, address, chain_id,symbol,name, return_coins_id, created_at, updated_at)
-- VALUES
--     ('42161_0xafafd68afe3fe65d376eec9eab1802616cfaccb8', '0xafafd68afe3fe65d376eec9eab1802616cfaccb8', '42161','SolvBTC','Solv BTC', '42161_0x3647c54c4c2c65bc7a2d63c0da2809b399dbbdc0', NOW(), NOW());

-- arb network
INSERT INTO coins (id, address, chain_id, symbol, name, return_coins_id, created_at, updated_at)
VALUES
    ('42161_0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '42161', '', '', '42161_0x82af49447d8a07e3bd95bd0d56f35241523fbab1', NOW(), NOW())
ON CONFLICT (id)
DO UPDATE SET return_coins_id = EXCLUDED.return_coins_id, updated_at = EXCLUDED.updated_at;

-- bsc network wbnb
INSERT INTO coins (id, address, chain_id, symbol, name, return_coins_id, created_at, updated_at)
VALUES
    ('56_0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '56', '', '', '56_0xbb4cdb9cbd36b01bd1cbaebf2de08d9173bc095c', NOW(), NOW())
ON CONFLICT (id)
DO UPDATE SET return_coins_id = EXCLUDED.return_coins_id, updated_at = EXCLUDED.updated_at;

-- avax network wavax
INSERT INTO coins (id, address, chain_id, symbol, name, return_coins_id, created_at, updated_at)
VALUES
    ('43114_0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '43114', '', '', '43114_0xb31f66aa3c1e785363f0875a1b74e27b85fd66c7', NOW(), NOW())
ON CONFLICT (id)
DO UPDATE SET return_coins_id = EXCLUDED.return_coins_id, updated_at = EXCLUDED.updated_at;

-- eth network weth
INSERT INTO coins (id, address, chain_id, symbol, name, return_coins_id, created_at, updated_at)
VALUES
    ('1_0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '1', '', '', '1_0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2', NOW(), NOW())
ON CONFLICT (id)
DO UPDATE SET return_coins_id = EXCLUDED.return_coins_id, updated_at = EXCLUDED.updated_at;

-- polygon network wmatic
INSERT INTO coins (id, address, chain_id, symbol, name, return_coins_id, created_at, updated_at)
VALUES
    ('137_0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '137', '', '', '137_0x0d500b1d8e8ef31e21c99d1db9a6444d3adf1270', NOW(), NOW())
ON CONFLICT (id)
DO UPDATE SET return_coins_id = EXCLUDED.return_coins_id, updated_at = EXCLUDED.updated_at;

-- dodochain test network USDT USDT ETH
INSERT INTO coins (id, address, chain_id, symbol, name, return_coins_id, created_at, updated_at)
VALUES
(
    '53457_0x668d4ed054aa62d12f95a64aa1c7e2791f176d0e', 
    '0x668d4ed054aa62d12f95a64aa1c7e2791f176d0e', 
    '53457', 
    'DFT1', 
    'Token1 DFT', 
    '1_0xdac17f958d2ee523a2206206994597c13d831ec7', 
    NOW(), 
    NOW()
),
(
    '11155111_0x2b36c1be2a16acb71e6f6cccfcd7d20cdfe01867', 
    '0x2b36c1be2a16acb71e6f6cccfcd7d20cdfe01867', 
    '11155111', 
    'TK1', 
    'token1', 
    '1_0xdac17f958d2ee523a2206206994597c13d831ec7', 
    NOW(), 
    NOW()
),
(
    '11155111_0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', 
    '0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', 
    '11155111', 
    'ETH', 
    'ETH', 
    '1_0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2', 
    NOW(), 
    NOW()
);


CREATE TABLE request_logs (
    ip_address VARCHAR(45),
    endpoint VARCHAR(255),
    request_params TEXT,
    response TEXT,
    execution_time BIGINT,
    created_at            TIMESTAMPTZ,
    updated_at            TIMESTAMPTZ,
    deleted_at            TIMESTAMPTZ
);

CREATE INDEX idx_request_logs_created_at ON request_logs(created_at);
CREATE INDEX idx_request_logs_endpoint ON request_logs(endpoint);

CREATE TABLE slack_notifications (
                                     source VARCHAR(255) NOT NULL,
                                     coin_id VARCHAR(255) NOT NULL,
                                     day_date VARCHAR(255) NOT NULL,
                                     date BIGINT NOT NULL,
                                     counter INT DEFAULT 1 NOT NULL,
                                     created_at TIMESTAMPTZ,
                                     updated_at TIMESTAMPTZ,
                                     deleted_at TIMESTAMPTZ,
                                     CONSTRAINT unique_notification UNIQUE (coin_id, day_date)
);

UPDATE coins
SET price_source = 'coingecko'
WHERE coingecko_coin_id IS NOT NULL;

-- INSERT INTO coins (id, address, chain_id, symbol, name, return_coins_id, created_at, updated_at)
-- VALUES
--     ('534352_0x912ab742e1ab30ffa87038c425f9bc8ed12b3ef4', '0x912ab742e1ab30ffa87038c425f9bc8ed12b3ef4', '534352', '', '', '1_0x43dfc4159d86f3a37a5a4b3d4580b888ad7d4ddd', NOW(), NOW())
-- ON CONFLICT (id)
-- DO UPDATE SET return_coins_id = EXCLUDED.return_coins_id, updated_at = EXCLUDED.updated_at;

INSERT INTO coins (id, address, chain_id, symbol, name, return_coins_id, created_at, updated_at)
VALUES
    ('421614_0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '421614', '', '', '1_0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2', NOW(), NOW())
ON CONFLICT (id)
DO UPDATE SET return_coins_id = EXCLUDED.return_coins_id, updated_at = EXCLUDED.updated_at;

INSERT INTO coins (id, address, chain_id, symbol, name, return_coins_id, created_at, updated_at)
VALUES
    ('421614_0x980b62da83eff3d4576c647993b0c1d7faf17c73', '0x980b62da83eff3d4576c647993b0c1d7faf17c73', '421614', '', '', '1_0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2', NOW(), NOW())
ON CONFLICT (id)
DO UPDATE SET return_coins_id = EXCLUDED.return_coins_id, updated_at = EXCLUDED.updated_at;


INSERT INTO coins (id, address, chain_id, symbol, name, return_coins_id, created_at, updated_at)
VALUES
    ('48900_0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', '48900', '', '', '1_0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2', NOW(), NOW())
ON CONFLICT (id)
DO UPDATE SET return_coins_id = EXCLUDED.return_coins_id, updated_at = EXCLUDED.updated_at;

INSERT INTO coins (id, address, chain_id, symbol, name, return_coins_id, created_at, updated_at)
VALUES
    ('48900_0x4200000000000000000000000000000000000006', '0x4200000000000000000000000000000000000006', '48900', '', '', '1_0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2', NOW(), NOW())
ON CONFLICT (id)
DO UPDATE SET return_coins_id = EXCLUDED.return_coins_id, updated_at = EXCLUDED.updated_at;