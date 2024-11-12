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
	"strings"
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

var infoData *futures.ExchangeInfo

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
			log.Fatalf("Invalid: %v", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
		binance.SetWsProxyUrl(config.Proxy)
		fmt.Println("Use Proxy")
	} else {
		fmt.Println("No Proxy")
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
		fmt.Println("Coinank OK")
	} else {
		log.Fatal("Connection failed Coinank")
	}
	if checkConnection("https://fapi.binance.com/fapi/v1/time") {
		fmt.Println("Binance OK")
	} else {
		log.Fatal("Connection failed Binance")
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
	infoData = info
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

	// log.Println("[Go] 开始")

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
	go func() {
		for {
			// 时间偏移
			time.Sleep(300 * time.Second)
			_, err = client.NewSetServerTimeService().Do(context.Background())
			if err != nil {
				log.Fatal(err)
			}
		}
	}()
	//
	select {}
}

// 开始
func CoinankGo() error {

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
	if len(symbolsFilter) > 0 {

		OpenSymbols, err := ordersAccount(symbolsFilter)
		if err != nil {
			log.Println(err)
			return nil
		}
		// log.Panicln(OpenSymbols)
		err = ordersOrders(OpenSymbols)
		if err != nil {
			log.Println(err)
			return nil
		}
	} else {
		log.Println("----------")
	}
	return nil
}

// 处理已有订单
func ordersAccount(symbols []FundData) (OpenSymbols []FundData, err error) {
	// 账户信息
	account, err := client.NewGetAccountService().Do(context.Background())
	if err != nil {
		log.Println(err)
		return nil, err
	}
	// 账户的总钱包余额，表示可用余额。
	totalWalletBalance, err := strconv.ParseFloat(account.TotalWalletBalance, 64)
	if err != nil {
		return nil, err
	}
	// 总保证金余额，表示账户中可用的保证金总额。
	totalPositionInitialMargin, err := strconv.ParseFloat(account.TotalPositionInitialMargin, 64)
	if err != nil {
		return nil, err
	}
	totalOpenOrderInitialMargin, err := strconv.ParseFloat(account.TotalOpenOrderInitialMargin, 64)
	if err != nil {
		return nil, err
	}
	// 计算已用余额
	usedBalance := totalPositionInitialMargin + totalOpenOrderInitialMargin
	if usedBalance > totalWalletBalance*config.MarginUtilizationRate || totalWalletBalance == 0 {
		return nil, fmt.Errorf("use more than 50%")
	}

	for _, symbol := range symbols {
		asset, err := getAccountPositionSymbolsFundData(account.Positions, symbol)
		if err != nil {
			log.Println(err)
			continue
		}
		PositionAmt, err := strconv.ParseFloat(asset.PositionAmt, 64)
		if err != nil {
			log.Println(err)
			continue
		}
		asset2, err := getAccountPositionSymbolsFundDataFan(account.Positions, symbol)
		if err != nil {
			log.Println(err)
			continue
		}
		PositionAmt2, err := strconv.ParseFloat(asset2.PositionAmt, 64)
		if err != nil {
			log.Println(err)
			continue
		}
		if !config.Duak {
			if PositionAmt == 0 && PositionAmt2 == 0 { //都没有
				log.Println(symbol.Coin, "NO POSITION")
				// 去挂单
				OpenSymbols = append(OpenSymbols, symbol)
				continue

			} else if PositionAmt != 0 && PositionAmt2 == 0 { //正常方向有，反方向没有
				continue
			} else { //反方向有
				if config.ProfitExit > 0 {
					unrealizedProfit, err := strconv.ParseFloat(asset2.UnrealizedProfit, 64)
					if err != nil {
						continue
					}
					if unrealizedProfit < config.ProfitExit {
						continue
					}
				}
				if asset2.PositionSide == "LONG" && !symbol.Side {
					log.Println(symbol.Coin, "LONG->SHORT / ", asset2.PositionSide)
					OpenSymbols = append(OpenSymbols, symbol)
					err = placeOrder(symbol.Coin+"USDT", "SELL", "LONG", false)
					if err != nil {
						log.Println(err)
						continue
					}
				} else if asset2.PositionSide == "SHORT" && symbol.Side {
					log.Println(symbol.Coin, "SHORT->LONG / ", asset2.PositionSide)
					OpenSymbols = append(OpenSymbols, symbol)
					err = placeOrder(symbol.Coin+"USDT", "BUY", "SHORT", false)
					if err != nil {
						log.Println(err)
						continue
					}

				}
			}
			continue
		} else {
			if PositionAmt == 0 {
				log.Println(symbol.Coin, "NO POSITION")
				// 去挂单
				OpenSymbols = append(OpenSymbols, symbol)
			}
		}

	}

	return OpenSymbols, nil
}

