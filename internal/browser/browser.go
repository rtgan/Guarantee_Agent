package browser

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/playwright-community/playwright-go"
)

// Page 是工具操作的浏览器抽象,基于 Playwright 真实浏览器实现。
//
// 它保留了与 HTTP 版相同的方法集(Navigate/Fill/Click/AssertText/Snapshot),
// 因此 tools/agent 层无需改动即可使用真实浏览器。Close 用于释放浏览器资源。
type Page struct {
	BaseURL string // 站点根,斜杠相对 URL 会相对它解析
	URL     string // 当前页面 URL
	Forms   map[string]string
	pw      *playwright.Playwright
	browser playwright.Browser
	context playwright.BrowserContext
	page    playwright.Page
	runDir  string // 产物目录,用于写截图
}

// NewPage 启动一个 Playwright Chromium 并返回绑定到 baseURL 的 Page。
// runDir 用于存放截图等产物;headless 控制是否无头;debug 时可传 false 便于观察。
func NewPage(baseURL, runDir string, headless bool) (*Page, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("start playwright: %w", err)
	}
	hl := headless
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: &hl})
	if err != nil {
		pw.Stop()
		return nil, fmt.Errorf("launch chromium: %w", err)
	}
	ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{Width: 1440, Height: 900},
	})
	if err != nil {
		browser.Close()
		pw.Stop()
		return nil, fmt.Errorf("new context: %w", err)
	}
	page, err := ctx.NewPage()
	if err != nil {
		browser.Close()
		pw.Stop()
		return nil, fmt.Errorf("new page: %w", err)
	}
	return &Page{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Forms:   map[string]string{},
		pw:      pw, browser: browser, context: ctx, page: page,
		runDir: runDir,
	}, nil
}

// Close 释放浏览器与 Playwright 进程资源,可重复调用。
func (p *Page) Close() error {
	if p == nil {
		return nil
	}
	var errs []error
	if p.browser != nil {
		errs = append(errs, p.browser.Close())
	}
	if p.pw != nil {
		errs = append(errs, p.pw.Stop())
	}
	return errors.Join(errs...)
}

// Navigate 解析 raw(绝对或斜杠相对)后用真实浏览器打开页面。
func (p *Page) Navigate(ctx context.Context, raw string) error {
	target, err := p.resolve(raw)
	if err != nil {
		return err
	}
	resp, err := p.page.Goto(target, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded})
	if err != nil {
		return fmt.Errorf("navigation failed: %w", err)
	}
	if resp != nil && resp.Status() >= 400 {
		return fmt.Errorf("navigation failed: %s", resp.StatusText())
	}
	p.URL = target
	return nil
}

// Fill 按目标描述找到输入框并填入文本。target 优先作为可见文本/标签/placeholder,
// 都不命中时尝试作为 CSS 选择器。空目标会被拒绝。
func (p *Page) Fill(target, text string) error {
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("fill target is required")
	}
	p.Forms[target] = text
	loc := p.locatorFor(target)
	if loc == nil {
		return fmt.Errorf("element not found for %q", target)
	}
	if err := loc.First().Fill(text); err != nil {
		return fmt.Errorf("fill %q: %w", target, err)
	}
	return nil
}

// Scroll 滚动页面。direction 为 "up"/"down"(默认 down),用鼠标滚轮事件滚动一屏。
func (p *Page) Scroll(direction string) error {
	if p.page == nil {
		return fmt.Errorf("page not ready")
	}
	delta := "window.scrollBy(0, window.innerHeight)"
	if strings.EqualFold(direction, "up") {
		delta = "window.scrollBy(0, -window.innerHeight)"
	}
	_, err := p.page.Evaluate(delta)
	return err
}

// Wait 阻塞指定毫秒。用于在异步 UI 出现前等待。
func (p *Page) Wait(milliseconds int) error {
	if milliseconds <= 0 {
		milliseconds = 500
	}
	p.page.WaitForTimeout(float64(milliseconds))
	return nil
}

// SelectOption 在 <select> 上选择某项。target 用于定位 select 元素(文本/label/CSS),
// value 按 label 匹配选项(也兼容 value)。空 target 或 value 会被拒绝。
func (p *Page) SelectOption(target, value string) error {
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("select target is required")
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("select value is required")
	}
	loc := p.locatorFor(target)
	if loc == nil {
		return fmt.Errorf("element not found for %q", target)
	}
	// 优先按 label 选,失败再按 value 选。
	labels := []string{value}
	if _, err := loc.First().SelectOption(playwright.SelectOptionValues{Labels: &labels}); err != nil {
		values := []string{value}
		if _, err := loc.First().SelectOption(playwright.SelectOptionValues{Values: &values}); err != nil {
			return fmt.Errorf("select %q=%q: %w", target, value, err)
		}
	}
	return nil
}

