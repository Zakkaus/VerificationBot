# VerificationBot

Telegram 群組驗證 Bot — **Go** 語言，支援 **Google reCAPTCHA v2** 與 **Cloudflare Turnstile** 🛡️

---

## 功能

| 功能 | 說明 |
|------|------|
| Approve 模式 | 用戶申請入群 → Bot 私訊 → Mini App 驗證 → 批准 |
| 傳統模式 | 入群後禁言 → 完成驗證 → 恢復權限 |
| 兩種人機驗證 | Google reCAPTCHA v2 **或** Cloudflare Turnstile |
| 管理後台 | 操作記錄、外觀設定、角色帳號管理 (RBAC) |
| 延遲最低 | Webhook 模式，延遲 < 100ms |
| 單一二進位 | Go 靜態編譯，CGO_ENABLED=0，含嵌入前端 |

---

## 快速開始

```bash
# 1. 建置
go build -o bot .

# 2. 複製設定
cp .env.example .env   # 填入 token / captcha key

# 3. 啟動（Polling 測試）
./bot --telegram-token TOKEN \
      --captcha-type turnstile \           # 或 recaptcha
      --captcha-secret CF_SECRET \
      --captcha-site-key CF_SITE_KEY \
      --webapp-url https://your-domain.com/webapp/ \
      --jwt-secret "隨機長字串" \
      --admin-pass "管理員密碼"
```

管理後台：`https://your-domain.com/admin/`

---

## Cloudflare Turnstile 設定

1. 進入 [Cloudflare Zero Trust → Turnstile](https://dash.cloudflare.com/?to=/:account/turnstile)
2. 新增站點 → 選 **Invisible** 或 **Managed**
3. 拿到 **Site Key** 和 **Secret Key**
4. 在 `.env` 設定：

```env
CAPTCHA_TYPE=turnstile
CAPTCHA_SITE_KEY=0x4AAAAAAA...
CAPTCHA_SECRET=0x4AAAAAAA...
```

> Google reCAPTCHA 仍然支援，只需把 `CAPTCHA_TYPE=recaptcha` 即可。

---

## NixOS 部署（完整指南）

### Step 1 — 交叉編譯 Linux 靜態二進位

```bash
# 在 Mac / 任何機器上
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -ldflags="-s -w" -o bot .

scp bot root@your-server:/opt/verificationbot/
```

### Step 2 — 建立 secrets 檔案

```bash
# 在 NixOS 伺服器上
mkdir -p /run/secrets
cat > /run/secrets/verificationbot.env << 'EOF'
TELEGRAM_TOKEN=8499180997:AAG...
CAPTCHA_TYPE=turnstile
CAPTCHA_SITE_KEY=0x4AAAAAAA...
CAPTCHA_SECRET=0x4AAAAAAA...
WEBAPP_URL=https://your-domain.com/webapp/
APPROVE_MODE=true
TEST_TIME=120
SERVER_HOST=your-domain.com
SERVER_PORT=8443
WEBHOOK_PATH=/webhook
ADMIN_HOST=127.0.0.1
ADMIN_PORT=8080
JWT_SECRET=你的隨機長字串至少32字元
ADMIN_USER=admin
ADMIN_PASS=你的管理員密碼
DB_PATH=/var/lib/verificationbot/bot.db
EOF
chmod 600 /run/secrets/verificationbot.env
```

### Step 3 — configuration.nix

在你的 `/etc/nixos/configuration.nix` 加入：

```nix
{ config, pkgs, ... }:
{
  # ── 用戶與服務 ───────────────────────────────────────
  users.users.verificationbot = {
    isSystemUser = true;
    group        = "verificationbot";
    home         = "/var/lib/verificationbot";
    createHome   = true;
  };
  users.groups.verificationbot = {};

  systemd.services.verificationbot = {
    description = "VerificationBot";
    after       = [ "network-online.target" ];
    wants       = [ "network-online.target" ];
    wantedBy    = [ "multi-user.target" ];
    serviceConfig = {
      Type             = "simple";
      User             = "verificationbot";
      Group            = "verificationbot";
      WorkingDirectory = "/var/lib/verificationbot";
      EnvironmentFile  = "/run/secrets/verificationbot.env";
      ExecStart        = "/opt/verificationbot/bot";
      Restart          = "on-failure";
      RestartSec       = "5s";
      NoNewPrivileges  = true;
      PrivateTmp       = true;
      ProtectSystem    = "strict";
      ReadWritePaths   = [ "/var/lib/verificationbot" ];
    };
  };

  # ── Nginx + Let's Encrypt ────────────────────────────
  security.acme = {
    acceptTerms = true;
    defaults.email = "your@email.com";
  };

  services.nginx = {
    enable = true;

    virtualHosts."your-domain.com" = {
      enableACME = true;
      forceSSL   = true;

      # Telegram Webhook（Bot 內部跑在 8443）
      locations."/webhook" = {
        proxyPass = "http://127.0.0.1:8443";
        extraConfig = ''
          proxy_http_version 1.1;
          proxy_set_header Host $host;
          proxy_read_timeout 30s;
        '';
      };

      # Mini App（必須 HTTPS）
      locations."/webapp/" = {
        proxyPass = "http://127.0.0.1:8080/webapp/";
        extraConfig = ''
          proxy_http_version 1.1;
          proxy_set_header Host $host;
          add_header Cache-Control "public, max-age=3600";
        '';
      };

      # 管理後台只開放本地（用 SSH Tunnel 存取）
      # 如需公開，取消以下注解並設定 IP：
      # locations."/admin/" = {
      #   proxyPass = "http://127.0.0.1:8080/admin/";
      #   extraConfig = ''
      #     allow YOUR.IP.HERE;
      #     deny all;
      #   '';
      # };
    };
  };

  # ── 防火牆 ───────────────────────────────────────────
  networking.firewall.allowedTCPPorts = [ 80 443 ];
}
```

### Step 4 — 套用設定

```bash
sudo nixos-rebuild switch
```

### Step 5 — 確認運行

```bash
# 查看 log
sudo journalctl -u verificationbot -f

# 預期輸出
# Authorised as @your_bot
# Webhook mode: https://your-domain.com/webhook
# Admin dashboard listening on http://127.0.0.1:8080/admin/
# Bot is running
```

### Step 6 — 存取管理後台（SSH Tunnel）

```bash
# 在你的本地機器上
ssh -L 8080:localhost:8080 root@your-domain.com

# 然後在瀏覽器打開
open http://localhost:8080/admin/
```

---

## 完整參數列表

| 環境變數 / CLI 旗標 | 說明 | 預設 |
|---|---|---|
| `TELEGRAM_TOKEN` | Bot Token（必填） | — |
| `CAPTCHA_TYPE` | `recaptcha` 或 `turnstile` | `recaptcha` |
| `CAPTCHA_SECRET` | 伺服端驗證 Secret（必填） | — |
| `CAPTCHA_SITE_KEY` | 前端 Site Key（必填） | — |
| `WEBAPP_URL` | Mini App HTTPS URL（必填） | — |
| `APPROVE_MODE` | Approve 模式（true/false） | `true` |
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
| `PROXY` | HTTP/SOCKS5 代理 URL | 空 |