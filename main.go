package main

import (
	"context"
	"log"
	"runtime"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
)

func main() {
	log.Print("Go version:", runtime.Version())

	client := binance.NewFuturesClient(config.ApiKey, config.ApiSecret)
	// 时间偏移
	client.NewSetServerTimeService().Do(context.Background())

	// 获取交易信息
	info, err := client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	res, err := client.NewGetBalanceService().Do(context.Background())
	if err != nil {
		log.Fatal(err)
		return
	}
	for _, b := range res {
		log.Print(b.Asset, ":  ", b.AvailableBalance)
	}

	// 赛选币种
	symbols := make([]futures.Symbol, 0)
	for _, s := range info.Symbols {
		if s.QuoteAsset == "USDT" && s.ContractType == "PERPETUAL" && s.Status == "TRADING" && !contains(config.Blacklist, s.BaseAsset) {
			symbols = append(symbols, s)
		}
	}

	log.Println(symbols[0].Symbol)
}

// 检查一个字符串是否在切片中
func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}
