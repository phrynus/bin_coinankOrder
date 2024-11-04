package main

import (
	"encoding/json"
	"log"
	"os"
)

var config Config

type Config struct {
	ApiKey        string   `json:"apiKey"`
	ApiSecret     string   `json:"apiSecret"`
	Amount        float64  `json:"amount"`        // 挂单金额
	Duration      int      `json:"duration"`      // 挂单超时 s
	MaxCoins      int      `json:"maxCoins"`      // 单边最多挂单
	PriceDepth    int      `json:"priceDepth"`    // orderBook挂单索引
	BuyNetAmount  float64  `json:"buyNetAmount"`  // 多单挂单额度
	SideNetAmount float64  `json:"sideNetAmount"` // 空单挂单额度
	Blacklist     []string `json:"blacklist"`     // 黑名单
}

func init() {
	b, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(b, &config); err != nil {
		log.Fatal(err)
	}
	// log.Print(config)
}
