package parallel

import (
	"fmt"
	"math/rand/v2"
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

type rodPagePoolCrawler struct {
	browser       *rod.Browser
	pagePool      rod.Pool[rod.Page]
	createPage    func() (*rod.Page, error)
	browserRouter *rod.HijackRouter
}

func InitRodPagePoolCrawler(cfg *config.Config, pagePoolSize int) (ParallelCrawler, error) {
	url := options.CreateLauncher(cfg.Rod.UserMode,
		options.WithBin(cfg.Rod.Bin),
		options.WithUserDataDir(cfg.Rod.UserDataDir),
		options.WithHeadless(cfg.Rod.Headless),
		options.WithDisableBlinkFeatures(cfg.Rod.DisableBlinkFeatures),
		options.WithIncognito(cfg.Rod.Incognito),
		options.WithDisableDevShmUsage(cfg.Rod.DisableDevShmUsage),
		options.WithNoSandbox(cfg.Rod.NoSandbox),
		options.WithUserAgent(cfg.Rod.UserAgent),
		options.WithLeakless(cfg.Rod.Leakless),
		options.WithDisableBackgroundNetworking(cfg.Rod.DisableBackgroundNetworking),
		options.WithDisableBackgroundTimerThrottling(cfg.Rod.DisableBackgroundTimerThrottling),
	)
	urlStr, err := url.Launch()
	if err != nil {
		return nil, fmt.Errorf("启动浏览器失败: %v", err)
	}

	browser := rod.New().ControlURL(urlStr).MustConnect()

	// 创建页面池
	pagePool := rod.NewPagePool(pagePoolSize)

	createPage := func() (*rod.Page, error) {
		return stealth.Page(browser)
	}

	// 创建浏览器路由
	browserRouter := browser.HijackRequests()

	return &rodPagePoolCrawler{
		browser:       browser,
		pagePool:      pagePool,
		createPage:    createPage,
		browserRouter: browserRouter,
	}, nil
}

func (rppc *rodPagePoolCrawler) Close() {
	// 关闭浏览器路由
	rppc.browserRouter.MustStop()
	rppc.pagePool.Cleanup(func(p *rod.Page) { p.MustClose() })
	rppc.browser.MustClose()
}

func (rppc *rodPagePoolCrawler) PerformOpentionsALL(operations []*param.URLOperation) error {

	// 设置所有网络监听器
	rppc.setAllNetListener(operations)
	go rppc.browserRouter.Run()

	operationCh := make(chan *param.URLOperation, len(operations))
	for _, url := range operations {
		operationCh <- url
	}
	close(operationCh)

	errCh := make(chan error, max(len(operations), len(rppc.pagePool)))

	wg := sync.WaitGroup{}
	for i := range len(rppc.pagePool) {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for op := range operationCh {
				rppc.processUrlOperation(workerID, errCh, op)
			}
		}(i)
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

func (rppc *rodPagePoolCrawler) processUrlOperation(workerID int, errCh chan<- error, operation *param.URLOperation) {
	page, err := rppc.pagePool.Get(rppc.createPage)
	if err != nil {
		errCh <- fmt.Errorf("获取页面失败: %v", err)
		return
	}
	// 确保页面放回池中
	defer func() {
		rppc.pagePool.Put(page)
		if operation.Listener != nil {
			close(operation.Listener.RespChan)
		}
	}()

	err = rppc.navigateURL(page, workerID, operation.Url)
	if err != nil {
		errCh <- fmt.Errorf("处理URL失败: %v", err)
		return
	}

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

func (rppc *rodPagePoolCrawler) navigateURL(page *rod.Page, workerID int, url string) error {

	// 导航到指定URL
	fmt.Printf("Worker %d 处理: %s\n", workerID, url)

	err := page.Navigate(url)
	if err != nil {
		return fmt.Errorf("导航失败: %v", err)
	}

	// 3. 等待页面加载
	err = page.WaitLoad()
	if err != nil {
		return fmt.Errorf("等待加载失败: %v", err)
	}

	page.MustWaitStable()
	time.Sleep(2 * time.Second)

	return nil
}

func (rppc *rodPagePoolCrawler) performClick(page *rod.Page, operation *param.URLOperation) error {
	randomDelay := rand.Float64() * float64(operation.RandomDelaySeconds)
	totalSleep := time.Duration((float64(operation.StandardSleepSeconds) + randomDelay) * float64(time.Second))

	element, err := page.Element(operation.Selector)
	if err != nil {
		return fmt.Errorf("查找元素失败: %v", err)
	}
	for range operation.Times {

		page.MustActivate()

		err = element.Click(proto.InputMouseButtonLeft, 1)
		if err != nil {
			return fmt.Errorf("点击失败: %v", err)
		}
		// 等待页面稳定
		page.MustWaitStable()
		time.Sleep(totalSleep)
	}

	return nil
}

func (rppc *rodPagePoolCrawler) performXClick(page *rod.Page, operation *param.URLOperation) error {
	randomDelay := rand.Float64() * float64(operation.RandomDelaySeconds)
	totalSleep := time.Duration((float64(operation.StandardSleepSeconds) + randomDelay) * float64(time.Second))

	for range operation.Times {

		page.MustActivate()

		element, err := page.ElementX(operation.Selector)
		if err != nil {
			return fmt.Errorf("查找元素失败: %v", err)
		}
		err = element.Click(proto.InputMouseButtonLeft, 1)
		if err != nil {
			return fmt.Errorf("点击失败: %v", err)
		}
		// 等待页面稳定
		page.MustWaitStable()
		time.Sleep(totalSleep)
	}

	return nil
}

func (rppc *rodPagePoolCrawler) performScrolling(page *rod.Page, operation *param.URLOperation) error {
	fmt.Println("开始执行滚动任务...")
	/*
		randomDelay := rand.Float64() * float64(operation.RandomDelaySeconds)
		totalSleep := time.Duration((float64(operation.StandardSleepSeconds) + randomDelay) * float64(time.Second))
	*/
	for i := range operation.Times {

		page.MustActivate()

		// 获取页面高度
		height, err := page.Eval(`() => document.body.scrollHeight`)
		if err != nil {
			return fmt.Errorf("获取页面高度失败: %v", err)
		}

		// 计算目标滚动位置（随机滚动到 80%-95% 位置）
		totalHeight := height.Value.Int()
		currentScroll := float64(totalHeight) * (0.7 + rand.Float64()*0.25)

		// 使用 Rod 的 API 滚动
		err = page.Mouse.Scroll(0, currentScroll, 1)
		if err != nil {
			for range 3 {
				err = page.KeyActions().Press(input.AddKey("PageDown", "", "PageDown", 34, 0)).Do()
				if err != nil {
					return fmt.Errorf("执行 PageDown 失败: %v", err)
				}
			}
		}

		fmt.Printf("第 %d 次滚动完成，目标位置: %f\n", i+1, currentScroll)
		page.MustWaitStable()

		//time.Sleep(totalSleep)

	}

	fmt.Println("滚动任务完成")
	return nil
}

func (rppc *rodPagePoolCrawler) setAllNetListener(options []*param.URLOperation) {
	for _, option := range options {
		if option.Listener != nil {
			rppc.setNetListener(option.Listener)
		}
	}
}

func (rppc *rodPagePoolCrawler) setNetListener(listener *param.Listener) {
	rppc.browserRouter.MustAdd(listener.UrlPattern, func(hijack *rod.Hijack) {
		hijack.MustLoadResponse()
		body := hijack.Response.Body()
		listener.RespChan <- []types.NetworkResponse{
			{
				URL:  hijack.Request.URL().String(),
				Body: []byte(body),
			},
		}
	})
}
