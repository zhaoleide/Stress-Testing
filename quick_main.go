package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"
)

func main() {
	fmt.Println("=== 快速登录压测 ===")
	
	url := "https://example.com/apiv2/getVerifyCode"
	userCount := 5
	concurrency := 2
	
	fmt.Printf("目标URL: %s\n", url)
	fmt.Printf("用户数: %d, 并发数: %d\n", userCount, concurrency)
	
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)
	
	success := 0
	failed := 0
	var mu sync.Mutex
	
	start := time.Now()
	
	for i := 0; i < userCount; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			client := &http.Client{
				Timeout: 30 * time.Second,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
			}
			
			resp, err := client.Post(url, "application/json", nil)
			if err != nil {
				fmt.Printf("用户%d 请求失败: %v\n", userID, err)
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}
			defer resp.Body.Close()
			
			fmt.Printf("用户%d 状态码: %d\n", userID, resp.StatusCode)
			
			mu.Lock()
			if resp.StatusCode == 200 {
				success++
			} else {
				failed++
			}
			mu.Unlock()
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	fmt.Println("\n=== 测试结果 ===")
	fmt.Printf("总耗时: %v\n", duration)
	fmt.Printf("成功: %d\n", success)
	fmt.Printf("失败: %d\n", failed)
	fmt.Printf("成功率: %.2f%%\n", float64(success)/float64(userCount)*100)
	fmt.Printf("QPS: %.2f\n", float64(success)/duration.Seconds())
}