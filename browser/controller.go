package browser

import (
	"encoding/base64"
	"fmt"
	"log"

	"github.com/playwright-community/playwright-go"
)

// BrowserController manages the Playwright browser instance and current page.
type Controller struct {
	pw      *playwright.Playwright
	browser playwright.Browser
	context playwright.BrowserContext
	page    playwright.Page
}

// NewController initializes Playwright and launches Chromium.
// Set headless to true for background execution, false to see the browser.
func NewController(headless bool) (*Controller, error) {
	log.Println("[Browser] Initializing Playwright...")
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("could not start playwright: %w", err)
	}

	log.Println("[Browser] Launching Chromium...")
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
		Args: []string{
			"--no-sandbox",
			"--disable-dev-shm-usage",
			"--disable-blink-features=AutomationControlled", // Light stealth
		},
	})
	if err != nil {
		pw.Stop()
		return nil, fmt.Errorf("could not launch browser: %w", err)
	}

	opts := playwright.BrowserNewContextOptions{
		Viewport:  &playwright.Size{Width: 1280, Height: 720},
		UserAgent: playwright.String("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	}

	context, err := browser.NewContext(opts)
	if err != nil {
		browser.Close()
		pw.Stop()
		return nil, fmt.Errorf("could not create context: %w", err)
	}

	page, err := context.NewPage()
	if err != nil {
		context.Close()
		browser.Close()
		pw.Stop()
		return nil, fmt.Errorf("could not create page: %w", err)
	}

	// Auto-dismiss JavaScript dialogs (alert, confirm, prompt) so they never block automation
	page.On("dialog", func(dialog playwright.Dialog) {
		log.Printf("[Browser] Auto-dismissing dialog: type=%s message=%s", dialog.Type(), dialog.Message())
		dialog.Accept()
	})

	return &Controller{
		pw:      pw,
		browser: browser,
		context: context,
		page:    page,
	}, nil
}

// Close gracefully shuts down the browser and Playwright driver.
func (c *Controller) Close() {
	if c.page != nil {
		c.page.Close()
	}
	if c.context != nil {
		c.context.Close()
	}
	if c.browser != nil {
		c.browser.Close()
	}
	if c.pw != nil {
		c.pw.Stop()
	}
	log.Println("[Browser] Closed")
}

// getScreenshot takes a full-page screenshot and returns it as a base64 string.
func (c *Controller) getScreenshot() string {
	if c.page == nil {
		return ""
	}
	bytes, err := c.page.Screenshot(playwright.PageScreenshotOptions{
		Type:    playwright.ScreenshotTypeJpeg,
		Quality: playwright.Int(60), // keep payload size moderate
	})
	if err != nil {
		log.Printf("[Browser] Screenshot failed: %v", err)
		return ""
	}
	return base64.StdEncoding.EncodeToString(bytes)
}
