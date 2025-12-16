package chrome

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

type rodCrawler struct {
	browser *rod.Browser
	page    *rod.Page
}

func InitRodCrawler() ChromeCrawler {
	url := launcher.New().
		Headless(false).             // 显示浏览器界面
		Set("disable-web-security"). // 禁用同源策略
		MustLaunch()

	browser := rod.New().ControlURL(url).MustConnect()
	return &rodCrawler{
		browser: browser,
	}
}
func (rc *rodCrawler) PageContext() context.Context {
	return rc.page.GetContext()
}

func (rc *rodCrawler) Close() {
	rc.browser.MustClose()
}

func (rc *rodCrawler) InitAndNavigate(url string) error {
	var err error
	rc.page, err = rc.browser.Page(proto.TargetCreateTarget{
		URL: url,
	})
	if err != nil {
		return err
	}
	return nil
}

func (rc *rodCrawler) PerformScrolling(scrollTimes, standardSleepSeconds, randomDelaySeconds int) error {
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
			_ = rc.page.MustEval(js, nil)
			fmt.Printf("第 %d 次滑动: 到底部\n", i+1)
		case 1:
			// 滑动到随机比例
			ratio := 0.7 + localRand.Float64()*0.3 // 70%-100% 位置
			js := fmt.Sprintf(`window.scrollTo({
					top: document.body.scrollHeight * %f,
					behavior: 'smooth'
				});`, ratio)
			_ = rc.page.MustEval(js, nil)
			fmt.Printf("第 %d 次滑动: 到 %.0f%% 位置\n", i+1, ratio*100)
		}

		randomDelay := time.Duration(localRand.Float64() * float64(randomDelaySeconds) * float64(time.Second))
		totalSleep := time.Duration(standardSleepSeconds)*time.Second + randomDelay

		fmt.Printf("等待 %.1f 秒\n", totalSleep.Seconds())
		time.Sleep(totalSleep)
	}

	// 最终滑动和等待
	finalJS := `window.scrollTo({top: document.body.scrollHeight, behavior: 'smooth'});`
	_ = rc.page.MustEval(finalJS, nil)

	finalSleep := 2 * time.Duration(randomDelaySeconds) * time.Second
	fmt.Printf("最终等待 %.1f 秒\n", finalSleep.Seconds())
	time.Sleep(finalSleep)

	fmt.Printf("完成 %d 次滑动\n", scrollTimes)
	time.Sleep(time.Duration(standardSleepSeconds*3)*time.Second + time.Duration(randomDelaySeconds*3)*time.Second)
	return nil
}
