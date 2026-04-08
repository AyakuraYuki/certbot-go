package log

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"golang.org/x/term"
)

var (
	levelPrinters map[zerolog.Level]*color.Color
)

func init() {
	levelPrinters = make(map[zerolog.Level]*color.Color)
	for level, v := range zerolog.LevelColors {
		levelPrinters[level] = color.New(color.Attribute(v))
	}
}

// isColorSupported 判断了当前运行程序的环境能不能打印颜色
func isColorSupported() bool {
	// 1. 遵循 NO_COLOR 标准约定 (https://no-color.org/)
	if _, exists := os.LookupEnv("NO_COLOR"); exists {
		return false
	}

	// 2. TERM=dumb 明确表示终端不支持颜色
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}

	// 3. 检测 systemd 托管环境（INVOCATION_ID 由 systemd 注入）
	if os.Getenv("INVOCATION_ID") != "" {
		return false
	}

	// 4. 检测 Kubernetes 环境
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return false
	}

	// 5. 检测 Docker 容器（/.dockerenv 是 Docker 注入的标志文件）
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return false
	}

	// 6. 最核心的判断：stdout 是否是真实 TTY
	//    systemd / supervisor / 管道重定向 / k8s pod 均不是 TTY
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return false
	}

	return true
}

func formatMessage(i any) string {
	return fmt.Sprintf("%s  ", i)
}

func formatLevel(i any) string {
	var (
		label   = "???"
		text    string
		ll      string
		level   zerolog.Level
		printer *color.Color
		ok      bool
	)

	if i != nil {
		if ll, ok = i.(string); ok {
			if level, _ = zerolog.ParseLevel(ll); level != zerolog.NoLevel {
				label = strings.ToUpper(ll)
			}
		}
	}

	text = fmt.Sprintf("%-5s", label)

	if noColor {
		return fmt.Sprintf("| %s |", text)
	}

	printer, ok = levelPrinters[level]
	if !ok {
		return fmt.Sprintf("| %s |", color.WhiteString(text))
	}

	return fmt.Sprintf("| %s |", printer.Sprintf("%s", text))
}

func formatFieldName(i any) string {
	if noColor {
		return fmt.Sprintf("%s:", i)
	}
	return color.CyanString("%s:", i)
}
