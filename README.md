# VerificationBot

> 基於 **Telegram** 的入群驗證機器人（Go 語言）  
> 支援 Cloudflare Turnstile / Google reCAPTCHA v2  
> 管理後台透過 **Telegram Mini App** 一鍵進入，群管理員自動獲得權限  
> 全程使用 [Claude](https://claude.ai) 4.6 Sonnet 協助開發

---

## 目錄

- [功能特色](#功能特色)
- [Telegram Bot 命令](#telegram-bot-命令)
- [快速開始（Docker）](#快速開始docker)
- [Docker Compose + Cloudflare Tunnel](#docker-compose--cloudflare-tunnel)
- [環境變數說明](#環境變數說明)
- [管理後台](#管理後台)
- [NixOS 部署](#nixos-部署)
- [本地開發](#本地開發)

---

## 功能特色

| 功能 | 說明 |
|------|------|
| 🛡️ 人機驗證 | reCAPTCHA v2 / Cloudflare Turnstile |
| ✅ Approve 模式 | 先審核申請，驗證通過後才放行 |
| 🔇 禁言模式 | 進群後靜音，驗證通過才解除 |
| 🚫 封禁 | 驗證失敗可踢出並刪所有消息 |
| 🎛️ Mini App 後台 | 群管理員私訊 `/admin` 直接進入後台 |
| 📊 操作記錄 | 驗證日誌、搜尋、CSV 匯出 |
| 🌐 Cloudflare Tunnel | 無需開放端口，安全暴露後台 |
| 🏗️ 多架構 Docker | amd64 + arm64 |

---

## Telegram Bot 命令

| 命令 | 說明 |
|------|------|
| `/start` | 查看幫助 |
| `/ping` | 確認 Bot 在線 |
| `/version` | 查看版本與模式 |
| `/admin` | 🎛️ 群管理員：開啟管理後台（Mini App） |

### `/admin` 運作原理

1. 在機器人**私訊**發送 `/admin`
2. Bot 呼叫 Telegram API 確認你是配置群組的管理員（`creator` 或 `administrator`）
3. 傳送帶有「🎛️ 開啟管理後台」按鈕的訊息
4. 點擊 → Telegram Mini App 自動登入（無需密碼）

> `creator` = superadmin，`administrator` = admin

---

## 快速開始（Docker）

### 1. 建立 `.env`

```env
TELEGRAM_TOKEN=1234567890:AAAXXX...
CAPTCHA_TYPE=turnstile          # 或 recaptcha
CAPTCHA_SITE_KEY=0x4AAAA...
CAPTCHA_SECRET=0x4SSSS...
WEBAPP_URL=https://bot.example.com/webapp/
ADMIN_PUBLIC_URL=https://bot.example.com
JWT_SECRET=your-strong-random-secret
ADMIN_PASS=your-admin-password
GROUPS=2452654588               # 群組 ID（不需要 - 前綴）
APPROVE_MODE=true
DB_PATH=/data/bot.db
```

> ℹ️ `GROUPS` 支援任意格式：`2452654588`、`-2452654588`、`-1002452654588` 均可

### 2. 執行

```bash
docker run -d \
  --name verificationbot \
  --env-file .env \
  -v ./data:/data \
  ghcr.io/zakkaus/verificationbot:latest
```

---

## Docker Compose + Cloudflare Tunnel

這是**推薦的生產部署方式**，透過 Cloudflare Tunnel 暴露 Mini App 和後台，無需開 80/443。

### 1. 取得 Cloudflare Tunnel Token

[Cloudflare Zero Trust](https://one.dash.cloudflare.com/) → Networks → Tunnels → Create Tunnel

Tunnel 設定（Public Hostname）：
| Subdomain | Service |
|-----------|---------|
| `bot.example.com` | `http://bot:8080` |

### 2. `docker-compose.yml`

```yaml
services:
  bot:
    image: ghcr.io/zakkaus/verificationbot:latest
    env_file: .env
    volumes:
      - ./data:/data
    restart: unless-stopped

  cloudflared:
    image: cloudflare/cloudflared:latest
    command: tunnel run
    environment:
      - TUNNEL_TOKEN=${TUNNEL_TOKEN}
    depends_on:
      - bot
    restart: unless-stopped
```

### 3. `.env` 追加

```env
TUNNEL_TOKEN=eyJhI...（Cloudflare Tunnel Token）
```

### 4. 啟動

```bash
docker compose up -d
```

---

## 環境變數說明

| 變數 | 必填 | 預設值 | 說明 |
|------|------|--------|------|
| `TELEGRAM_TOKEN` | ✅ | — | Bot Token |
| `CAPTCHA_TYPE` | — | `recaptcha` | `recaptcha` 或 `turnstile` |
| `CAPTCHA_SITE_KEY` | ✅ | — | 前端 Site Key |
| `CAPTCHA_SECRET` | ✅ | — | 後端 Secret |
| `WEBAPP_URL` | ✅ | — | Mini App 的 HTTPS URL（結尾含 `/`） |
| `ADMIN_PUBLIC_URL` | ✅ | — | 後台的 HTTPS URL（用於 `/admin` 指令） |
| `JWT_SECRET` | ✅ | — | JWT 簽名密鑰（隨機字串） |
| `ADMIN_PASS` | ✅ | — | 後台初始密碼（帳號為 `admin`） |
| `GROUPS` | — | 全部 | 逗號分隔的群組 ID 或用戶名 |
| `APPROVE_MODE` | — | `true` | `true`=申請制，`false`=直接進群後驗證 |
| `BAN` | — | `false` | 驗證失敗是否封禁 |
| `BAN_TIME` | — | `0` | 封禁秒數（0=永久） |
| `SHUTUP` | — | `true` | 驗證前禁言 |
| `TEST_TIME` | — | `120` | 驗證超時秒數 |
| `ADMIN_HOST` | — | `0.0.0.0` | 後台監聽 IP |
| `ADMIN_PORT` | — | `8080` | 後台監聽端口 |
| `DB_PATH` | — | `/data/bot.db` | SQLite 資料庫路徑 |
| `TUNNEL_TOKEN` | — | — | Cloudflare Tunnel Token |
| `PROXY` | — | — | HTTP/SOCKS5 代理 |

---

## 管理後台

後台位於 `https://your-domain.com/admin/`

### 登入方式

**方法一（推薦）：Telegram Mini App 自動登入**

1. 機器人私訊 `/admin`
2. 點擊「🎛️ 開啟管理後台」
3. 自動登入，群管理員完全不需要密碼

**方法二：帳號密碼**

訪問 `https://your-domain.com/admin/login`
- 帳號：`admin`（可改）
- 密碼：`ADMIN_PASS` 環境變數

### 後台功能

| 頁面 | 功能 |
|------|------|
| 📊 儀表板 | 今日驗證統計、監管群組狀態、最近記錄 |
| 📋 操作記錄 | 搜尋/篩選/日期範圍/匯出 CSV |
| ⚙️ 設定 | 網站名稱/Logo/主色/驗證類型/API Key |
| 👥 帳號管理 | 新增/刪除管理員（超管限定） |

---

## 驗證流程

### Approve 模式（推薦）

```
用戶申請加入群組
    ↓
Bot 私訊：驗證鍵盤（立即顯示，無需點擊連結）
    ↓
用戶點擊「點擊驗證 ✅」→ 開啟 Captcha Mini App
    ↓
完成驗證 → Bot 自動批准申請 → 用戶進群
```

### 直接進群模式

```
用戶加入群組 → 群內提示 → callback 按鈕跳轉 Bot → 完成驗證
```

---

## NixOS 部署

NixOS 推薦透過 Docker 部署：

```bash
# 安裝 Docker
nix-env -iA nixpkgs.docker
systemctl enable --now docker

# 使用 docker compose
git clone https://github.com/Zakkaus/VerificationBot
cd VerificationBot
cp .env.example .env
# 編輯 .env ...
docker compose up -d
```

詳細 NixOS 模組設定見 [`deploy/nixos.nix`](deploy/nixos.nix)。

---

## 本地開發

**需求：** Go 1.22+，ngrok 或 localtunnel（用於 HTTPS）

```bash
git clone https://github.com/Zakkaus/VerificationBot
cd VerificationBot

# 啟動 localtunnel 暴露 8080
npx localtunnel --port 8080 &

# 複製環境變數模板
cp .env.example .env
# 編輯 .env 填入 Token / Captcha Key 等

# 編譯並啟動
go build -o bot .
./bot \
  --telegram-token "YOUR_TOKEN" \
  --captcha-type recaptcha \
  --captcha-site-key "YOUR_SITE_KEY" \
  --captcha-secret "YOUR_SECRET" \
  --webapp-url "https://xxx.loca.lt/webapp/" \
  --admin-public-url "https://xxx.loca.lt" \
  --jwt-secret "dev-secret-xyz" \
  --admin-pass "admin123" \
  --groups "YOUR_GROUP_ID" \
  --approve-mode \
  --admin-host "127.0.0.1"
```

### 多架構 Docker 構建

```bash
docker buildx create --use
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t ghcr.io/zakkaus/verificationbot:latest \
  --push .
```

---

## 人機驗證選擇

| | Cloudflare Turnstile | Google reCAPTCHA v2 |
|--|--|--|
| 用戶體驗 | ✅ 無感驗證 | ⚠️ 需完整點擊 |
| 免費配額 | ✅ 無限制 | ✅ 有限額 |
| 推薦程度 | ⭐⭐⭐ 推薦 | ⭐⭐ 備用 |

申請地址：
- Turnstile：[dash.cloudflare.com](https://dash.cloudflare.com/?to=/:account/turnstile)
- reCAPTCHA：[google.com/recaptcha](https://www.google.com/recaptcha/admin)