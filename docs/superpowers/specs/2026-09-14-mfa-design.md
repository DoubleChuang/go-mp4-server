# 設計：Session 登入 + 可選 TOTP 兩步驟驗證（MFA）

日期：2026-09-14
狀態：已獲使用者核准（方案 A、自助綁定、可選啟用、加單元測試、不加備用碼）

## 背景與動機

目前認證是 Fiber `basicauth` middleware（`pkg/videoserver/videoserver.go:93`），瀏覽器原生對話框、無狀態，無法支援多步驟驗證。目標：登入改為自製登入頁 + Session cookie，並提供**可選的** TOTP（驗證器 App）第二步驟驗證，使用者可自助綁定。

## 架構變更

- 移除 `configBaseAuth()` 的 basicauth middleware。
- 新增 Fiber session middleware（in-memory，內建於 fiber，無新的大型依賴）。
- 新增自訂 auth middleware：
  - session 有 `authenticated` 標記 → 放行。
  - 無 → 重導向 `/login`。
  - 排除路徑：`/login`、`/verify`、`/static`（登入頁需要 CSS）。
- `/videos`（含 ByteRange 靜態服務）與所有頁面自動受保護；video tag 在頁面載入後才請求，session 已存在，無需額外處理。
- Session 時效 24 小時。in-memory：重啟後所有使用者需重新登入（可接受，操作說明中記錄）。

## 登入流程（兩步驟）

| 步驟 | 路由 | 行為 |
|---|---|---|
| 1 | `GET /login` | 帳號密碼表單 |
| 1 | `POST /login` | **每次登入時**讀取 auth.json 驗證（而非啟動時，允許改密碼免重啟）；未開 2FA → 建立 session 跳轉 `/`；有開 → session 標記 `awaiting_otp`，跳轉 `/verify` |
| 2 | `GET/POST /verify` | 輸入 6 位 TOTP 驗證碼，`totp.Validate` 通過後完成登入 |
| — | `GET /logout` | 摧毀 session |

錯誤訊息統一為「Invalid credentials / Invalid code」，不洩漏使用者是否存在。

## TOTP 綁定（`/settings` 設定頁）

- `POST /settings/2fa/enable` → 產生密鑰（暫存於 session）→ 頁面顯示 QR code（後端 `skip2/go-qrcode` 產生 PNG，不依賴 CDN，適合離線 LAN）＋密鑰文字。
- `POST /settings/2fa/confirm` → 輸入驗證碼確認 → 寫入 `2fa.json`（避免「綁定但沒確認」的死狀態）。
- `POST /settings/2fa/disable` → 需輸入目前 TOTP 碼才能關閉。
- 遺失驗證器的補救：手動編輯/刪除 `2fa.json` 後重綁（操作說明記錄）。不加備用碼。

## 資料儲存

- `auth.json` 不變（帳號密碼維持原樣）。
- 新增 `2fa.json`（加入 `.gitignore`）：
  ```json
  {"users": [{"name": "admin", "totp_secret": "JBSWY3DPEHPK3PXP", "enabled": true}]}
  ```
- 檔案讀寫以 mutex 保護並發存取；每次操作（登入/綁定/關閉）時重新讀取；`2fa.json` 不存在視為「無人開啟 2FA」。寫入僅發生於 confirm（新增/更新）與 disable（移除/標記停用）。

## 新增依賴與設定

- Go 依賴：`github.com/pquerna/otp`（TOTP）、`github.com/skip2/go-qrcode`（QR PNG）。
- 新 viper 預設值（env 慣例 `GOMP4_` 前綴、`.`→`_`）：
  - `SERVER.TOTP.CONFIG.PATH` → `GOMP4_SERVER_TOTP_CONFIG_PATH`，預設 `./2fa.json`。
  - `SERVER.TOTP.ISSUER` → `GOMP4_SERVER_TOTP_ISSUER`，預設 `go-mp4-server`。

## 檔案變更清單

- 修改：`main.go`（傳 TOTP 設定）、`pkg/config/config.go`（新預設值）、`pkg/videoserver/videoserver.go`（移除 basicauth、掛載新 middleware 與路由）、`.gitignore`（加 `2fa.json`）。
- 新增：`pkg/videoserver/auth.go`（session middleware + login/verify/logout handlers）、`pkg/videoserver/totp.go`（2fa.json 存取）、`pkg/videoserver/settings.go`（enable/confirm/disable handlers）、`views/login.django`、`views/verify.django`、`views/settings.django`。

## 測試

- 專案目前無測試；本次新增 2 個單元測試（新慣例的開始）：
  - TOTP 驗證邏輯（`totp.go`）。
  - 登入流程 + auth middleware（fiber `app.Test()` + `t.TempDir()` 產生暫時 2fa.json）。
- 驗證：`go build ./...`、`go vet ./...`、`go test ./...` 全數通過。
- 手動流程：啟動 server → 登入 → 綁定 TOTP（驗證器 App）→ 登出 → 兩步驟登入 → 關閉 2FA。