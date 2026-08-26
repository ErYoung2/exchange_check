package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// API 响应结构
type ExchangeAPIResponse struct {
	Rates          map[string]float64 `json:"rates"`
	TimeLastUpdate string             `json:"time_last_update_utc"`
}

// 内存中保存的汇率缓存数据
type RateCache struct {
	sync.RWMutex
	USDCNY     float64 `json:"usd_cny"`
	HKDCNY     float64 `json:"hkd_cny"`
	UpdateTime string  `json:"update_time"`
	FetchTime  string  `json:"fetch_time"`
}

var cache RateCache

// 从外部 API 更新汇率
func updateRates() {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://open.er-api.com/v6/latest/USD")
	if err != nil {
		log.Printf("[错误] 获取汇率失败: %v", err)
		return
	}
	defer resp.Body.Close()

	var data ExchangeAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		log.Printf("[错误] 解析 JSON 失败: %v", err)
		return
	}

	cnyRate, hasCNY := data.Rates["CNY"]
	hkdRate, hasHKD := data.Rates["HKD"]

	if !hasCNY || !hasHKD {
		log.Printf("[错误] 汇率数据不完整")
		return
	}

	// 加锁更新内存缓存
	cache.Lock()
	cache.USDCNY = cnyRate
	cache.HKDCNY = cnyRate / hkdRate
	cache.UpdateTime = data.TimeLastUpdate
	cache.FetchTime = time.Now().Format("2006-01-02 15:04:05")
	cache.Unlock()

	log.Printf("[成功] 汇率已更新 | USD/CNY: %.4f | HKD/CNY: %.4f", cache.USDCNY, cache.HKDCNY)
}

// 启动 5 分钟定时任务
func startCronJob() {
	// 服务启动时立即拉取一次
	updateRates()

	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			updateRates()
		}
	}()
}

// 提供给小程序的 HTTP 接口
func ratesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// 允许跨域（方便开发调试）
	w.Header().Set("Access-Control-Allow-Origin", "*")

	cache.RLock()
	defer cache.RUnlock()

	if cache.USDCNY == 0 {
		http.Error(w, `{"error": "数据尚未准备就绪"}`, http.StatusServiceUnavailable)
		return
	}

	json.NewEncoder(w).Encode(cache)
}

func main() {
	// 初始化 Go module (若未初始化)
	// 启动定时拉取任务
	startCronJob()

	// 注册 API 路由
	http.HandleFunc("/api/rates", ratesHandler)

	port := ":8080"
	log.Printf("服务启动在端口 %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}
