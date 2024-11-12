package main

import (
	"encoding/json"
	"log"
	"os"
)

var config Config

type Config struct {
	ApiKey                string   `json:"apiKey"`
	ApiSecret             string   `json:"apiSecret"`
	Proxy                 string   `json:"proxy"`
	Duak                  bool     `json:"duak"`                  // 是否使用双向 为假时遇到翻转时直接退出（关联ProfitExit） 为真不处理
	Amount                float64  `json:"amount"`                // 挂单金额
	Timeout               int      `json:"timeout"`               // 网络超时
	RsiLength             int      `json:"rsiLength"`             // RSI 长度
	RsiLevel              float64  `json:"rsiLevel"`              // RSI 标准值
	Duration              int      `json:"duration"`              // 循环时间 s
	MaxCoins              int      `json:"maxCoins"`              // 前几个币种
	PriceDepth            int      `json:"priceDepth"`            // orderBook挂单位置
	ProfitExit            float64  `json:"profitExit"`            // 盈利退出 大于0时多空翻转判断盈利金额是否达标，不达标则不会退出（需要关闭双向 Duak）
	OrdersTimeout         int64    `json:"ordersTimeout"`         // 挂单超时退出时间
	BuyNetAmount          float64  `json:"buyNetAmount"`          // 多单挂单量
	SideNetAmount         float64  `json:"sideNetAmount"`         // 空单挂单量
	MultipleNetAmount     float64  `json:"multipleNetAmount"`     // 挂单量倍数 5分钟的n倍>15分钟
	MarginUtilizationRate float64  `json:"marginUtilizationRate"` // 仓位使用率
	Blacklist             []string `json:"blacklist"`             // 黑名单
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