// Click 按目标描述点击元素。若目标是页面内链接则等价于导航;否则点击匹配元素。
func (p *Page) Click(ctx context.Context, target string) error {
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("click target is required")
	}
	loc := p.locatorFor(target)
	if loc == nil {
		return fmt.Errorf("element not found for %q", target)
	}
	if err := loc.First().Click(); err != nil {
		return fmt.Errorf("click %q: %w", target, err)
	}
	p.URL = p.page.URL()
	return nil
}

// AssertText 校验文本在页面上可见(不区分大小写)。空文本会被拒绝。
func (p *Page) AssertText(text string) error {
	if text == "" {
		return fmt.Errorf("assert text is required")
	}
	loc := p.page.GetByText(text, playwright.PageGetByTextOptions{Exact: playwright.Bool(false)})
	cnt, err := loc.Count()
	if err != nil {
		return err
	}
	if cnt == 0 {
		return fmt.Errorf("text %q not present", text)
	}
	visible, err := loc.First().IsVisible()
	if err != nil {
		return err
	}
	if !visible {
		return fmt.Errorf("text %q not visible", text)
	}
	return nil
}

// Snapshot 返回页面可见文本作为给模型的无障碍风格观察值。
// 取不到时返回哨兵字符串。
func (p *Page) Snapshot() string {
	if p.page == nil {
		return "NO_AX_SNAPSHOT_AVAILABLE"
	}
	body, err := p.page.Locator("body").InnerText()
	if err != nil || strings.TrimSpace(body) == "" {
		return "NO_AX_SNAPSHOT_AVAILABLE"
	}
	return strings.Join(strings.Fields(body), " ")
}

// Screenshot 抓取当前页面截图并写入 runDir/snapshots/<name>.png,返回文件路径。
func (p *Page) Screenshot(name string) (string, error) {
	if p.page == nil || p.runDir == "" {
		return "", nil
	}
	dir := filepath.Join(p.runDir, "snapshots")
	buf, err := p.page.Screenshot()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, name+".png")
	return path, writeFile(path, buf)
}

// resolve 把绝对或斜杠相对 URL 解析为相对 BaseURL 的完整 URL。
func (p *Page) resolve(raw string) (string, error) {
	if raw == "" || raw == "/" {
		return p.BaseURL + "/", nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.IsAbs() {
		return u.String(), nil
	}
	base, err := url.Parse(p.BaseURL + "/")
	if err != nil {
		return "", err
	}
	return base.ResolveReference(u).String(), nil
}

// locatorFor 按目标描述尝试多种定位策略,返回第一个命中的 Locator,都不命中返回 nil。
// 顺序:精确文本 → 角色 → label → placeholder → CSS 选择器。
func (p *Page) locatorFor(target string) playwright.Locator {
	clean := strings.TrimSpace(strings.Trim(target, "`\"'"))
	if clean == "" {
		return nil
	}
	// 1) 精确文本
	if loc := p.page.GetByText(clean, playwright.PageGetByTextOptions{Exact: playwright.Bool(true)}); loc != nil {
		if n, _ := loc.Count(); n > 0 {
			return loc
		}
	}
	// 2) label
	if loc := p.page.GetByLabel(clean, playwright.PageGetByLabelOptions{Exact: playwright.Bool(true)}); loc != nil {
		if n, _ := loc.Count(); n > 0 {
			return loc
		}
	}
	// 3) placeholder
	if loc := p.page.GetByPlaceholder(clean, playwright.PageGetByPlaceholderOptions{Exact: playwright.Bool(true)}); loc != nil {
		if n, _ := loc.Count(); n > 0 {
			return loc
		}
	}
	// 4) 模糊文本
	if loc := p.page.GetByText(clean, playwright.PageGetByTextOptions{Exact: playwright.Bool(false)}); loc != nil {
		if n, _ := loc.Count(); n > 0 {
			return loc
		}
	}
	// 5) CSS 选择器兜底
	if loc := p.page.Locator(clean); loc != nil {
		if n, _ := loc.Count(); n > 0 {
			return loc
		}
	}
	return nil
}

// Bool 返回指向 v 的指针,便于传给需要 *bool 的 Playwright 选项。
func Bool(v bool) *bool { return &v }
