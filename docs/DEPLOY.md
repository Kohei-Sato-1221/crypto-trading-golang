# デプロイ手順（Raspberry Pi）

対象: `feature/order-lifecycle-overhaul` の成果（注文ライフサイクル整備 + Phase 3 レビュー修正 31件）

---

## 0. 事前に把握しておくこと

### 成果物は main に入っていない

```
main                              ← 46コミット遅れ。ここから pull しても何も変わらない
feature/order-lifecycle-overhaul  ← 成果物はすべてこちら（未 push）
```

### main と origin/main の履歴が分岐している

作業中にリベースを行ったため、ローカル `main` と `origin/main` のコミットSHAが食い違っている。

```
ローカル main にあって origin に無い : a2a54db, 4d92eb0
origin/main にあってローカルに無い   : 3968042  ← 4d92eb0 としてリベース済み（内容は同一）
```

**内容はローカル main が origin/main を包含しているため、情報の欠落は無い。**
ただし `git push origin main` は通常のpushでは弾かれ、`--force-with-lease` が必要になる。

### go/config.ini は git 管理対象

Pi 上で `go/config.ini` を編集していると `git pull` が衝突する。編集していなければ
リポジトリの値（`budget_criteria=500000` 等）で**上書きされる**。

**Pi 上で `budget_criteria` を変更している場合は、pull 後に必ず設定し直すこと。**

`go/private_config.ini`（APIキー・DB接続情報）は gitignore 済みなので影響を受けない。

### Go のバージョン要件

`go/go.mod` は **`go 1.25.0`** を要求する。Pi の Go が古い場合:

- Go 1.21 以降なら、ビルド時に必要な toolchain を自動ダウンロードする（ネットワークとディスクが必要）
- Go 1.20 以前なら**ビルドが失敗する**。事前に Go を更新すること

```bash
go version                    # Pi 上で確認
```

なお `make build` は `GOOS` / `GOARCH` を指定しないネイティブビルドなので、
Pi 上でビルドする限りアーキテクチャの問題は起きない。

---

## 1. 【手元のMac】ブランチを push する

デプロイ方法は2つある。**方法Aを推奨**（main の force push を伴わない）。

### 方法A（推奨）: feature ブランチを push して Pi でそれを使う

```bash
cd /Users/sugar/Workspace/crypto-trading-golang
git checkout feature/order-lifecycle-overhaul
git push -u origin feature/order-lifecycle-overhaul
```

新規ブランチなので force は不要。GitHub 上で PR を作ってレビュー・マージするのは後日でよい。

### 方法B: main にマージして push する

```bash
git checkout main
git merge --ff-only feature/order-lifecycle-overhaul   # main は feature の祖先なので fast-forward
git push --force-with-lease origin main                # 履歴分岐のため force が必要
```

**`--force-with-lease` は origin/main の履歴を書き換える。** 他の誰か・他のマシンが
このリポジトリを clone している場合は事前に周知すること。個人利用なら問題ない。

---

## 2. 【Pi】現在の状態を確認する

```bash
ssh <pi>
cd /home/pi/workspace/crypto-trading-golang

# 何が動いているか
systemctl status bfTradingApp.service

# ローカル変更の有無（config.ini を編集していると pull が衝突する）
git status
git branch --show-current

# 現在の設定値を退避しておく（pull で上書きされるため）
cp go/config.ini /tmp/config.ini.bak
grep -E "budget_criteria|is_test|max_buy_orders|max_sell_orders" go/config.ini
```

**`git status` に `go/config.ini` が modified で出た場合**は、pull 前に退避する。

```bash
git stash push go/config.ini      # または cp で退避してから git checkout -- go/config.ini
```

---

## 3. 【Pi】サービスを停止する

バイナリを差し替える前に必ず停止する。稼働中に置き換えるとファイルが破損する。

```bash
sudo systemctl stop bfTradingApp.service
systemctl is-active bfTradingApp.service      # inactive になっていること
```

---

## 4. 【Pi】コードを取得する

### 方法Aの場合

```bash
git fetch origin
git checkout feature/order-lifecycle-overhaul
git pull origin feature/order-lifecycle-overhaul
```

### 方法Bの場合

```bash
git fetch origin
git checkout main
git reset --hard origin/main      # ★履歴が書き換わっているため pull ではなく reset が必要
```

`git log --oneline -3` で最新コミットが手元のものと一致することを確認する。

---

## 5. 【Pi】config.ini を設定する

**ここが最も事故りやすい。**

```bash
# pull 後の値を確認
grep -E "budget_criteria|is_test|max_buy_orders|max_sell_orders|expire_sweep_grace_minutes" go/config.ini
```

### 必ず確認・設定する項目

| キー | 設定値 | 備考 |
|---|---|---|
| `[app] budget_criteria` | **運用したい値** | pull で `500000` に上書きされる。Pi 側で変更していたなら設定し直す |
| `[app] is_test` | `false` | `true` だと 16:24 にテスト用の −5% 深指値ジョブが動く |
| `[bitflyer] max_buy_orders` | `28` | **既定値が効かない唯一のキー。欠落すると 0 になり買い注文が全停止する** |
| `[bitflyer] max_sell_orders` | `60` | **同上** |
| `[bitflyer] expire_sweep_grace_minutes` | `45` | 旧値 10 が残っていると失効検出とローリングが同日競合する |

