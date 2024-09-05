from locust import HttpUser, task, constant
import random
import json
import time

class PriceUser(HttpUser):
    wait_time = constant(1)  

    token_list = [
        "0xdAC17F958D2ee523a2206206994597C13d831ec7",
        "0x6B175474E89094C44Da98b954EedeAC495271d0F",
        "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
        "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599",
        "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
        "0xB8c77482e45F1F44dE1745F52C74426C631bDD52",
        "0x2170Ed0880ac9A755fd29B2688956BD959F933F8",
        "0x55d398326f99059fF775485246999027B3197955",
        "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c",
        "0x7ceB23fD6bC0adD59E62ac25578270cFf1b9f619",
        "0x3BA4c387f786bFEE076A58914F5Bd38d668B42c3",
        "0x0000000000000000000000000000000000001010",
        "0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270"
    ]

    networks = ["ethereum", "bsc", "polygon"]

    def check_response(self, response, params, headers):
        if response.status_code == 200:
            try:
                data = response.json()
                if data.get("code") == 0:
                    response.success()
                else:
                    response.failure(f"fail , params: {params}, headers: {headers}")
            except json.JSONDecodeError:
                response.failure(f"JSON error, params: {params}, headers: {headers}")
        else:
            response.failure(f"error code: {response.status_code}, params: {params}, headers: {headers}")

    @task
    def get_single_current_price(self):
        address = random.choice(self.token_list)
        network = random.choice(self.networks)
        params = {
            "address": address,
            "network": network,
        }
        with self.client.get("/price/current", params=params, catch_response=True) as response:
            self.check_response(response, params, None)

    # @task
    # def get_single_historical_price(self):
    #     address = random.choice(self.token_list)
    #     network = random.choice(self.networks)
    #     date = int(time.time()) - 86400 * random.randint(1, 30)  # 过去30天内的某一天
    #     params = {
    #         "address": address,
    #         "network": network,
    #         "date": date,
    #     }
    #     with self.client.get("/price/historical", params=params, catch_response=True) as response:
    #         self.check_response(response, params, None)

    @task
    def get_batch_current_price(self):
        addresses = random.sample(self.token_list, 13)
        networks = random.choices(self.networks, k=13)
        if addresses and networks:
            params = {
                "addresses": addresses,
                "networks": networks
            }
            headers = {
                "Content-Type": "application/json",
            }
            with self.client.post("/price/current/batch", json=params, headers=headers, catch_response=True) as response:
                self.check_response(response, params, headers)

    # @task
    # def get_batch_historical_price(self):
    #     addresses = random.sample(self.token_list, 13)
    #     networks = random.choices(self.networks, k=13)
    #     dates = [int(time.time()) - 86400 * random.randint(1, 30) for _ in range(13)]
    #     if addresses and networks and dates:
    #         params = {
    #             "addresses": addresses,
    #             "networks": networks,
    #             "dates": dates
    #         }
    #         headers = {
    #             "Content-Type": "application/json",
    #         }
    #         with self.client.post("/price/historical/batch", json=params, headers=headers, catch_response=True) as response:
    #             self.check_response(response, params, headers)
