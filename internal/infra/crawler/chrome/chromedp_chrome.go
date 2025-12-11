package chrome

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/LouYuanbo1/crawleragent/internal/config"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type chromedpCrawler struct {
	requestCache  sync.Map
	allocCtx      context.Context
	allocCtxFuc   context.CancelFunc
	pageCtx       context.Context
	pageCtxFuc    context.CancelFunc
	timeoutCtxFuc context.CancelFunc
}

func InitChromedpCrawler(ctx context.Context, cfg *config.Config) ChromedpCrawler {
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

func (cc *chromedpCrawler) Close() {
	cc.pageCtxFuc()
	cc.allocCtxFuc()
	cc.timeoutCtxFuc()
}

func (cc *chromedpCrawler) PageContext() context.Context {
	return cc.pageCtx
}

func (cc *chromedpCrawler) RequestCache() *sync.Map {
	return &cc.requestCache
}

func (cc *chromedpCrawler) InitAndNavigate(url string) error {
	return chromedp.Run(cc.pageCtx,
		network.Enable(),       // 开启网络监听
		chromedp.Navigate(url), // 导航到页面
		chromedp.Sleep(3*time.Second),
	)
}

func (cc *chromedpCrawler) ResetAndScroll(scrollTimes, standardSleepSeconds, randomDelaySeconds int) error {
	// 2. 清空请求缓存 (sync.Map)
	cc.requestCache.Range(func(key, value any) bool {
		cc.requestCache.Delete(key)
		return true // 继续遍历
	})
	err := chromedp.Run(cc.pageCtx,

		// 执行滑动操作
		cc.performScrolling(scrollTimes, standardSleepSeconds, randomDelaySeconds),

		chromedp.Sleep(time.Duration(standardSleepSeconds*3)*time.Second+time.Duration(randomDelaySeconds*3)*time.Second),
	)

	if err != nil {
		return fmt.Errorf("浏览器自动化执行失败: %v", err)
	}
	return nil
}

func (cc *chromedpCrawler) performScrolling(scrollTimes, standardSleepSeconds, randomDelaySeconds int) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		fmt.Println("开始执行滑动操作...")

		// 创建本地随机数生成器
		localRand := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := range scrollTimes {
			// 随机选择滑动策略
			switch localRand.Intn(2) {
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
				ratio := 0.7 + localRand.Float64()*0.3 // 70%-100% 位置
				js := fmt.Sprintf(`window.scrollTo({
					top: document.body.scrollHeight * %f,
					behavior: 'smooth'
				});`, ratio)
				if err := chromedp.Evaluate(js, nil).Do(ctx); err != nil {
					return fmt.Errorf("滑动到比例位置失败: %v", err)
				}
				fmt.Printf("第 %d 次滑动: 到 %.0f%% 位置\n", i+1, ratio*100)
			}

			randomDelay := time.Duration(localRand.Float64() * float64(randomDelaySeconds) * float64(time.Second))
			totalSleep := time.Duration(standardSleepSeconds)*time.Second + randomDelay

			fmt.Printf("等待 %.1f 秒\n", totalSleep.Seconds())
			chromedp.Sleep(totalSleep).Do(ctx)
		}

		// 最终滑动和等待
		finalJS := `window.scrollTo({top: document.body.scrollHeight, behavior: 'smooth'});`
		if err := chromedp.Evaluate(finalJS, nil).Do(ctx); err != nil {
			return fmt.Errorf("最终滑动失败: %v", err)
		}

		finalSleep := 2 * time.Duration(randomDelaySeconds) * time.Second
		fmt.Printf("最终等待 %.1f 秒\n", finalSleep.Seconds())
		chromedp.Sleep(finalSleep).Do(ctx)

		fmt.Printf("完成 %d 次滑动\n", scrollTimes)
		return nil
	}
}
