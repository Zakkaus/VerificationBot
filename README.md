# VerificationBot

Telegram 群組驗證 Bot — **Go 語言**，支援 **Google reCAPTCHA v2** 與 **Cloudflare Turnstile** 🛡️

> Built with [Antigravity](https://github.com/google-deepmind) (Google Deepmind AI assistant)

---

## 功能

| 功能 | 說明 |
|------|------|
| Approve 模式 | 申請入群 → Bot 私訊 → Mini App 驗證 → 批准 |
| 傳統模式 | 入群後禁言 → 完成驗證 → 恢復權限 |
| Cloudflare Turnstile / reCAPTCHA | 兩種人機驗證，後台可動態切換 |
| 管理後台 | 操作記錄查詢、外觀設定、RBAC 帳號管理 |
| Cloudflare Tunnel | 管理後台零端口暴露安全訪問 |
| 多架構 Docker | `linux/amd64` + `linux/arm64`，單一二進位映像 |

---

## 🐳 Docker 部署（推薦，包含 NixOS）

### 前置條件

```bash
# 任何系統安裝 Docker
curl -fsSL https://get.docker.com | sh

# NixOS 則在 configuration.nix 加入：
# virtualisation.docker.enable = true;
# virtualisation.docker.autoPrune.enable = true;
# then: sudo nixos-rebuild switch
```

---

### Step 1 — 設定精靈（自動生成 .env）

```bash
git clone https://github.com/Zakkaus/VerificationBot.git
cd VerificationBot

# 需要 Go（只跑一次，也可手動複製 .env.example）
go run ./cmd/setup
```

設定精靈會自動偵測你的系統（NixOS / Linux / macOS），逐步詢問並生成 `.env` 文件。

或者手動複製範本：

```bash
cp .env.example .env
nano .env   # 填入以下必填項目
```

---

### Step 2 — 填寫 .env

```env
# ── 必填 ──────────────────────────────────────
TELEGRAM_TOKEN=1234567890:AAG...         # 從 @BotFather 取得
WEBAPP_URL=https://your-domain.com/webapp/

# ── 人機驗證（擇一）──────────────────────────
CAPTCHA_TYPE=turnstile                   # 或 recaptcha
CAPTCHA_SITE_KEY=0x4AAAAAAA...
CAPTCHA_SECRET=0x4AAAAAAA...

# ── 管理後台 ──────────────────────────────────
ADMIN_HOST=0.0.0.0
ADMIN_PORT=8080
JWT_SECRET=至少32字元的隨機字串請用openssl生成
ADMIN_USER=admin
ADMIN_PASS=你的管理員密碼

# ── 資料庫 ────────────────────────────────────
DB_PATH=/data/bot.db

# ── Webhook（比 Polling 延遲更低，需要域名）──
SERVER_HOST=your-domain.com
SERVER_PORT=8443
WEBHOOK_PATH=/webhook

# ── Cloudflare Tunnel（管理後台用，見 Step 4）─
# TUNNEL_TOKEN=eyJhIjoi...
```

生成隨機 JWT_SECRET：
```bash
openssl rand -hex 32
```

---

### Step 3 — 啟動（Polling 模式，快速測試）

```bash
docker compose up -d bot
docker compose logs -f bot
```

期望看到：
```
Authorised as @your_bot
Polling mode
Admin dashboard listening on http://0.0.0.0:8080/admin/
Bot is running
```

管理後台：`http://你的伺服器IP:8080/admin/`

---

### Step 4 — Cloudflare Tunnel（管理後台安全訪問 ✅ 推薦）

不需要開放任何端口，管理後台通過 Cloudflare 的加密隧道訪問。

```bash
# 安裝 cloudflared
# Linux/NixOS:
curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o cloudflared
chmod +x cloudflared && sudo mv cloudflared /usr/local/bin/

# 1. 登入 Cloudflare
cloudflared tunnel login

# 2. 建立 Tunnel
cloudflared tunnel create verificationbot

# 3. 取得 Token（貼到 .env 的 TUNNEL_TOKEN）
cloudflared tunnel token verificationbot

# 4. 在 Cloudflare Dashboard 設定路由：
#    Tunnel → verificationbot → Public Hostnames
#    Hostname: admin.your-domain.com → http://bot:8080

# 5. 啟動含 Tunnel 的完整服務
docker compose up -d
```

管理後台：`https://admin.your-domain.com/admin/` 🔒（直接在瀏覽器打開，無需 SSH）

---

### Step 5 — Webhook 模式（更低延遲）

Webhook 模式需要 HTTPS，用 Nginx + Let's Encrypt 或直接用 Cloudflare Tunnel：

**方案 A：Nginx + Certbot**

```bash
sudo apt install nginx certbot python3-certbot-nginx
sudo certbot --nginx -d your-domain.com

sudo nano /etc/nginx/sites-available/verificationbot
```

```nginx
server {
    listen 443 ssl;
    server_name your-domain.com;
    # SSL 由 Certbot 自動管理

    location /webhook {
        proxy_pass http://127.0.0.1:8443;
        proxy_set_header Host $host;
    }
    location /webapp/ {
        proxy_pass http://127.0.0.1:8080/webapp/;
        proxy_set_header Host $host;
        add_header Cache-Control "public, max-age=3600";
    }
}
```

```bash
sudo ln -s /etc/nginx/sites-available/verificationbot /etc/nginx/sites-enabled/
sudo nginx -s reload

# 啟動
docker compose up -d
```

**方案 B：直接用 Cloudflare Tunnel 代理 Webhook**

```bash
# 在 Cloudflare Tunnel → Public Hostnames 加：
# your-domain.com /webhook → http://bot:8443
# your-domain.com /webapp/ → http://bot:8080

# .env 設定（無需 Nginx）
SERVER_HOST=your-domain.com
SERVER_PORT=443
```

---

### Step 6 — NixOS 完整部署

NixOS 使用 Docker Compose（最簡單的方式）：

```bash
# 1. 啟用 Docker
sudo nano /etc/nixos/configuration.nix
```

```nix
{
  virtualisation.docker.enable = true;
  virtualisation.docker.autoPrune.enable = true;

  # 開放防火牆（Webhook 用，Tunnel 方案不需要）
  networking.firewall.allowedTCPPorts = [ 80 443 ];
}
```

```bash
sudo nixos-rebuild switch

# 2. 部署
mkdir -p /opt/verificationbot
cd /opt/verificationbot
git clone https://github.com/Zakkaus/VerificationBot.git .
go run ./cmd/setup  # 或手動填寫 .env

# 3. 啟動
docker compose up -d
journalctl -u docker -f  # 查看 log

# 或用 Docker 自帶指令
docker compose logs -f
```

---

### 常用維運指令

```bash
# 查看運行狀態
docker compose ps

# 查看即時 log
docker compose logs -f bot

# 重啟 bot
docker compose restart bot

# 更新到最新版
docker compose pull && docker compose up -d

# 備份資料庫
docker compose exec bot cp /data/bot.db /data/bot.bak.db

# 完全停止
docker compose down
```

---

## 🏗️ 多架構 Docker Build

```bash
# 建立並推送 amd64 + arm64 映像
docker buildx create --use
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t ghcr.io/zakkaus/verificationbot:latest \
  --push .
```

---

## ⚙️ 設定精靈

```bash
go run ./cmd/setup
```

自動偵測系統（NixOS / Linux / macOS），逐步詢問所有參數並生成 `.env`，最後給出針對你的系統的部署建議。

---

## 🛡️ Cloudflare Turnstile 設定

1. 前往 [Cloudflare Dashboard → Turnstile](https://dash.cloudflare.com/?to=/:account/turnstile)
2. **Add Site** → Widget type 選 **Managed**（推薦）或 **Invisible**
3. 允許域名填寫你的域名
4. 複製 **Site Key** 和 **Secret Key**

```env
CAPTCHA_TYPE=turnstile
CAPTCHA_SITE_KEY=0x4AAAAAAA...
CAPTCHA_SECRET=0x4AAAAAAA...
```

> 也可在管理後台 → **設定** → **人機驗證設定** 動態切換，無需重啟 Bot。

---

## 管理後台

| 頁面 | 功能 | 最低權限 |
|------|------|----------|
| 儀表板 | 今日統計、最新記錄 | viewer |
| 操作記錄 | 搜尋（用戶名/日期/事件）、匯出 CSV | viewer |
| 設定 | 外觀、人機驗證 Key 動態切換 | admin |
| 帳號管理 | 新增/刪除管理員 | superadmin |

---

## 完整參數列表

| 環境變數 | 說明 | 預設 |
|---|---|---|
| `TELEGRAM_TOKEN` | Bot Token（必填） | — |
| `CAPTCHA_TYPE` | `turnstile` 或 `recaptcha` | `recaptcha` |
| `CAPTCHA_SECRET` | 後端驗證 Secret（必填） | — |
| `CAPTCHA_SITE_KEY` | 前端 Site Key（必填） | — |
| `WEBAPP_URL` | Mini App HTTPS URL（必填） | — |
| `APPROVE_MODE` | Approve 模式 | `true` |
| `BAN` | 超時後封禁 | `false` |
| `BAN_TIME` | 臨時封禁秒數（0=永久） | `0` |
| `SHUTUP` | 入群後禁言（傳統模式） | `true` |
| `TEST_TIME` | 驗證超時秒數 | `120` |
| `GROUPS` | 群組白名單（逗號分隔） | 全部 |
| `SERVER_HOST` | Webhook 域名（空=Polling） | 空 |
| `SERVER_PORT` | Webhook 端口（443/8443） | `0` |
| `ADMIN_HOST` | 管理後台綁定 Host | `0.0.0.0` |
| `ADMIN_PORT` | 管理後台端口 | `8080` |
| `JWT_SECRET` | JWT 密鑰（必填） | — |
| `ADMIN_USER` | 初始管理員帳號 | `admin` |
| `ADMIN_PASS` | 初始管理員密碼（必填） | — |
| `DB_PATH` | SQLite 資料庫路徑 | `bot.db` |
| `TUNNEL_TOKEN` | Cloudflare Tunnel Token | 空 |
| `PROXY` | HTTP/SOCKS5 代理 URL | 空 |