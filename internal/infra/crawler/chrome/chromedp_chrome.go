package chrome

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/LouYuanbo1/crawleragent/internal/config"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/types"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type chromedpCrawler struct {
	allocCtx      context.Context
	allocCtxFuc   context.CancelFunc
	pageCtx       context.Context
	pageCtxFuc    context.CancelFunc
	timeoutCtxFuc context.CancelFunc
	requestCache  sync.Map
}

func InitChromedpCrawler(ctx context.Context, cfg *config.Config) ChromeCrawler {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", cfg.Chromedp.Headless),
		chromedp.Flag("disable-blink-features", cfg.Chromedp.DisableBlinkFeatures),
		chromedp.Flag("incognito", cfg.Chromedp.Incognito),
		chromedp.Flag("disable-dev-shm-usage", cfg.Chromedp.DisableDevShmUsage),
		chromedp.Flag("no-sandbox", cfg.Chromedp.NoSandbox),
		chromedp.UserDataDir(cfg.Chromedp.UserDataDir),
		chromedp.UserAgent(cfg.Chromedp.UserAgent),
	)
	timeoutCtx, cancelTimeout := context.WithTimeout(ctx, time.Duration(cfg.Chromedp.LifeTime)*time.Second)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(timeoutCtx, opts...)
	pageCtx, cancelPage := chromedp.NewContext(allocCtx)

	return &chromedpCrawler{
		allocCtx:      allocCtx,
		allocCtxFuc:   cancelAlloc,
		pageCtx:       pageCtx,
		pageCtxFuc:    cancelPage,
		timeoutCtxFuc: cancelTimeout,
	}
}

func (cc *chromedpCrawler) PageContext() context.Context {
	return cc.pageCtx
}

func (cc *chromedpCrawler) Close() {
	cc.pageCtxFuc()
	cc.allocCtxFuc()
	cc.timeoutCtxFuc()
}

func (cc *chromedpCrawler) InitAndNavigate(url string) error {
	return chromedp.Run(cc.pageCtx,
		network.Enable(),       // 开启网络监听
		chromedp.Navigate(url), // 导航到页面
		chromedp.Sleep(3*time.Second),
	)
}

func (cc *chromedpCrawler) PerformScrolling(scrollTimes, standardSleepSeconds, randomDelaySeconds int) error {
	scrollFunc := chromedp.ActionFunc(func(ctx context.Context) error {
		fmt.Println("开始执行滑动操作...")

		// 创建本地随机数生成器

		var totalSleep time.Duration

		for i := range scrollTimes {
			// 随机选择滑动策略
			switch rand.IntN(2) {
			case 0:
				// 滑动到底部
				js := `window.scrollTo({
					top: document.body.scrollHeight,
					behavior: 'smooth'
				});`
				if err := chromedp.Evaluate(js, nil).Do(ctx); err != nil {
					return fmt.Errorf("滑动到底部失败: %v", err)
				}
				fmt.Printf("第 %d 次滑动: 到底部\n", i+1)
			case 1:
				// 滑动到随机比例
				ratio := 0.7 + rand.Float64()*0.3 // 70%-100% 位置
				js := fmt.Sprintf(`window.scrollTo({
					top: document.body.scrollHeight * %f,
					behavior: 'smooth'
				});`, ratio)
				if err := chromedp.Evaluate(js, nil).Do(ctx); err != nil {
					return fmt.Errorf("滑动到比例位置失败: %v", err)
				}
				fmt.Printf("第 %d 次滑动: 到 %.0f%% 位置\n", i+1, ratio*100)
			}

			randomDelay := rand.Float64() * float64(randomDelaySeconds)
			totalSleep = time.Duration((float64(standardSleepSeconds) + randomDelay) * float64(time.Second))

			fmt.Printf("等待 %.1f 秒\n", totalSleep.Seconds())
			chromedp.Sleep(totalSleep).Do(ctx)
		}
		fmt.Printf("完成 %d 次滑动\n", scrollTimes)
		return nil
	})
	err := chromedp.Run(cc.pageCtx, scrollFunc)
	if err != nil {
		return fmt.Errorf("浏览器自动化执行失败: %v", err)
	}
	return nil
}

func (cc *chromedpCrawler) SetNetworkListener(urlPattern string, respChan chan *types.NetworkResponse) {
	chromedp.ListenTarget(cc.pageCtx, func(ev any) {
		switch ev := ev.(type) {
		case *network.EventResponseReceived:
			resp := ev.Response
			if strings.Contains(resp.URL, urlPattern) {
				fmt.Printf("请求ID: %s\n", ev.RequestID)
				fmt.Printf("检测到目标API响应: %s\n", resp.URL)
				fmt.Printf("响应状态码: %d\n", resp.Status)
				cc.requestCache.Store(ev.RequestID, resp.URL)
			}

		case *network.EventLoadingFinished:
			// 当请求加载完成时获取响应体
			if cachedURL, ok := cc.requestCache.Load(ev.RequestID); ok {
				// 类型断言，因为Load返回any类型
				if urlStr, ok := cachedURL.(string); ok {
					if strings.Contains(urlStr, urlPattern) {
						// 处理完成后删除
						cc.requestCache.Delete(ev.RequestID)
						go cc.getResponseBody(ev.RequestID, urlStr, respChan)
					}
				}
			}
		}
	})
}

func (cc *chromedpCrawler) PerformClick(selector string, clickCount, standardSleepSeconds, randomDelaySeconds int) error {
	randomDelay := rand.Float64() * float64(randomDelaySeconds)
	totalSleep := time.Duration((float64(standardSleepSeconds) + randomDelay) * float64(time.Second))
	for range clickCount {
		err := chromedp.Run(cc.pageCtx,
			chromedp.Click(selector),
			chromedp.Sleep(totalSleep),
		)
		if err != nil {
			return fmt.Errorf("点击失败: %v", err)
		}
	}
	return nil
}

func (cc *chromedpCrawler) getResponseBody(requestID network.RequestID, cachedURL string, respChan chan *types.NetworkResponse) {
	c := chromedp.FromContext(cc.pageCtx)
	responseBodyParams := network.GetResponseBody(requestID)
	ctx := cdp.WithExecutor(cc.pageCtx, c.Target)
	body, err := responseBodyParams.Do(ctx)
	if err != nil {
		log.Printf("获取响应体失败 (RequestID: %s): %v",
			requestID, err)
		return
	}

	fmt.Printf("成功获取响应体 (URL: %s, RequestID: %s, 大小: %d bytes)\n", cachedURL, requestID, len(body))
	respChan <- &types.NetworkResponse{
		Url:  cachedURL,
		Body: body,
	}
}
