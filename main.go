package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
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

// 全局客户端
var client *futures.Client

var symbols []futures.Symbol

var symbolsString []string

var httpClient *http.Client

func main() {
	fmt.Printf("Go version: %s\n", runtime.Version())

	// / 获取当前日期，按日期生成日志文件名
	currentDate := time.Now().Format("2006-01-02")
	logFileName := fmt.Sprintf("logs/%s.log", currentDate)

	// 创建日志文件目录（如果不存在）
	err := os.MkdirAll("logs", os.ModePerm)
	if err != nil {
		fmt.Println("Error creating logs directory:", err)
		return
	}

	// 打开或创建日志文件，追加模式
	file, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Println(err)
		}
	}(file)

	multiWriter := io.MultiWriter(file, os.Stdout)

	log.SetFlags(log.LstdFlags) // 清除默认的时间标志
	log.SetOutput(multiWriter)

	client = binance.NewFuturesClient(config.ApiKey, config.ApiSecret)

	httpClient = &http.Client{}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	if config.Proxy != "" {
		proxyURL, err := url.Parse(config.Proxy)
		if err != nil {
			log.Fatalf("无效的代理地址: %v", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
		binance.SetWsProxyUrl(config.Proxy)
		fmt.Println("代理已连接 - 网络判断")
	} else {
		fmt.Println("无代理 - 网络判断")
	}

	timeout := 360 // 默认超时时间
	if config.Timeout != 0 {
		timeout = config.Timeout
	}
	httpClient = &http.Client{
		Transport: transport,
		Timeout:   time.Second * time.Duration(timeout),
	}

	client.HTTPClient = httpClient
	if checkConnection("https://coinank.com/api/fund/fundReal?page=1&size=50&type=1&productType=SWAP&sortBy=&baseCoin=&isFollow=false") {
		fmt.Println("Coinank连接正常")
	} else {
		log.Fatal("无法连接到Coinank")
	}
	if checkConnection("https://fapi.binance.com/fapi/v1/time") {
		fmt.Println("Binance连接正常")
	} else {
		log.Fatal("无法连接到Binance")
	}

	// 时间偏移
	_, err = client.NewSetServerTimeService().Do(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	// 获取交易信息
	info, err := client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	// 赛选币种
	for _, s := range info.Symbols {
		if s.QuoteAsset == "USDT" && s.ContractType == "PERPETUAL" && s.Status == "TRADING" && !contains(config.Blacklist, s.BaseAsset) {
			symbols = append(symbols, s)
			symbolsString = append(symbolsString, s.BaseAsset)
		}
	}

	now := time.Now()
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	duration := nextMinute.Sub(now)
	time.Sleep(duration)

	log.Println("[CoinankGo] 开始")
	go func() {
		for {
			go func() {
				err := CoinankGo()
				if err != nil {
					log.Println(err)
				}
			}()
			// 等待 s时间
			time.Sleep(time.Duration(config.Duration) * time.Second)
		}
	}()
	//
	select {}
}

// 初始化

// CoinankGo Coinank开始
func CoinankGo() error {
	// if err := isAccount(); err != nil {
	// 	log.Println(err)
	// 	return nil
	// }

	coinank, err := fetchFundCoinankData()
	if err != nil {
		log.Println(err)
		return nil
	}
	symbolsNet, err := getTopAndBottomM5Net(coinank)
	if err != nil {
		log.Println(err)
		return nil
	}
	symbolsFilter, err := filterSymbols(symbolsNet)
	if err != nil {
		log.Println(err)
		return nil
	}
	// log.Println(symbolsFilter)
	if len(symbolsFilter) < 1 {
		log.Println("空")
	}

	return nil
}

// 判断账户是否有资格
func isAccount() error {
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
		// log.Printf("创建请求失败: %v", err)
		return nil, err
	}
	req.Header.Add("coinank-apikey", getKey())

	// 发送请求
	resp, err := httpClient.Do(req)
	if err != nil {
		// log.Printf("请求失败: %v", err)
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(resp.Body)

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求失败，状态码: %d", resp.StatusCode)
	}

	// 解析响应
	var responseData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
		// log.Printf("解析响应失败: %v", err)
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

		// 检查是否在名单中
		if contains(symbolsString, Coin) {
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
func getTopAndBottomM5Net(data []FundData) (symbolsNet []FundData, err error) {
	// 使用 sort.Slice 来排序
	target := make([]FundData, len(data))
	copy(target, data)
	sort.Slice(target, func(i, j int) bool {
		return target[i].M5Net > target[j].M5Net // 降序排序
	})

	// 获取最高的两个
	if len(data) >= config.MaxCoins {
		symbolsNet = append(symbolsNet, target[:config.MaxCoins]...)
	} else {
		return symbolsNet, fmt.Errorf("数据量不足")
	}

	target1 := make([]FundData, len(data))
	copy(target1, data)
	// 获取最低的两个，使用升序排序
	sort.Slice(target1, func(i, j int) bool {
		return target1[i].M5Net < target1[j].M5Net // 升序排序
	})

	// 获取最低的两个
	if len(target1) >= config.MaxCoins {
		symbolsNet = append(symbolsNet, target1[:config.MaxCoins]...)
	} else {
		return symbolsNet, fmt.Errorf("数据量不足")
	}
	target2 := make([]FundData, 0)

	for _, s := range symbolsNet {
		if s.Side && s.M5Net > config.BuyNetAmount || !s.Side && s.M5Net < -config.SideNetAmount {
			target2 = append(target2, s)
		}
	}

	return target2, nil
}

// 筛选币种
func filterSymbols(symbols []FundData) ([]FundData, error) {
	target := make([]FundData, 0)
	for _, s := range symbols {
		klines, err := client.NewKlinesService().Symbol(s.Coin + "USDT").
			Interval("1m").Limit(201).Do(context.Background())
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		closedPrices := make([]float64, 0, len(klines))
		for _, kline := range klines[:len(klines)] {
			closeFloat, err := strconv.ParseFloat(kline.Close, 64)
			if err != nil {
				log.Println(err)
				return nil, err
			}
			closedPrices = append(closedPrices, closeFloat) // 将每个 K 线的 Close 值添加到切片中
		}
		//rsi := RSI(closedPrices, 6)
		crsi := CRSI(closedPrices, config.RsiLength)
		// fmt.Println(s.Coin, s.Side, closedPrices[200], crsi[200])
		if s.Side && crsi[200] < config.RsiLevel {
			log.Println("["+s.Coin+"] 符合要求 多 RSI | ", closedPrices[200], crsi[200], s)
			target = append(target, s)
			continue
		}
		if !s.Side && crsi[200] > (100-config.RsiLevel) {
			log.Println("["+s.Coin+"] 符合要求 空 RSI | ", closedPrices[200], crsi[200], s)
			target = append(target, s)
			continue
		}
		if s.Side && s.M5Net > config.BuyNetAmount && s.M15Net > 1 && s.M15Net > (s.M5Net*config.MultipleNetAmount) {
			log.Println("["+s.Coin+"] 符合要求 多 量 | ", closedPrices[200], crsi[200], s)
			target = append(target, s)
			continue
		}
		if !s.Side && s.M5Net < -config.SideNetAmount && s.M15Net < 1 && s.M15Net < (s.M5Net*config.MultipleNetAmount) {
			log.Println("["+s.Coin+"] 符合要求 空 量 | ", closedPrices[200], crsi[200], s)
			target = append(target, s)
			continue
		}
	}

	return target, nil
}

func checkConnection(url string) bool {
	// 检查 URL 是否正确
	if url == "" {
		fmt.Println("URL 不能为空")
		return false
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("创建请求失败: %v", err)
		return false
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Println("无法连接到", url, "错误:", err)
		return false
	}
	// 确保 resp 不为 nil 再调用 Close()
	defer func() {
		if resp != nil {
			resp.Body.Close()
		}
	}()
	if resp.StatusCode == http.StatusOK {
		return true
	}

	fmt.Println("连接到", url, "失败，状态码:", resp.StatusCode)
	return false
}

// 筛选持仓
// func HoldOrder(s []FundData) ([]FundData, error) {
// 	account, err := client.NewGetAccountService().Do(context.Background())
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	// 输出账户中的所有资产信息
// 	for _, asset := range account.Positions {
// 		if asset.PositionAmt != "0" {

// 		}
// 	}
// }
