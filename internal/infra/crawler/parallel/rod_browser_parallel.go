package parallel

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"sync"
	"time"

	"github.com/LouYuanbo1/crawleragent/internal/config"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/options"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/types"
	"github.com/LouYuanbo1/crawleragent/param"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

type rodBrowserPoolCrawler struct {
	browserPool        rod.Pool[rod.Browser]
	createBrowser      func() (*rod.Browser, error)
	controlURLCh       chan string
	networkResponseChs []chan *types.NetworkResponse
}

func InitRodBrowserPoolCrawler(cfg *config.Config, browserPoolSize int) (ParallelCrawler, error) {
	controlURLCh := make(chan string, browserPoolSize)
	for instanceID := range browserPoolSize {

		instanceDataDir := fmt.Sprintf("%s/instance_%d", cfg.Rod.UserDataDir, instanceID)
		err := os.MkdirAll(instanceDataDir, 0755)
		if err != nil {
			return nil, fmt.Errorf("创建实例数据目录失败: %v", err)
		}

		port := cfg.Rod.BasicRemoteDebuggingPort + instanceID
		url := options.CreateLauncher(cfg.Rod.UserMode,
			options.WithBin(cfg.Rod.Bin),
			options.WithUserDataDir(instanceDataDir),
			options.WithHeadless(cfg.Rod.Headless),
			options.WithDisableBlinkFeatures(cfg.Rod.DisableBlinkFeatures),
			options.WithIncognito(cfg.Rod.Incognito),
			options.WithDisableDevShmUsage(cfg.Rod.DisableDevShmUsage),
			options.WithNoSandbox(cfg.Rod.NoSandbox),
			options.WithUserAgent(cfg.Rod.UserAgent),
			options.WithLeakless(cfg.Rod.Leakless),
			options.WithDisableBackgroundNetworking(cfg.Rod.DisableBackgroundNetworking),
			options.WithDisableBackgroundTimerThrottling(cfg.Rod.DisableBackgroundTimerThrottling),
			options.WithRemoteDebuggingPort(cfg.Rod.BasicRemoteDebuggingPort+port),
		)
		urlStr, err := url.Launch()
		if err != nil {
			return nil, fmt.Errorf("启动浏览器失败: %v", err)
		}

		log.Printf("浏览器可以连接的URL: %s", urlStr)
		controlURLCh <- urlStr
	}
	close(controlURLCh)
	// 创建页面池
	BrowserPool := rod.NewBrowserPool(browserPoolSize)

	createBrowser := func() (*rod.Browser, error) {
		// 从 controlURLCh 中获取 URL
		urlStr := <-controlURLCh
		browser := rod.
			New().
			ControlURL(urlStr).
			Trace(cfg.Rod.Trace) // 开启 CDP 通信追踪（日志会输出请求/响应）
		if err := browser.Connect(); err != nil {
			return nil, fmt.Errorf("连接浏览器失败: %v", err)
		}
		return browser, nil
	}

	networkResponseChs := make([]chan *types.NetworkResponse, 0, browserPoolSize)

	return &rodBrowserPoolCrawler{
		browserPool:        BrowserPool,
		createBrowser:      createBrowser,
		controlURLCh:       controlURLCh,
		networkResponseChs: networkResponseChs,
	}, nil
}

func (rppc *rodBrowserPoolCrawler) Close() {
	log.Printf("开始关闭，停止接收新请求...")

	// 2. 等待一段时间让正在进行的请求完成
	time.Sleep(3 * time.Second) // 可以根据实际情况调整

	// 关闭所有监听管道
	log.Printf("关闭 %d 个监听管道", len(rppc.networkResponseChs))
	for _, ch := range rppc.networkResponseChs {
		close(ch)
	}
	log.Printf("关闭 %d 个浏览器连接", len(rppc.browserPool))
	rppc.browserPool.Cleanup(func(b *rod.Browser) { b.MustClose() })
}

