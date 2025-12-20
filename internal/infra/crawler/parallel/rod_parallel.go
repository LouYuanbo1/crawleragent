package parallel

import (
	"context"
	"fmt"
	"log"
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

func (rppc *rodPagePoolCrawler) PerformAllUrlOperations(ctx context.Context, operations []*param.UrlOperation) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// 过滤无效操作
	validOperations := rppc.operationsChecker(operations)

	// 设置所有网络监听器
	rppc.setAllNetListener(validOperations)
	go rppc.browserRouter.Run()

	operationCh := make(chan *param.UrlOperation, len(validOperations))
	for _, op := range validOperations {
		operationCh <- op
	}
	close(operationCh)

	errCh := make(chan error, max(len(validOperations), len(rppc.pagePool)))

	wg := sync.WaitGroup{}
	for i := range min(len(rppc.pagePool), len(validOperations)) {
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
					rppc.processUrlOperation(workerID, errCh, op)
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

func (rppc *rodPagePoolCrawler) operationsChecker(operations []*param.UrlOperation) []*param.UrlOperation {
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

func (rppc *rodPagePoolCrawler) processUrlOperation(workerID int, errCh chan<- error, operation *param.UrlOperation) {

	page, err := rppc.pagePool.Get(rppc.createPage)
	if err != nil {
		errCh <- fmt.Errorf("获取页面失败: %v", err)
		return
	}
	// 确保页面放回池中
	defer func() {
		rppc.pagePool.Put(page)
		close(operation.Listener.RespChan)
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

	page.MustWaitStable()
	time.Sleep(2 * time.Second)

	return nil
}

func (rppc *rodPagePoolCrawler) performClick(page *rod.Page, operation *param.UrlOperation) error {
	randomDelay := rand.Float64() * float64(operation.RandomDelaySeconds)
	totalSleep := time.Duration((float64(operation.StandardSleepSeconds) + randomDelay) * float64(time.Second))

	element, err := page.Element(operation.Selector)
	if err != nil {
		return fmt.Errorf("查找元素失败: %v", err)
	}
	for range operation.NumActions {

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

func (rppc *rodPagePoolCrawler) performXClick(page *rod.Page, operation *param.UrlOperation) error {
	randomDelay := rand.Float64() * float64(operation.RandomDelaySeconds)
	totalSleep := time.Duration((float64(operation.StandardSleepSeconds) + randomDelay) * float64(time.Second))

	for range operation.NumActions {

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

func (rppc *rodPagePoolCrawler) performScrolling(page *rod.Page, operation *param.UrlOperation) error {
	fmt.Println("开始执行滚动任务...")
	/*
		randomDelay := rand.Float64() * float64(operation.RandomDelaySeconds)
		totalSleep := time.Duration((float64(operation.StandardSleepSeconds) + randomDelay) * float64(time.Second))
	*/
	for i := range operation.NumActions {

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

func (rppc *rodPagePoolCrawler) setAllNetListener(options []*param.UrlOperation) {
	for _, option := range options {
		rppc.setNetListener(option.Listener)
	}
}

func (rppc *rodPagePoolCrawler) setNetListener(listener *param.ListenerConfig) {
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