// 处理挂单
func ordersOrders(symbols []FundData) error {
	// 挂单
	openOrders, err := client.NewListOpenOrdersService().Do(context.Background())
	if err != nil {
		log.Println(err)
		return err
	}
	// 收集已取消的挂单
	// 取消超时订单
	for i, order := range openOrders {
		// 只有开单才处理
		if order.ClosePosition {
			continue
		}
		// 取订单时间
		now := time.Now().UnixMilli()
		if now-order.UpdateTime > config.OrdersTimeout*1000 {
			// 判断UpdateTime 更新时间是否过期
			log.Println(order.Symbol, "Expired")
			err := cancelOrder(order.Symbol, order.OrderID)
			if err != nil {
				log.Println(err)
				continue
			}
			//清除订单
			openOrders[i].Symbol = ""
		}

	}
	// 开始挂单
	for _, symbol := range symbols {
		order, err := getOrderSymbolsFundData(openOrders, symbol.Coin+"USDT")
		if err != nil { // 没有持有
			log.Println(symbol.Coin, "Order")
			if symbol.Side {
				err = placeOrder(symbol.Coin+"USDT", "BUY", "LONG", true)
				if err != nil {
					log.Println(err)
					continue
				}
			} else {
				err = placeOrder(symbol.Coin+"USDT", "SELL", "SHORT", true)
				if err != nil {
					log.Println(err)
					continue
				}
			}
			continue
		}
		// 平仓单
		if order.ClosePosition {
			continue
		}
		// 反转
		if symbol.Side && order.PositionSide == "SHORT" {
			// 反转 修改订单 空转多
			err := cancelOrder(order.Symbol, order.OrderID)
			if err != nil {
				log.Println(err)
				continue
			}
			err = placeOrder(order.Symbol, "BUY", "LONG", true)
			if err != nil {
				log.Println(err)
				continue
			}
			continue
		} else if !symbol.Side && order.PositionSide == "LONG" {
			// 反转 多转空
			err := cancelOrder(order.Symbol, order.OrderID)
			if err != nil {
				log.Println(err)
				continue
			}
			err = placeOrder(order.Symbol, "SELL", "SHORT", true)
			if err != nil {
				log.Println(err)
				continue
			}
			continue
		}
	}

	return nil
}

// 下单
func placeOrder(symbol string, side futures.SideType, positionSide futures.PositionSideType, isBook bool) error {
	// 取订单铺
	book, ree := client.NewDepthService().Symbol(symbol).Limit(50).Do(context.Background())
	if ree != nil {
		log.Println(ree)
		return ree
	}
	var price string

	if side == "BUY" && positionSide == "LONG" {
		price = book.Bids[config.PriceDepth-1].Price
	} else if side == "SELL" && positionSide == "LONG" {
		price = book.Asks[config.PriceDepth-1].Price
	} else if side == "BUY" && positionSide == "SHORT" {
		price = book.Bids[config.PriceDepth-1].Price
	} else if side == "SELL" && positionSide == "SHORT" {
		price = book.Asks[config.PriceDepth-1].Price
	}

	prices, err := strconv.ParseFloat(price, 64)
	if err != nil {
		log.Println(err)
		return err
	}
	log.Println(symbol, side, positionSide, prices)
	// 根据  prices 和cconfig.amount 计算出数量
	amount := config.Amount / prices
	// infoData 取到币种信息 设置数量和价格小数位
	infoDataSymbols, err := getInfoSymbolsFundData(infoData, symbol)
	if err != nil {
		log.Println(err)
		return err
	}

	amountStr, err := takeDivisible(amount, infoDataSymbols.Filters[1]["stepSize"].(string))
	if err != nil {
		log.Println(err)
		return err
	}
	if isBook {
		pricesStr := strconv.FormatFloat(prices, 'f', -1, 64)
		_, err = client.NewCreateOrderService().Symbol(symbol).Type("LIMIT").Price(pricesStr).Side(side).PositionSide(positionSide).Quantity(amountStr).TimeInForce("GTC").Do(context.Background())
		if err != nil {
			log.Println(err)
			return err
		}
	} else {
		_, err = client.NewCreateOrderService().Symbol(symbol).Type("MARKET").Side(side).PositionSide(positionSide).Quantity(amountStr).Do(context.Background())
		if err != nil {
			log.Println(err)
			return err
		}
	}

	return nil
}