func (rppc *rodBrowserPoolCrawler) PerformAllUrlOperations(ctx context.Context, operations []*param.UrlOperation) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 过滤无效操作
	validOperations := rppc.operationsChecker(operations)

	for _, op := range validOperations {
		if op.ListenerConfig != nil {
			rppc.networkResponseChs = append(rppc.networkResponseChs, op.ListenerConfig.ListenerCh)
		}
	}

	operationCh := make(chan *param.UrlOperation, len(validOperations))
	for _, op := range validOperations {
		operationCh <- op
	}
	close(operationCh)

	errCh := make(chan error, max(len(validOperations), len(rppc.browserPool)))

	wg := sync.WaitGroup{}
	for i := range min(len(rppc.browserPool), len(validOperations)) {
		wg.Add(1)
		go func(ctx context.Context, workerID int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done(): // 主动监听 ctx 取消
					log.Printf("worker %d 取消执行，退出", workerID)
					return
				case op, ok := <-operationCh: // 读取任务
					if !ok { // 通道关闭则退出
						return
					}
					rppc.processUrlOperation(ctx, workerID, errCh, op)
				}
			}
		}(ctx, i)
	}
	wg.Wait()

	close(errCh)
	// 收集错误
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d errors occurred: %v", len(errs), errs)
	}
	return nil
}

func (rppc *rodBrowserPoolCrawler) operationsChecker(operations []*param.UrlOperation) []*param.UrlOperation {
	validOperations := make([]*param.UrlOperation, 0, len(operations))
	for _, op := range operations {
		if op.IsValid() {
			validOperations = append(validOperations, op)
		} else {
			log.Printf("无效的操作参数,已经跳过: %v", op)
		}
	}
	return validOperations
}

func (rppc *rodBrowserPoolCrawler) processUrlOperation(ctx context.Context, workerID int, errCh chan<- error, operation *param.UrlOperation) {
	browser, err := rppc.browserPool.Get(rppc.createBrowser)
	if err != nil {
		errCh <- fmt.Errorf("获取浏览器失败: %v", err)
		return
	}
	// 设置所有网络监听器
	router := rppc.setNetListener(ctx, browser, operation.ListenerConfig)
	go func() {
		router.Run()
		log.Printf("Worker %d 路由器停止运行", workerID)
	}()

	page, err := stealth.Page(browser)
	if err != nil {
		errCh <- fmt.Errorf("获取页面失败: %v", err)
		return
	}
	// 确保页面放回池中
	defer func() {
		log.Printf("Worker %d 路由器停止运行", workerID)
		router.Stop()
		log.Printf("Worker %d 页面关闭", workerID)
		page.MustClose()
		log.Printf("将 browser %d 返回池，处理的URL模式: %s", workerID, operation.ListenerConfig.UrlPatterns)
		rppc.browserPool.Put(browser)
	}()

	err = rppc.navigateURL(page, workerID, operation.Url)
	if err != nil {
		errCh <- fmt.Errorf("处理URL失败: %v", err)
		return
	}

	//time.Sleep(120 * time.Second)
	switch operation.OperationType {
	case param.OperationClick:
		err = rppc.performClick(page, operation)
		if err != nil {
			errCh <- fmt.Errorf("点击操作失败: %v", err)
			return
		}
	case param.OperationXClick:
		err = rppc.performXClick(page, operation)
		if err != nil {
			errCh <- fmt.Errorf("xPath点击操作失败: %v", err)
			return
		}
	case param.OperationScroll:
		err = rppc.performScrolling(page, operation)
		if err != nil {
			errCh <- fmt.Errorf("滚动操作失败: %v", err)
			return
		}
	default:
		errCh <- fmt.Errorf("未知操作类型: %v", operation.OperationType)
		return
	}
}

func (rppc *rodBrowserPoolCrawler) navigateURL(page *rod.Page, workerID int, url string) error {
	// 导航到指定URL
	fmt.Printf("Worker %d 处理: %s\n", workerID, url)

	err := page.Navigate(url)
	if err != nil {
		return fmt.Errorf("导航失败: %v", err)
	}

	page.MustWaitStable()
	time.Sleep(2 * time.Second)

	return nil
}

