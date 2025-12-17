package config

import "net/http/cookiejar"

type Config struct {
	Elasticsearch struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Address  string `json:"address"`
	} `json:"elasticsearch"`

	Rod struct {
		UserDataDir          string `json:"user_data_dir"`
		Headless             bool   `json:"headless"`
		DisableBlinkFeatures string `json:"disable_blink_features"`
		Incognito            bool   `json:"incognito"`
		DisableDevShmUsage   bool   `json:"disable_dev_shm_usage"`
		NoSandbox            bool   `json:"no_sandbox"`
		UserAgent            string `json:"user_agent"`
		Leakless             bool   `json:"leakless"`
		Bin                  string `json:"bin"`
	} `json:"rod"`

	Chromedp struct {
		LifeTime             int    `json:"life_time"`
		UserDataDir          string `json:"user_data_dir"`
		Headless             bool   `json:"headless"`
		DisableBlinkFeatures string `json:"disable_blink_features"`
		Incognito            bool   `json:"incognito"`
		DisableDevShmUsage   bool   `json:"disable_dev_shm_usage"`
		NoSandbox            bool   `json:"no_sandbox"`
		UserAgent            string `json:"user_agent"`
	} `json:"chromedp"`

	Colly struct {
		AllowedDomains   []string           `json:"allowed_domains"`
		MaxDepth         int                `json:"max_depth"`
		UserAgent        string             `json:"user_agent"`
		IgnoreRobotsTxt  bool               `json:"ignore_robots_txt"`
		Async            bool               `json:"async"`
		Parallelism      int                `json:"parallelism"`
		Delay            int                `json:"delay"`
		RandomDelay      int                `json:"random_delay"`
		EnableCookieJar  bool               `json:"enable_cookie_jar"`
		CookieJarOptions *cookiejar.Options `json:"cookie_jar_options"`
	} `json:"colly"`

	Embedder struct {
		Host      string `json:"host"`
		Port      int    `json:"port"`
		Model     string `json:"model"`
		BatchSize int    `json:"batch_size"`
	} `json:"embedder"`
	LLM struct {
		Host  string `json:"host"`
		Port  int    `json:"port"`
		Model string `json:"model"`
	} `json:"llm"`
}