`trigger_time_01`〜`09`、`buy_minute_to_expire`、`child_orders_count`、
`sell_rollover_*`、`no_order_alert_days`、`balance_diff_threshold_*`、
`untracked_holding_*` は**未設定でも既定値が効く**ので必須ではない。

```bash
# private_config.ini が残っていることを確認（gitignore 済みなので pull の影響は受けない）
ls -l go/private_config.ini
```

---

## 6. 【Pi】ビルドする

```bash
cd /home/pi/workspace/crypto-trading-golang
make build
ls -l go/bfTradingApp        # タイムスタンプが今であること
```

失敗する場合:
- `go: go.mod requires go >= 1.25.0` → Go を更新するか、Go 1.21+ ならネットワーク接続を確認（toolchain 自動DL）
- メモリ不足で OOM → `GOFLAGS=-p=1 make build` で並列度を下げる

---

## 7. 【Pi】systemd ユニットを更新する

今回 `bfTradingApp.service` に **時刻同期待ち**を追加した（Pi に RTC が無く、
起動直後の時刻ずれで買い注文が意図しないタイミングで発火する問題への対処）。

```bash
# リポジトリのユニットファイルを配置（配置先は環境に合わせる）
sudo cp bfTradingApp.service /etc/systemd/system/
sudo cp restart-bftradingapp.service /etc/systemd/system/

# ★これを有効にしないと After=time-sync.target が機能しない（即座に到達扱いになる）
sudo systemctl enable --now systemd-time-wait-sync.service

# ネットワーク待ちも有効か確認
systemctl is-enabled NetworkManager-wait-online.service || \
  systemctl is-enabled systemd-networkd-wait-online.service

sudo systemctl daemon-reload
```

---

## 8. 【Pi】起動して確認する

```bash
sudo systemctl start bfTradingApp.service
systemctl status bfTradingApp.service
tail -f go/trading.log
```

### 起動直後に確認すること

1. **スケジュール登録失敗の Slack 通知が来ないこと**
   来なければ全26本のジョブ登録に成功している。来た場合は通知に失敗したジョブ名が出る
2. `trading.log` に設定値の警告が出ていないこと
   `trigger_time` が不正だと「既定値へフォールバック」の警告が出る
3. 90秒以内に `syncBuyOrders` が動き、DBの `expire_date` が埋まり始めること

```sql
-- 手元から Supabase を参照して確認
SELECT count(*) FILTER (WHERE expire_date IS NOT NULL) AS filled,
       count(*) AS total
  FROM buy_orders WHERE status='UNFILLED' AND order_id <> '';
```

---

## 9. デプロイ後に起きること（想定内・周知事項）

| タイミング | 起きること |
|---|---|
| 起動から90秒以内 | `syncBuyOrders` が既存の未約定注文に `expire_date` を書き始める。それまでは失効検出が方式B（消去法）で動く |
| 当日 22:45 JST | **`cancelBuyOrderJob` が買い注文3件を能動キャンセルする**（7日超のもの）<br>`JRF20260810-223720-006155` / `JRF20260810-223736-004358` / `JRF20260824-041919-077747`<br>いずれも現在値から −20〜−25% の深指値で約定見込みは薄い |
| 翌日 05:30 JST | `rolloverSellOrderJob` 初回実行（対象0件のはず） |
| 翌日 06:05 JST | `expireSweepJob` 初回実行 |
| 翌日 06:15 JST | `reconcileJob` 初回実行。日次サマリが Slack に届く |
| 翌日 06:30 JST | 買い注文ジョブ初回実行 |

---

## 10. 切り戻し手順

問題が起きた場合。

```bash
sudo systemctl stop bfTradingApp.service
cd /home/pi/workspace/crypto-trading-golang

git checkout <デプロイ前のコミットSHA>       # 事前に控えておくこと
cp /tmp/config.ini.bak go/config.ini         # 手順2で退避したもの
make build
sudo systemctl start bfTradingApp.service
```

### DB は切り戻さなくてよい

`expire_date` カラムは **NULL 許容の追加のみ**なので、旧バイナリでも問題なく動く
（旧コードはこのカラムを参照しない）。`strategy` の `smallint → integer` も同様。

**DB のバックアップテーブルは残してある**（`buy_orders_bk20260906` / `sell_orders_bk20260906`）。
不要になったら `DROP TABLE` してよい。

---

## 11. デプロイとは独立の未対応事項

| # | 内容 |
|---|---|
| F1 | `scripts/migration/import_to_supabase.sh` / `reset_supabase.sh` に**本番DBのパスワードが平文でコミットされている**。パスワードのローテーションを推奨 |
| F29 | `GetBalance` / `GetTicker` がレスポンス全文をログ出力するため、`trading.log` に残高が残る。`clear-trading-log.sh` でのローテーション運用を継続 |
| F27 | 売り注文の部分約定時に損益が過小計上される。発生時の手動対応手順はルート `CLAUDE.md`「売り注文の部分約定が起きたときの対応」に記載 |