func (rppc *rodBrowserPoolCrawler) performClick(page *rod.Page, operation *param.UrlOperation) error {
	randomDelay := rand.Float64() * float64(operation.RandomDelaySeconds)
	totalSleep := time.Duration((float64(operation.StandardSleepSeconds) + randomDelay) * float64(time.Second))

	element, err := page.Element(operation.ClickSelector)
	if err != nil {
		return fmt.Errorf("查找元素失败: %v", err)
	}
	for range operation.NumActions {

		err = element.Click(proto.InputMouseButtonLeft, 1)
		if err != nil {
			return fmt.Errorf("点击失败: %v", err)
		}

		page.WaitRequestIdle(time.Second, operation.ListenerConfig.UrlPatterns, nil, []proto.NetworkResourceType{proto.NetworkResourceTypeDocument})

		time.Sleep(totalSleep)
	}

	return nil
}

func (rppc *rodBrowserPoolCrawler) performXClick(page *rod.Page, operation *param.UrlOperation) error {
	randomDelay := rand.Float64() * float64(operation.RandomDelaySeconds)
	totalSleep := time.Duration((float64(operation.StandardSleepSeconds) + randomDelay) * float64(time.Second))

	for range operation.NumActions {

		element, err := page.ElementX(operation.ClickSelector)
		if err != nil {
			return fmt.Errorf("查找元素失败: %v", err)
		}
		err = element.Click(proto.InputMouseButtonLeft, 1)
		if err != nil {
			return fmt.Errorf("点击失败: %v", err)
		}

		page.WaitRequestIdle(time.Second, operation.ListenerConfig.UrlPatterns, nil, []proto.NetworkResourceType{proto.NetworkResourceTypeDocument})

		time.Sleep(totalSleep)
	}

	return nil
}

func (rppc *rodBrowserPoolCrawler) performScrolling(page *rod.Page, operation *param.UrlOperation) error {
	fmt.Println("开始执行滚动任务...")

	randomDelay := rand.Float64() * float64(operation.RandomDelaySeconds)
	totalSleep := time.Duration((float64(operation.StandardSleepSeconds) + randomDelay) * float64(time.Second))

	for i := range operation.NumActions {

		_, _ = page.Eval(`() => {
			Object.defineProperty(navigator, 'webdriver', {get: () => undefined});
			Object.defineProperty(window, 'chrome', {value: {runtime: {}}});
		}`)
		// 获取页面高度
		height, err := page.Eval(`() => document.body.scrollHeight`)
		if err != nil {
			return fmt.Errorf("获取页面高度失败: %v", err)
		}

		// 计算目标滚动位置（随机滚动到 80%-95% 位置）
		totalHeight := height.Value.Int()
		currentScroll := float64(totalHeight) * (0.7 + rand.Float64()*0.25)

		_, err = page.Eval(fmt.Sprintf(`() => {
            window.scrollTo({
                top: %f,
                behavior: 'smooth'
            });
        }`, currentScroll))
		if err != nil {
			log.Printf("执行Js滚动失败: %v", err)
			// 使用 Rod 的 API 滚动
			err = page.Mouse.Scroll(0, currentScroll, 1)
			if err != nil {
				log.Printf("执行鼠标滚动失败: %v", err)
				for range 3 {
					err = page.KeyActions().Press(input.AddKey("PageDown", "", "PageDown", 34, 0)).Do()
					if err != nil {
						return fmt.Errorf("执行 PageDown 失败: %v", err)
					}
				}
			}
		}

		fmt.Printf("第 %d 次滚动完成，目标位置: %f\n", i+1, currentScroll)

		page.WaitRequestIdle(time.Second, operation.ListenerConfig.UrlPatterns, nil, []proto.NetworkResourceType{proto.NetworkResourceTypeDocument})

		time.Sleep(totalSleep)

	}

	fmt.Println("滚动任务完成")
	return nil
}

func (rppc *rodBrowserPoolCrawler) setNetListener(ctx context.Context, browser *rod.Browser, listener *param.ListenerConfig) *rod.HijackRouter {
	router := browser.HijackRequests()
	for _, urlPattern := range listener.UrlPatterns {
		router.MustAdd(urlPattern, func(hijack *rod.Hijack) {
			select {
			case <-ctx.Done():
				return
			default:
			}
			hijack.MustLoadResponse()
			body := hijack.Response.Body()
			listener.ListenerCh <- &types.NetworkResponse{
				Url:        hijack.Request.URL().String(),
				UrlPattern: urlPattern,
				Body:       []byte(body),
			}
		})
	}

	return router
}
