package main

import (
	"encoding/json"
	"log"
	"os"
)

var config Config

type Config struct {
	Amount     float64  `json:"amount"`
	ApiKey     string   `json:"apiKey"`
	ApiSecret  string   `json:"apiSecret"`
	Duration   int      `json:"duration"`    // 挂单超时 s
	Leverage   int      `json:"leverage"`    // 倍数
	MaxCoins   int      `json:"max_coins"`   // 单边最多挂单
	PriceDepth int      `json:"price_depth"` // orderBook挂单索引
	Blacklist  []string `json:"blacklist"`   // 黑名单
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