// 取消订单
func cancelOrder(symbol string, orderId int64) error {
	_, err := client.NewCancelOrderService().Symbol(symbol).OrderID(orderId).Do(context.Background())
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

// 调整小数位数并确保可以整除
func takeDivisible(inputVal float64, divisor string) (string, error) {

	divisorVal, err := strconv.ParseFloat(divisor, 64)
	if err != nil || divisorVal == 0 {
		return "", fmt.Errorf("无效的 divisor: %v", err)
	}

	// 计算小数点位数
	decimalPlaces := 0
	if dot := strings.Index(divisor, "."); dot != -1 {
		decimalPlaces = len(divisor) - dot - 1
	}

	// 计算最大整除的值
	quotient := int(inputVal / divisorVal)
	maxDivisible := divisorVal * float64(quotient)

	// 格式化输出
	format := fmt.Sprintf("%%.%df", decimalPlaces)
	return fmt.Sprintf(format, maxDivisible), nil
}

// 取得InfoSymbo币种数据
func getInfoSymbolsFundData(symbols *futures.ExchangeInfo, symbolName string) (getSymbol futures.Symbol, err error) {
	for _, s := range symbols.Symbols {
		if s.Symbol == symbolName {
			return s, nil
		}
	}
	return getSymbol, fmt.Errorf("没有找到%s", symbolName)
}

// 取得当前挂单的币种信息
func getOrderSymbolsFundData(symbols []*futures.Order, symbolName string) (getSymbol *futures.Order, err error) {
	for _, s := range symbols {
		if s.Symbol == symbolName {
			return s, nil
		}
	}
	return getSymbol, fmt.Errorf("没有找到%s", symbolName)
}

// 取得当前仓位的币种信息
func getAccountPositionSymbolsFundData(symbols []*futures.AccountPosition, symbolName FundData) (getSymbol *futures.AccountPosition, err error) {
	var side futures.PositionSideType
	if symbolName.Side {
		side = "LONG"
	} else {
		side = "SHORT"
	}
	for _, s := range symbols {
		if s.Symbol == symbolName.Coin+"USDT" && s.PositionSide == side {
			return s, nil
		}
	}
	return getSymbol, fmt.Errorf("没有找到%s", symbolName)
}

// 取得当前仓位的币种信息 反
func getAccountPositionSymbolsFundDataFan(symbols []*futures.AccountPosition, symbolName FundData) (getSymbol *futures.AccountPosition, err error) {
	var side futures.PositionSideType
	if symbolName.Side {
		side = "SHORT"
	} else {
		side = "LONG"
	}
	for _, s := range symbols {
		if s.Symbol == symbolName.Coin+"USDT" && s.PositionSide == side {
			return s, nil
		}
	}
	return getSymbol, fmt.Errorf("没有找到%s", symbolName)
}

// 取得币种数据
func getSymbolsFundData(symbols []FundData, symbolName string) (getSymbol FundData, err error) {
	for _, s := range symbols {
		if s.Coin == symbolName {
			return s, nil
		}
	}
	return getSymbol, fmt.Errorf("没有找到%s", symbolName)
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

	return symbolsNet, nil
}

// 筛选币种
func filterSymbols(symbols []FundData) ([]FundData, error) {
	target := make([]FundData, 0)
	for _, s := range symbols {
		klines, err := client.NewKlinesService().Symbol(s.Coin + "USDT").
			Interval("5m").Limit(202).Do(context.Background())
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		closedPrices := make([]float64, 0, len(klines)-1)
		for _, kline := range klines[:len(klines)-1] {
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
		src200 := fmt.Sprintf("%.2f", crsi[200])
		M5Net, _ := takeDivisible(s.M5Net/1000000, "0.01")
		M15Net, _ := takeDivisible(s.M15Net/1000000, "0.01")
		// VOL
		if s.Side && s.M5Net > config.BuyNetAmount && s.M15Net > 1 && s.M15Net > (s.M5Net*config.MultipleNetAmount) {
			log.Println("["+s.Coin+"][LONG][VOL] | ", "RSI:", src200, " M5:", M5Net, " M15:", M15Net)
			target = append(target, s)
			continue
		}
		if !s.Side && s.M5Net < -config.SideNetAmount && s.M15Net < 1 && s.M15Net < (s.M5Net*config.MultipleNetAmount) {
			log.Println("["+s.Coin+"][SHORT][VOL] | ", "RSI:", src200, " M5:", M5Net, " M15:", M15Net)
			target = append(target, s)
			continue
		}
		// CRSI
		if s.Side && crsi[200] < config.RsiLevel {
			log.Println("["+s.Coin+"][LONG][RSI] | ", "RSI:", src200, " M5:", M5Net, " M15:", M15Net)
			target = append(target, s)
			continue
		}
		if !s.Side && crsi[200] > (100-config.RsiLevel) {
			log.Println("["+s.Coin+"][SHORT][RSI] | ", "RSI:", src200, " M5:", M5Net, " M15:", M15Net)
			target = append(target, s)
			continue
		}

	}

	return target, nil
}

// 链接检查
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
