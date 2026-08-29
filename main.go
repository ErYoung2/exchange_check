package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// 1. 第三方 API 返回的 JSON 结构体
type ExchangeAPIResponse struct {
	Rates          map[string]float64 `json:"rates"`
	TimeLastUpdate string             `json:"time_last_update_utc"`
}

// 2. 本地缓存并输出给前端的 JSON 结构体
type ExchangeCache struct {
	USDCNY    float64   `json:"usd_cny"`
	HKDCNY    float64   `json:"hkd_cny"`
	FetchTime time.Time `json:"fetch_time"`
}

var (
	cache ExchangeCache
	mu    sync.RWMutex
)

// 兜底逻辑：防止第三方 API 失败时返回全 0
func useFallbackRates() {
	mu.Lock()
	defer mu.Unlock()
	if cache.USDCNY == 0 {
		cache = ExchangeCache{
			USDCNY:    7.2350,
			HKDCNY:    0.9250,
			FetchTime: time.Now(),
		}
		log.Println("[INFO] 启用兜底汇率数据成功")
	}
}

// 从第三方 API 获取实时汇率
func fetchRates() {
	// 设置 5 秒严格超时，防止请求挂起
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get("https://open.er-api.com/v6/latest/USD")
	if err != nil {
		log.Printf("[ERROR] 请求第三方 API 失败: %v", err)
		useFallbackRates()
		return
	}
	defer resp.Body.Close()

	var data ExchangeAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil || len(data.Rates) == 0 {
		log.Printf("[ERROR] 解析第三方 API 数据失败: %v", err)
		useFallbackRates()
		return
	}

	cnyRate := data.Rates["CNY"]
	hkdRate := data.Rates["HKD"]

	if cnyRate == 0 || hkdRate == 0 {
		log.Println("[ERROR] 第三方 API 返回汇率为 0")
		useFallbackRates()
		return
	}

	mu.Lock()
	cache = ExchangeCache{
		USDCNY:    cnyRate,
		HKDCNY:    cnyRate / hkdRate,
		FetchTime: time.Now(),
	}
	mu.Unlock()

	log.Printf("[SUCCESS] 汇率拉取成功: USD/CNY=%.4f, HKD/CNY=%.4f", cnyRate, cnyRate/hkdRate)
}

// 处理前端请求的 HTTP Handler
func handleRates(w http.ResponseWriter, r *http.Request) {
	// 设置跨域与数据格式
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	// 禁用缓存，防止前端收到 304 导致数据不更新
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	mu.RLock()
	currentCache := cache
	mu.RUnlock()

	// 如果数据尚未初始化，自动触发一次兜底
	if currentCache.USDCNY == 0 {
		useFallbackRates()
		mu.RLock()
		currentCache = cache
		mu.RUnlock()
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(currentCache)
}

func main() {
	// 1. 启动时异步拉取一次汇率（非阻塞）
	go fetchRates()

	// 2. 5 分钟定时器刷新数据
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			fetchRates()
		}
	}()

	// 3. 注册路由并启动服务
	http.HandleFunc("/api/rates", handleRates)

	log.Println("Server running on :8080...")
	if err := http.ListenAndServe("0.0.0.0:8080", nil); err != nil {
		log.Fatal(err)
	}
}
