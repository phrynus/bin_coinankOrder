package main

import (
	"encoding/json"
	"log"
	"os"
)

var config Config

type Config struct {
	ApiKey            string   `json:"apiKey"`
	ApiSecret         string   `json:"apiSecret"`
	Proxy             string   `json:"proxy"`
	Timeout           int      `json:"timeout"`           // 网络超时
	Duration          int      `json:"duration"`          // 挂单超时 s
	MaxCoins          int      `json:"maxCoins"`          // 前几个币种
	PriceDepth        int      `json:"priceDepth"`        // orderBook挂单位置
	RsiLength         int      `json:"rsiLength"`         // RSI 长度
	RsiLevel          float64  `json:"rsiLevel"`          // RSI 标准值
	Amount            float64  `json:"amount"`            // 挂单金额
	BuyNetAmount      float64  `json:"buyNetAmount"`      // 多单挂单量
	SideNetAmount     float64  `json:"sideNetAmount"`     // 空单挂单量
	MultipleNetAmount float64  `json:"multipleNetAmount"` // 挂单量倍数
	Blacklist         []string `json:"blacklist"`         // 黑名单
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
