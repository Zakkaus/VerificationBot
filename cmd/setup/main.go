// VerificationBot Setup Wizard
// Run: go run ./cmd/setup
// Generates a .env file with configuration and deployment recommendations.
package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"strings"
)

var reader = bufio.NewReader(os.Stdin)

func ask(prompt, def string) string {
	if def != "" {
		fmt.Printf("  %s [%s]: ", prompt, def)
	} else {
		fmt.Printf("  %s: ", prompt)
	}
	val, _ := reader.ReadString('\n')
	val = strings.TrimSpace(val)
	if val == "" {
		return def
	}
	return val
}

func askSecret(prompt string) string {
	fmt.Printf("  %s (hidden): ", prompt)
	val, _ := reader.ReadString('\n')
	return strings.TrimSpace(val)
}

func askChoice(prompt string, choices []string, def string) string {
	for {
		fmt.Printf("  %s %v [%s]: ", prompt, choices, def)
		val, _ := reader.ReadString('\n')
		val = strings.TrimSpace(val)
		if val == "" {
			return def
		}
		for _, c := range choices {
			if val == c {
				return val
			}
		}
		fmt.Printf("  ⚠️  請輸入以下其中一個: %v\n", choices)
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func detectOS() string {
	switch runtime.GOOS {
	case "linux":
		// Check if NixOS
		if _, err := os.Stat("/etc/nixos"); err == nil {
			return "nixos"
		}
		// Check if Docker available
		if _, err := os.Stat("/var/run/docker.sock"); err == nil {
			return "linux-docker"
		}
		return "linux"
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	default:
		return "unknown"
	}
}

func main() {
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Println("║   VerificationBot 配置生成器            ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println()

	detectedOS := detectOS()
	osLabels := map[string]string{
		"nixos": "NixOS", "linux-docker": "Linux (Docker)", "linux": "Linux",
		"macos": "macOS", "windows": "Windows",
	}
	fmt.Printf("🖥️  偵測到系統: %s\n\n", osLabels[detectedOS])

	// ── Telegram ─────────────────────────────────────────────────────────────
	fmt.Println("【1/6】Telegram Bot 設定")
	token := askSecret("Bot Token（從 @BotFather 取得）")
	approveMode := askChoice("驗證模式", []string{"approve", "classic"}, "approve")
	testTime := ask("驗證超時秒數", "120")

	// ── Captcha ───────────────────────────────────────────────────────────────
	fmt.Println("\n【2/6】人機驗證設定")
	captchaType := askChoice("驗證類型（推薦 turnstile，免費且更簡單）",
		[]string{"turnstile", "recaptcha"}, "turnstile")

	var captchaAdvice string
	if captchaType == "turnstile" {
		captchaAdvice = "→ 前往 https://dash.cloudflare.com → Zero Trust → Turnstile 新增站點"
	} else {
		captchaAdvice = "→ 前往 https://www.google.com/recaptcha/admin 新增 v2 Checkbox 站點"
	}
	fmt.Println(" ", captchaAdvice)

	captchaSiteKey := askSecret("Site Key（前端顯示用）")
	captchaSecret := askSecret("Secret Key（後端驗證用）")

	// ── Mini App ──────────────────────────────────────────────────────────────
	fmt.Println("\n【3/6】Mini App URL")
	fmt.Println("  Mini App 需要 HTTPS URL，Bot 會將驗證頁面託管在 /webapp/ 路徑")
	webappURL := ask("Mini App HTTPS URL（如 https://your-domain.com/webapp/）", "")

	// ── Deployment ────────────────────────────────────────────────────────────
	fmt.Println("\n【4/6】部署模式")

	var deployMode string
	var serverHost, serverPort string

	switch detectedOS {
	case "nixos":
		fmt.Println("  💡 NixOS 建議：使用 Docker + Cloudflare Tunnel（最簡單）")
		deployMode = askChoice("部署方式", []string{"docker", "binary"}, "docker")
	case "linux-docker":
		fmt.Println("  💡 偵測到 Docker，建議使用 docker-compose")
		deployMode = askChoice("部署方式", []string{"docker", "binary"}, "docker")
	case "macos":
		fmt.Println("  💡 macOS 建議：使用 Polling 模式在本地測試")
		deployMode = "binary"
	default:
		deployMode = askChoice("部署方式", []string{"docker", "binary"}, "docker")
	}

	useWebhook := askChoice("使用 Webhook（需要域名，延遲更低）還是 Polling？",
		[]string{"webhook", "polling"}, func() string {
			if detectedOS == "macos" {
				return "polling"
			}
			return "webhook"
		}())

	if useWebhook == "webhook" {
		serverHost = ask("你的域名（不含 https://）", "")
		serverPort = ask("Webhook 端口（443 或 8443）", "8443")
	}

	// ── Cloudflare Tunnel ─────────────────────────────────────────────────────
	var tunnelToken string
	if deployMode == "docker" {
		fmt.Println("\n【5/6】Cloudflare Tunnel（管理後台安全訪問）")
		fmt.Println("  💡 強烈推薦：Cloudflare Tunnel 可讓管理後台無需開放公網端口")
		fmt.Println("  設定步驟：cloudflared tunnel login → cloudflared tunnel create verificationbot")
		useTunnel := askChoice("使用 Cloudflare Tunnel？", []string{"yes", "no"}, "yes")
		if useTunnel == "yes" {
			tunnelToken = askSecret("Tunnel Token（cloudflared tunnel token 輸出）")
		}
	} else {
		fmt.Println("\n【5/6】跳過（非 Docker 部署）")
	}

	// ── Admin Dashboard ───────────────────────────────────────────────────────
	fmt.Println("\n【6/6】管理後台設定")
	jwtSecret := randomHex(32)
	fmt.Printf("  ✅ JWT Secret 已自動生成（32 字節隨機）\n")
	adminUser := ask("管理員帳號", "admin")
	adminPass := askSecret("管理員密碼")
	adminPort := ask("管理後台端口", "8080")

	// ── Generate .env ─────────────────────────────────────────────────────────
	outPath := ".env"
	if _, err := os.Stat(outPath); err == nil {
		fmt.Printf("\n⚠️  .env 檔案已存在，備份為 .env.bak\n")
		os.Rename(outPath, ".env.bak")
	}

	var sb strings.Builder
	sb.WriteString("# VerificationBot 設定檔\n")
	sb.WriteString("# 由 setup wizard 自動生成\n\n")

	sb.WriteString("# ── Telegram ────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("TELEGRAM_TOKEN=%s\n", token))
	sb.WriteString(fmt.Sprintf("APPROVE_MODE=%v\n", approveMode == "approve"))
	sb.WriteString(fmt.Sprintf("TEST_TIME=%s\n\n", testTime))

	sb.WriteString("# ── 人機驗證 ────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("CAPTCHA_TYPE=%s\n", captchaType))
	sb.WriteString(fmt.Sprintf("CAPTCHA_SITE_KEY=%s\n", captchaSiteKey))
	sb.WriteString(fmt.Sprintf("CAPTCHA_SECRET=%s\n\n", captchaSecret))

	sb.WriteString("# ── Mini App ────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("WEBAPP_URL=%s\n\n", webappURL))

	if serverHost != "" {
		sb.WriteString("# ── Webhook ─────────────────────────────────\n")
		sb.WriteString(fmt.Sprintf("SERVER_HOST=%s\n", serverHost))
		sb.WriteString(fmt.Sprintf("SERVER_PORT=%s\n", serverPort))
		sb.WriteString("WEBHOOK_PATH=/webhook\n\n")
	}

	sb.WriteString("# ── 管理後台 ────────────────────────────────\n")
	if deployMode == "docker" {
		sb.WriteString("ADMIN_HOST=0.0.0.0\n")
	} else {
		sb.WriteString("ADMIN_HOST=127.0.0.1\n")
	}
	sb.WriteString(fmt.Sprintf("ADMIN_PORT=%s\n", adminPort))
	sb.WriteString(fmt.Sprintf("JWT_SECRET=%s\n", jwtSecret))
	sb.WriteString(fmt.Sprintf("ADMIN_USER=%s\n", adminUser))
	sb.WriteString(fmt.Sprintf("ADMIN_PASS=%s\n\n", adminPass))

	sb.WriteString("# ── 資料庫 ──────────────────────────────────\n")
	if deployMode == "docker" {
		sb.WriteString("DB_PATH=/data/bot.db\n\n")
	} else {
		sb.WriteString("DB_PATH=/var/lib/verificationbot/bot.db\n\n")
	}

	if tunnelToken != "" {
		sb.WriteString("# ── Cloudflare Tunnel ───────────────────────\n")
		sb.WriteString(fmt.Sprintf("TUNNEL_TOKEN=%s\n\n", tunnelToken))
	}

	if err := os.WriteFile(outPath, []byte(sb.String()), 0600); err != nil {
		fmt.Printf("❌ 寫入失敗: %v\n", err)
		os.Exit(1)
	}

	// ── Recommendations ───────────────────────────────────────────────────────
	fmt.Printf("\n✅ 設定已寫入 %s\n\n", outPath)
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║   部署建議                               ║")
	fmt.Println("╚══════════════════════════════════════════╝")

	switch {
	case deployMode == "docker" && tunnelToken != "":
		fmt.Println(`
🐳 Docker + Cloudflare Tunnel 部署：

  # 啟動所有服務
  docker compose up -d

  # 配置 Cloudflare Tunnel 路由（一次性）
  cloudflared tunnel route dns verificationbot admin.your-domain.com

  # 確認狀態
  docker compose logs -f

管理後台可在 Cloudflare Tunnel 配置的域名上訪問 🎉`)

	case deployMode == "docker":
		fmt.Println(`
🐳 Docker 部署：

  docker compose up -d
  
管理後台：http://localhost:8080/admin/（用 SSH Tunnel 訪問）
  ssh -L 8080:localhost:8080 user@your-server`)

	case detectedOS == "nixos":
		fmt.Println(`
❄️  NixOS 部署：

  # 將以下加入 configuration.nix（詳見 README.md）
  virtualisation.docker.enable = true;
  
  # 啟動
  sudo nixos-rebuild switch
  sudo docker compose -f /opt/verificationbot/docker-compose.yml up -d`)

	default:
		fmt.Println(`
🖥️  本地/Binary 部署：

  # 建置
  go build -o bot .
  
  # 啟動
  ./bot
  
  # 管理後台：http://localhost:8080/admin/`)
	}
	fmt.Println()
}
