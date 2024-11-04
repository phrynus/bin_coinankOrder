package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
)

type FundData struct {
	Coin   string `json:"baseCoin"`
	Side   bool
	M5Net  float64 `json:"m5net"`
	M15Net float64 `json:"m15net"`
}

func main() {
	client := binance.NewFuturesClient(config.ApiKey, config.ApiSecret)
	// 时间偏移
	client.NewSetServerTimeService().Do(context.Background())
	// 获取交易信息
	info, err := client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	// 赛选币种
	symbols := make([]futures.Symbol, 0)
	for _, s := range info.Symbols {
		if s.QuoteAsset == "USDT" && s.ContractType == "PERPETUAL" && s.Status == "TRADING" && !contains(config.Blacklist, s.BaseAsset) {
			symbols = append(symbols, s)
		}
	}
	// log.Println(symbols[0])
	//
	go func() {
		for {
			err := Go(client, symbols)
			if err != nil {
				log.Fatal(err)
			}
			// 等待 s时间
			time.Sleep(time.Duration(config.Duration) * time.Second)
		}
	}()
	//
	select {}
}

func Go(client *futures.Client, symbols []futures.Symbol) error {
	// 判断账户是否有资格
	// if err := isAccount(client); err != nil {
	// 	log.Println(err)
	// 	return nil
	// }

	coinank, err := fetchFundCoinankData()
	if err != nil {
		log.Println(err)
		return nil
	}
	symbolsGo, err := getTopAndBottomM5Net(coinank)
	if err != nil {
		log.Println(err)
		return nil
	}
	for _, s := range symbolsGo {
		if s.Side && s.M5Net > config.BuyNetAmount || !s.Side && s.M5Net < -config.SideNetAmount {
			log.Println(s)
		}
	}
	return nil
}

// 判断账户是否有资格
func isAccount(client *futures.Client) error {
	// 账户信息
	account, err := client.NewGetAccountService().Do(context.Background())
	if err != nil {
		log.Println(err)
		return err
	}
	// 账户的总钱包余额，表示可用余额。
	totalWalletBalance, err := strconv.ParseFloat(account.TotalWalletBalance, 64)
	if err != nil {
		return err
	}
	// 总保证金余额，表示账户中可用的保证金总额。
	totalPositionInitialMargin, err := strconv.ParseFloat(account.TotalPositionInitialMargin, 64)
	if err != nil {
		return err
	}
	totalOpenOrderInitialMargin, err := strconv.ParseFloat(account.TotalOpenOrderInitialMargin, 64)
	if err != nil {
		return err
	}
	// 计算已用余额
	usedBalance := totalPositionInitialMargin + totalOpenOrderInitialMargin
	if usedBalance > totalWalletBalance*0.5 || totalWalletBalance == 0 {
		return fmt.Errorf("已使用超过资金的50%")
	}
	return nil
}

// Coinank-Apikey 获取
func getKey() string {
	signStr := fmt.Sprintf("%s|%d%d", "-b31e-c547-d299-b6d07b7631aba2c903cca2c903cc", time.Now().UnixNano()/int64(time.Millisecond)+1111111111111, 347)
	originalBytes := []byte(signStr)
	base64Bytes := base64.StdEncoding.EncodeToString(originalBytes)
	return base64Bytes
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

// 取Coinank数据
func fetchFundCoinankData() ([]FundData, error) {
	req, err := http.NewRequest("POST", "https://coinank.com/api/fund/fundReal?page=1&size=50&type=1&productType=SWAP&sortBy=&baseCoin=&isFollow=false", nil)
	if err != nil {
		log.Printf("创建请求失败: %v", err)
		return nil, err
	}
	req.Header.Add("coinank-apikey", getKey())

	// 发送请求
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("请求失败: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求失败，状态码: %d", resp.StatusCode)
	}

	// 解析响应
	var responseData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
		log.Printf("解析响应失败: %v", err)
		return nil, err
	}

	// 检查 success 字段
	if success, ok := responseData["success"].(bool); !ok || !success {
		return nil, fmt.Errorf("API返回数据失败，内容: %v", responseData)
	}

	// 处理数据
	dataList := responseData["data"].(map[string]interface{})["list"].([]interface{})
	fundDataList := make([]FundData, 0)

	for _, item := range dataList {
		itemMap := item.(map[string]interface{})
		Coin := itemMap["baseCoin"].(string)

		// 检查是否在黑名单中
		if !contains(config.Blacklist, Coin) {
			fundData := FundData{
				Coin:   Coin,
				Side:   itemMap["m5net"].(float64) > 50*10000,
				M5Net:  itemMap["m5net"].(float64),
				M15Net: itemMap["m15net"].(float64),
				// 处理其他字段...
			}
			// 使用 append 添加到切片
			fundDataList = append(fundDataList, fundData)
		}
	}

	return fundDataList, nil
}

// 取高低数据
func getTopAndBottomM5Net(data []FundData) (symbolsGo []FundData, err error) {
	// 使用 sort.Slice 来排序
	target := make([]FundData, len(data))
	copy(target, data)
	sort.Slice(target, func(i, j int) bool {
		return target[i].M5Net > target[j].M5Net // 降序排序
	})

	// 获取最高的两个
	if len(data) >= config.MaxCoins {
		symbolsGo = append(symbolsGo, target[:config.MaxCoins]...)
	} else {
		return symbolsGo, fmt.Errorf("数据量不足")
	}

	target1 := make([]FundData, len(data))
	copy(target1, data)
	// 获取最低的两个，使用升序排序
	sort.Slice(target1, func(i, j int) bool {
		return target1[i].M5Net < target1[j].M5Net // 升序排序
	})

	// 获取最低的两个
	if len(target1) >= config.MaxCoins {
		symbolsGo = append(symbolsGo, target1[:config.MaxCoins]...)
	} else {
		return symbolsGo, fmt.Errorf("数据量不足")
	}

	return symbolsGo, nil
}
