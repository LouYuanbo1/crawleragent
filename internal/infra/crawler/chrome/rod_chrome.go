package chrome

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/LouYuanbo1/crawleragent/internal/config"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/types"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

type rodCrawler struct {
	browser *rod.Browser
	page    *rod.Page
	router  *rod.HijackRouter
}

func InitRodCrawler(cfg *config.Config) (ChromeCrawler, error) {
	/*
		url := launcher.
			NewUserMode().
			Bin(cfg.Rod.Bin).                                            // 设置chrome二进制路径
			Headless(cfg.Rod.Headless).                                  // 是否无头模式
			Set("disable-blink-features", cfg.Rod.DisableBlinkFeatures). // 禁用Blink特征
			Set("incognito").                                            // 是否无痕模式
			Set("disable-dev-shm-usage").                                // 禁用/dev/shm使用
			Set("no-sandbox").                                           // 是否禁用沙箱
			Leakless(cfg.Rod.Leakless).                                  // 是否禁用内存泄漏检测
			Set("user-data-dir", cfg.Rod.UserDataDir).                   // 设置用户数据目录
			Set("disable-web-security").                                 // 禁用同源策略
			Set("user-agent", cfg.Rod.UserAgent).                        // 设置用户代理
			MustLaunch()
	*/
	url := CreateLauncher(cfg.Rod.UserMode,
		WithBin(cfg.Rod.Bin),
		WithUserDataDir(cfg.Rod.UserDataDir),
		WithHeadless(cfg.Rod.Headless),
		WithDisableBlinkFeatures(cfg.Rod.DisableBlinkFeatures),
		WithIncognito(cfg.Rod.Incognito),
		WithDisableDevShmUsage(cfg.Rod.DisableDevShmUsage),
		WithNoSandbox(cfg.Rod.NoSandbox),
		//WithWindowSize(cfg.Rod.DefaultPageWidth, cfg.Rod.DefaultPageHeight),
		WithUserAgent(cfg.Rod.UserAgent),
		WithLeakless(cfg.Rod.Leakless),
	)
	urlStr, err := url.Launch()
	if err != nil {
		return nil, fmt.Errorf("启动浏览器失败: %v", err)
	}

	browser := rod.New().ControlURL(urlStr).MustConnect()
	page := browser.MustPage()
	err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  cfg.Rod.DefaultPageWidth,
		Height: cfg.Rod.DefaultPageHeight,
	})
	if err != nil {
		return nil, fmt.Errorf("设置视口失败: %v", err)
	}
	router := page.HijackRequests()
	return &rodCrawler{
		browser: browser,
		page:    page,
		router:  router,
	}, nil
}
func (rc *rodCrawler) PageContext() context.Context {
	return rc.page.GetContext()
}

func (rc *rodCrawler) Close() {
	rc.router.MustStop()
	rc.browser.MustClose()
}

func (rc *rodCrawler) InitAndNavigate(url string) error {
	go rc.router.Run()
	err := rc.page.Navigate(url)
	if err != nil {
		return err
	}
	// 等待页面加载完成
	err = rc.page.WaitLoad()
	if err != nil {
		return fmt.Errorf("等待页面加载失败: %w", err)
	}

	// 等待更长时间确保JavaScript环境就绪
	rc.page.MustWaitStable()
	return nil
}

func (rc *rodCrawler) PerformScrolling(scrollTimes, standardSleepSeconds, randomDelaySeconds int) error {
	fmt.Println("开始执行滑动操作...")

	// 等待页面完全加载
	err := rc.page.WaitLoad()
	if err != nil {
		return fmt.Errorf("等待页面加载失败: %v", err)
	}

	// 等待页面稳定
	rc.page.MustWaitStable()
	time.Sleep(2 * time.Second)

	// 创建本地随机数生成器
	localRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := range scrollTimes {
		// 获取页面高度
		height, err := rc.page.Eval(`() => document.body.scrollHeight`)
		if err != nil {
			return fmt.Errorf("获取页面高度失败: %v", err)
		}

		// 计算目标滚动位置（随机滚动到 80%-95% 位置）
		totalHeight := height.Value.Int()
		currentScroll := float64(totalHeight) * (0.7 + localRand.Float64()*0.25)

		// 使用 Rod 的 API 滚动
		err = rc.page.Mouse.Scroll(0, currentScroll, 1)
		if err != nil {
			for range 3 {
				err = rc.page.KeyActions().Press(input.AddKey("PageDown", "", "PageDown", 34, 0)).Do()
				if err != nil {
					return fmt.Errorf("执行 PageDown 失败: %v", err)
				}
			}
		}

		fmt.Printf("第 %d 次滑动完成，目标位置: %f\n", i+1, currentScroll)

		// 随机延迟
		randomDelay := time.Duration(localRand.Float64() * float64(randomDelaySeconds) * float64(time.Second))
		totalSleep := time.Duration(standardSleepSeconds)*time.Second + randomDelay
		fmt.Printf("等待 %.1f 秒\n", totalSleep.Seconds())
		time.Sleep(totalSleep)
	}

	return nil
}

func (rc *rodCrawler) SetNetworkListener(urlPattern string, respChan chan []types.NetworkResponse) {
	rc.router.MustAdd(urlPattern, func(hijack *rod.Hijack) {
		hijack.MustLoadResponse()
		body := hijack.Response.Body()
		//fmt.Printf("URL: %s\nResponse Body: %s\n", hijack.Request.URL(), body)
		respChan <- []types.NetworkResponse{
			{
				URL:  hijack.Request.URL().String(),
				Body: []byte(body),
			},
		}
	})
	fmt.Printf("已设置网络监听器，监听URL模式: %s\n", urlPattern)
}
