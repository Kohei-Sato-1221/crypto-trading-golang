# 実行履歴: order-lifecycle-overhaul

対象: Bitflyer 自動売買ボットの注文ライフサイクル整備（全7スプリント）
ブランチ: `feature/order-lifecycle-overhaul`

| Sprint | テーマ | 試行 | 判定 | 日付 |
|---|---|---|---|---|
| 1 | Bitflyer API クライアント層の堅牢化 | 1回目 | **pass** | 2026-09-06 |
| 5 | 買い指値・期限・スロット設計の見直し | 1回目 | **pass** | 2026-09-06 |
| 2 | スキーマ変更と記録の正常化 | 1回目 | **pass** | 2026-09-06 |
| 3 | 失効検出（expireSweepJob） | 1回目 | **pass** | 2026-09-06 |
| 4 | 売り注文の27日ローリング | 1回目 pass → 指摘修正 → 回帰 **pass** | **pass** | 2026-09-07 |
| 6 | 利確幅の戦略別可変化 | 1回目 | **pass** | 2026-09-07 |
| 7 | 日次リコンサイル＆アラート | 1回目 pass → 指摘修正 → 回帰 **pass** | **pass** | 2026-09-07 |

**全7スプリント完了（2026-09-07）。**

> **実行順序について**: スプリント2が本番PostgreSQLへの DDL 適用待ちでブロックされたため、ユーザー判断により DB 非依存のスプリント5を先行実施した（1 → 5 → 2 → 3 → 4 → 6 → 7 の順）。

---

## Sprint 1 — Bitflyer API クライアント層の堅牢化

**判定: pass（1回目）**

Sprint 1 完了: Bitflyer クライアント層を堅牢化（CancelOrder のレスポンス検証／doRequest 切り出し／count・before ページング／個別照会／GetActiveBuyOrders→GetChildOrders 改名／child_orders_count 設定追加）。make build OK、go vet・gofmt は変更ファイルに新規指摘なし、make test 16件PASS。参照系 API でページングと個別照会の実動作を確認。CancelOrder の実機確認は更新系のため pending_user_approval に記載。

### 評価スコア

| 基準 | スコア | 閾値 |
|---|---|---|
| 機能完成度 | 9 | 7 |
| バグ | 9 | 7 |
| 回帰 | 9 | 8 |
| 取引安全性 | 9 | 8 |

ゲート（make build / go vet / gofmt）はすべて pass。`go vet` のベースラインは HEAD を `git worktree add --detach` で切り出して実測比較し、**新規指摘ゼロ**を確認。

### 変更ファイル

- `go/bitflyer/bitflyer.go`
- `go/config/config.go`
- `go/config.ini`
- `go/app/bitflyerApp/syncBuyOrders.go`
- `go/app/bitflyerApp/filledCheckJob.go`（改名追従のみ）

### 持ち越し事項（issues・すべて Low）

1. `service.go:219-221` — `CancelOrder()` の error を捨てている。既存挙動の据え置きだが **Sprint 3 の `cancelBuyOrderJob` 改修時に対応**すること
2. `bitflyer.go:251-253` — `GetChildOrdersAll()` はページング途中エラーで取得済みページを全破棄。**Sprint 3 の方式Bで「取得失敗時は sweep をスキップ」する実装が必要**
3. `syncBuyOrders` / `filledCheckJob` の取得件数が暗黙に 100→500 に増加。`models.SyncBuyOrders` が冪等なため安全

### 未確認事項

`CancelOrder()` のレスポンス検証は実機未確認（更新系API）。

**【ユーザー決定 2026-09-06】Sprint 4 のローリング E2E 検証とまとめて実施する（選択肢2を採用）。**
Sprint 1 単体での実発注は行わない。Sprint 4 着手時に、以下の内容で改めてユーザーの承認を得ること:

- `POST /v1/me/sendchildorder` → `POST /v1/me/cancelchildorder`
- ETH_JPY / SELL / LIMIT / 指値 = 現在LTP × 1.5 / size 0.01 / `minute_to_expire=10`
- 想定影響: 現物 0.01 ETH の一時拘束のみ。約定可能性は極めて低く、10分で自動失効
- 検証内容: キャンセル成功の判定 → 個別照会での消滅確認 → 再発注、の一連の流れ


---

## Sprint 5 — 買い指値・期限・スロット設計の見直し

**判定: pass（1回目）**

### 評価スコア

| 基準 | スコア | 閾値 |
|---|---|---|
| 機能完成度 | 9 | 7 |
| バグ | 9 | 7 |
| 回帰 | 9 | 8 |
| 取引安全性 | 9 | 8 |

`make build` OK / `make test` 16件PASS / `go vet`・`gofmt` はベースラインと指摘集合が完全一致（新規指摘ゼロ、`git worktree add --detach HEAD` で実測比較）。

### 実装内容

| 項目 | 変更 |
|---|---|
| 月曜 | `StrategyLTP97 = 10004`（`ltp*0.97`）。旧 `StrategyLTP95`(10003) から差し替え |
| 日曜 | `StrategyLtpLowestIn7days7t3 = 20003`（`ltp*0.7 + low7*0.3`）。旧 `2t8`(20002) から差し替え |
| 旧定数 10003 / 20002 | **残置**（`CalculateBuyPrice()` の case ごと。過去データの集計を壊さないため） |
| 買い注文期限 | 既定 10080分（7日）。`buy_minute_to_expire` で設定可 |
| `max_buy_orders` | 15 → **28**（超過は発注ブロック） |
| `max_sell_orders` | 25 → **60**（**超過しても Slack 警告のみ・発注続行**） |
| スロット判定 | `BuyOrderSlotStatus` / `GetBuyOrderSlotStatus()` を新設。`ShouldPlaceBuyOrder()` は後方互換ラッパとして維持 |
| `budget_criteria` | **変更なし**（ユーザー環境側で管理） |

### 変更ファイル

- `go/enums/strategy.go` / `go/utils/calcUtils.go` / `go/models/events.go`
- `go/app/bitflyerApp/service.go` / `go/app/bitflyerApp/placeBuyOrder.go`
- `go/config.ini` / `go/config/config.go`
- `go/cmds/send_results_job/main.go` / `go/cmds/save_price_history_job/main.go`（`NewBitflyer` 引数順修正）

### 発見・修正した既存バグ

**`NewBitflyer()` の引数順が3箇所すべてで逆だった。**
定義は `NewBitflyer(key, secret, max_buy_orders, max_sell_orders)` だが、呼び出し元3箇所（`service.go:94` / `cmds/send_results_job/main.go:24` / `cmds/save_price_history_job/main.go:24`）が `BFMaxSell, BFMaxBuy` の順で渡していた。

その結果、**本番では実効的に「buy上限=25 / sell上限=15」で動作しており、未約定 sell が15本たまった時点で買い注文が全面停止していた**（今回の障害の直接原因の一部）。修正しないとスプリント5の 28/60 が逆適用されるため、必須のバグ修正としてスコープ内と判断。

### 仕様変更（実装後）

**価格履歴の毎時記録は、ユーザー判断により取り消し。** 1日2回（`trigger_time_03=18:00` / `trigger_time_04=06:00`）のまま据え置く。一度実装したものを完全に巻き戻し済み（`savePriceHistoryJob.go` の差分ゼロ、関連キー・定数・ヘルパーの残骸ゼロを grep で確認）。`spec.md` / `design-dock.md` も修正済み。

トレードオフ: `GetLowestPriceInPast7Days()` のサンプルが14点のままなので「7日安値」は実際の安値より高めに出る。ただし日曜戦略を 2t8→7t3 に変更して7日安値への依存度を 80%→30% に下げたため影響は限定的。水曜 5t5（50%依存）は据え置き。

### 持ち越し事項（issues）

| 深刻度 | 内容 | 対応先 |
|---|---|---|
| Medium | `expire_date` の DB 記録なし / `CheckFilledBuyOrders()` の `Strategy` 代入漏れ | Sprint 2 |
| Medium | `cancelBuyOrderJob` の `BfCancelCriteria = -3` と期限7日化の不整合（3日で能動キャンセルされ期限延長の効果が出ない） | Sprint 3 |
| Low | sell 超過の Slack 警告が最大4通/日で常態化しうる | Sprint 7 の通知設計で吸収 |
| Low | `ShouldPlaceBuyOrder()` が実質テスト専用に（design-dock の確定仕様どおり） | — |

### デプロイ時の注意

本番 EC2 の `config.ini` に `max_buy_orders=28` / `max_sell_orders=60` / `buy_minute_to_expire=10080` の反映が必要（前2つは `MustInt()` のため本番ファイルの値がそのまま効く）。**ただしデプロイはスプリント2〜4完了後に行うこと**（失効検出・ローリングが無い状態で `max_sell_orders` の警告化だけ入れると幽霊レコードが再び溜まる余地が残る）。


---

## Sprint 2 — スキーマ変更と記録の正常化

**判定: pass（1回目）**

### 評価スコア

| 基準 | スコア | 閾値 |
|---|---|---|
| 機能完成度 | 9 | 7 |
| バグ | 9 | 7 |
| 回帰 | 10 | 8 |
| 取引安全性 | 9 | 8 |

`make build` OK / `make test` 16件PASS（`docker compose down -v` でボリューム再作成後）/ `go vet`・`gofmt` は main との実測比較で新規指摘ゼロ。

### 前提作業（メイン会話が実施・Generator の範囲外）

**本番 Supabase(PostgreSQL) を Atlas のマイグレーション管理下に置き、DDL を適用した。**

調査の結果、PostgreSQL 側にはマイグレーション機構が存在しなかった（`pg_class` / `pg_namespace` 直参照で確認。`public` にリビジョン管理テーブルなし。`auth.schema_migrations` 等はすべて Supabase 内部のもの）。MySQL(RDS) 向けの Atlas 系譜は存在したが、2026-05 の Supabase 移行時に引き継がれていなかった。

新設したもの:
- `db/crypto-trading-db-postgres/migrations/20260506000000_baseline_postgres.sql`（ベースライン。`--baseline` 指定で記録のみ・実行されない。本番の `pg_class` / `pg_trigger` を直参照して実スキーマと一致することを確認済み）
- `db/crypto-trading-db-postgres/migrations/20260906000000_add_expire_date.sql`
- `atlas.sum`（`atlas migrate hash` で生成）
- `db/Makefile` に `pg-remote-status` / `pg-remote-dry-run` / `pg-remote-apply`（`atlas` 行は `@` 付きでパスワードを echo しない）
- `db/envs/.pg.env`（gitignore 済み）

適用結果: `Current Version: 20260906000000` / `Pending Files: 0`。既存データ無傷（buy 710行 / sell 504行 / 未約定 buy 7 / 未約定 sell 0 / `[MANUAL_HOLD]` 65件）。

> **注意**: Atlas の `--dry-run` は `--baseline` と併用すると、シミュレーション前にベースラインの適用記録を**実際に書き込む**。完全な非破壊ではない（アプリのテーブルは変更されない）。
> **注意**: 接続は必ず **5432（session pooler）** を使う。6543 は transaction pooling のため Atlas のアドバイザリロックが壊れる。

### 実装内容

| 変更 | 内容 |
|---|---|
| `models/events.go` | `OrderEvent` に `ExpireDate *time.Time` / `Remarks` 追加。`SellOrderWithMeta()` 新設、`SellOrder(pid)` は委譲ラッパとして維持 |
| **`CheckFilledBuyOrders()`** | **`Strategy` 代入漏れバグを修正**（`events.go:318`）。`BuyOrderInfo.Strategy` を `float64` → `int` に |
| `models/orderLifecycle.go`（新規） | `OrderTable` 限定型でテーブル名連結を安全化 + `UpdateOrderExpireDate()` |
| `utils/util.go` | `ParseBitflyerTime()` を新設。スプリント1で bitflyer 内にあった重複パーサを一本化 |
| `enums/strategy.go` | `StrategyUnknown=99` / `StrategySaturatedUnknown=127` / `StrategyManual=90001` |
| `placeBuyOrder` / `placeSellOrder` | `expire_date`(UTC) を計算して保存。`sell_minute_to_expire=43200` を設定化（従来のハードコードと同値） |
| `syncBuyOrders` | API の `expire_date` をパースして保存・上書き。パース失敗時は DB を更新しない |
| `init/001_init.sql` / `schema.hcl` | `strategy` を `INTEGER`/`int` 化、`expire_date` 2本、インデックス2本（本番と一致） |

**過去データ（`strategy=127` / `99`）の UPDATE は一切行っていない**（差分に `UPDATE/SET strategy` は0件、本番実測でも残存を確認）。

### Evaluator が実測で裏取りした点

- **Bitflyer の時刻は UTC** であることを ticker の実時刻比較で決定的に確認（API `11:47:39` = UTC now、JST 20:47 ではない）。実 `expire_date` 8件すべてパース成功、小数秒 `.42` が実在
- `expire_date` は JST ロケーションの `time.Time` を渡しても UTC 実時刻で保存されることをローカルDBで実証（二重防御が機能）
- `OrderTable("buy_orders; DROP TABLE buy_orders")` が拒否されることを実証
- **インライン `#` コメントが go-ini で正しく剥がされる**ことを実測（ここが壊れると発注が全停止するため）
- 本番 `information_schema` / `pg_indexes` と `init/001_init.sql` / `schema.hcl` の突合

### 持ち越し事項（issues・すべて Low）

| 内容 | 対応先 |
|---|---|
| `BuyOrder()` の INSERT 失敗時、90秒後の `syncBuyOrders` がボット注文を `strategy=90001`（手動）として取り込み戦略値を失う（稀） | Sprint 7 |
| `bitflyer.Ticker.DateTime()` が RFC3339 でパースし常に失敗（**main にも存在する既存バグ**、参照0件で実害なし） | 任意 |
| `init/001_init.sql` の `expire_date` 物理カラム順が本番と異なる（列名指定SQLのみのため実害なし） | 任意 |
| `events.go` の WriteString 警告（**main にも存在する既存コード**、gocritic 系ヒントで実害なし） | 任意 |

### 評価後の修正

`enums/strategy.go` のコメントに書かれたレコード件数が誤っていた（メイン会話が渡した数値の誤り）。本番実測で **`127` = 472件 / `99` = 208件** に訂正。コメントのみの変更で挙動に影響なし。


---

## Sprint 3 — 失効検出（expireSweepJob）

**判定: pass（1回目）**

### 評価スコア

| 基準 | スコア | 閾値 |
|---|---|---|
| 機能完成度 | 9 | 7 |
| バグ | 8 | 7 |
| 回帰 | 9 | 8 |
| 取引安全性 | 9 | 8 |

`make build` OK / `make test` 16件PASS / **`go vet` の指摘が5件 → 1件に**（`filledCheckJob.go` の4件を解消、新規0件。`git worktree` でベースラインを実測比較）/ `gofmt` はベースラインと同一。

### 実装内容

**新規ファイル**

| ファイル | 内容 |
|---|---|
| `go/app/bitflyerApp/expireSweepJob.go` | 方式A（`expire_date` 経過→`CANCELLED`。**COMPLETED 一覧との突合を先に行い、約定済みなら `FILLED`**）＋ 方式B（`expire_date` NULL の旧レコードを ACTIVE/COMPLETED 一覧との突合で判定。**遡り限界より古いものは判定保留＋Slack通知**）。一覧取得に失敗した通貨ペアは sweep 自体をスキップ |
| `go/app/bitflyerApp/cancelBuyOrderJob.go` | `service.go` のインラインクロージャから分離 |

**変更**: `filledCheckJob.go`（500件明示指定・`go vet` 指摘4件解消・`break`→`continue`・エラー時 Slack 通知・goto 廃止）、`service.go`（`trigger_time_06`=06:05 で sweep 登録、cancel を 22:45 へ）、`models/orderLifecycle.go`（`OrderRecord` 型と4関数、`RemarkRolloverPending` 定数）、`config.go` / `config.ini`（`buy_order_cancel_days`=7、`expire_sweep_grace_minutes`=10、`trigger_time_06`）

### 前スプリントからの持ち越し3件を解消

1. `cancelBuyOrderJob` が `CancelOrder()` の error を捨てていた → **失敗時は DB を更新せず Slack 通知**へ
2. `GetChildOrdersAll()` のページ全破棄仕様への対処 → **一覧取得に失敗した通貨ペアは sweep をスキップ**
3. `cancelBuyOrderJob` を 23:45 → **22:45 へ移動**（当時は「EC2 稼働窓外で実質発火しない」という前提だったが、**後に実行環境が Raspberry Pi の24時間稼働と判明し、この前提は誤りだった**。23:45 でも発火していた。22:45 でも Pi 停止時間帯 01:30〜02:45 JST を避けており支障はないため据え置き）

### 仕様変更（ユーザー判断）

Generator は当初 `cancelBuyOrderJob` に「ボット発注と判別できない注文（`strategy` が 99/127/90001）はキャンセルしない」ガードを `enums.IsBotStrategy()` で実装したが、**spec.md にない追加のためユーザー判断で削除**した。

> ユーザーの言葉:「Bガードを外すでよいです。手動でも、自動でも同じ扱いでよいかと」

**確定仕様: `buy_order_cancel_days`（7日）を超えた未約定の買い注文は、ボット発注か手動発注かを問わず一律にキャンセルする。**

`enums.IsBotStrategy()` は参照ゼロになったため削除。design-dock §4.3 に記載はあるが、実際に必要となる Sprint 7（`reconcileJob`）で追加する方針。

### Evaluator が本番実データで検証した点

本番DBへ **SELECT のみ**のドライランを自作して実行:
- 方式A候補 **0件**
- 方式B候補 **7件**（BTC 4 / ETH 3）は**すべて「SKIP (exchange ACTIVE)」** → CANCELLED になる行は **0件**
- 参照系APIの ACTIVE 一覧（BTC 4 / ETH 3）と DB の order_id が完全一致
- `[MANUAL_HOLD]` 130件（buy 65 / sell 65）は `status='UNFILLED'` 条件で構造的に除外
- `OrderTable` の SELECT 列11個 ⇔ Scan 先11個の一致を本番へ実クエリを流して確認

### デプロイ後にキャンセルされる注文（確定仕様どおりの意図した挙動）

`buy_order_cancel_days=7` / 判定境界(UTC)=`2026-08-30T13:07:04Z`

| order_id | 通貨ペア | 価格 | 数量 | 発注(UTC) | 経過 |
|---|---|---|---|---|---|
| JRF202…6155 | BTC_JPY | 9,887,890 | 0.002 | 2026-08-11 07:37 | 26.2日 |
| JRF202…4358 | ETH_JPY | 289,551 | 0.03 | 2026-08-11 07:37 | 26.2日 |
| JRF202…7747 | BTC_JPY | 11,906,454 | 0.003 | 2026-08-24 13:19 | 13.0日 |

残り4件は7日を超えた時点で順次対象になる。**残したい注文があればデプロイ前に取引所側で対応すること。**

### 持ち越し事項（issues・すべて Low）

| 内容 | 対応先 |
|---|---|
| 方式Aには方式Bのような「COMPLETED 遡り限界より古いものは判定保留」ガードが無い。現状 `oldestCompleted` が BTC=2022-06 / ETH=2023-10 と十分古く実害なしだが、取引量が増えると約定済み注文を誤 CANCELLED しうる | Phase 3 レビューで検討 |
| `cancelBuyOrderJob.go:67-69` — `Timestamp` がゼロ値（パース失敗）だと「古い」と判定されキャンセルされる。方式Bは同じケースを安全側に倒しており非対称 | Phase 3 レビューで検討 |
| `service.go:250` の `"22:45"` がハードコード（他ジョブは config 化） | Phase 3 レビューで検討 |
| `GetUnfilledBuyOrderRecords` が exchange/product_code でフィルタしておらず、OKEX 由来の UNFILLED があると Bitflyer の CancelOrder を誤った product_code で叩く（旧実装も同様で新規劣化ではない） | Phase 3 レビューで検討 |

### 実機未確認

`cancelBuyOrderJob` の `CancelOrder()` 成功パスは実機未確認（更新系APIのため）。コード追跡＋本番データのドライランで代替。デプロイ後の初回 22:45 JST に3件がキャンセルされ Slack 通知で確認可能。


---

## Sprint 4 — 売り注文の27日ローリング（rolloverSellOrderJob）

**判定: pass（1回目）**

### 評価スコア

| 基準 | スコア | 閾値 |
|---|---|---|
| 機能完成度 | 9 | 7 |
| バグ | 8 | 7 |
| 回帰 | 10 | 8 |
| 取引安全性 | **8** | 8（ぎりぎり） |

`make build` OK / `make test` **20 passed / 0 failed**（16→20。追加4件はすべて実DB入出力の検証で CLAUDE.md のテスト方針に適合と評価）/ `go vet`・`gofmt` はベースラインのみ。

### 実装内容

**新規 `go/app/bitflyerApp/rolloverSellOrderJob.go`**

処理順序（安全性の要）:
1. COMPLETED 一覧と突合して約定確認
2. `CancelOrder()` の**成功を確認**
3. **個別照会で消滅/CANCELED を再確認** — `COMPLETED` なら再発注せず `FILLED` 更新（二重売り防止）、`ACTIVE` ならキャンセル未成立として再発注しない
4. 同一 `product_code` / `price` / `size` で再発注
5. **単一トランザクション**で DB 更新（旧 → `CANCELLED`、新 → INSERT）

Generator が自主的に追加した防御:
- 最小取引単位（BTC 0.001 / ETH 0.01）未満は**キャンセル前に**弾く（裸の保有を作らない）
- `minute_to_expire` を上限 43200 分で丸める

**`models/orderLifecycle.go` 追加**: `GetSellOrdersToRollover()` / `RolloverSellOrder()` / `AppendOrderRemark()`

**設定追加**: `trigger_time_05`=05:30 / `sell_rollover_days_before_expire`=3 / `sell_rollover_fallback_days`=27 / `sell_rollover_max_per_run`=20

スケジュール順序: **rollover(05:30) → sweep(06:05) → 買い注文(06:30)**（すべて Pi 停止時間帯 01:30〜02:45 JST を回避）

### Evaluator がローカルDBで実測した点

- `remarks` が `"OLD-SELL-1の売り注文の再注文"` と**完全一致**（等値比較で検証。ユーザーの明示要望）
- `parentid` 引き継ぎ後に `getResultsPostgres()` を通し **profit=74.92 / count=1**（旧 CANCELLED レコードの二重計上なし）
- `[MANUAL_HOLD]` 付き CANCELLED 65件を投入 → rollover / sweep ともに **0件**
- UPDATE失敗 / 0行 / INSERT失敗の**3経路すべてで Rollback** されること
- 実在しないIDの個別照会が `("", nil)` を返し「消滅=キャンセル成立」分岐に入ること（実データ）

### ★持ち越し issues

| 深刻度 | 内容 |
|---|---|
| **Medium** | **部分約定した売り注文を full size で再発注する。** `lookupChildOrderState()` が `ExecutedSize` / `OutstandingSize` を破棄しているため部分約定を検出できない。部分約定注文をキャンセルすると state は `CANCELED` になるので「消滅=キャンセル成立」分岐に落ち、そのまま元の size で再発注される。通常は取引所の残高チェックで拒否されるが、**手動保有65件ぶんの空き現物があると通ってしまう余地がある** |
| Low | 失効済み（`expire_date < now`）のレコードもローリング対象になる（抽出条件に下限がない）。`CancelOrder` が失敗して実態と異なる「キャンセル失敗」通知が1回出る |
| Low | 発注レスポンス消失時のオーファン注文（Sprint 7 の `reconcileJob` で拾うのが妥当） |
| Info | Slack 文言の `"Old%s"` 連結が `"OldOrderID=..."` と読める / `RowsAffected` の `rowsErr != nil` 時に行数未確認で INSERT へ進む |

### 実機未確認

実発注を伴う E2E（発注 → キャンセル → 個別照会 → 再発注）は**未実施**。`pending_user_approval` に3件記載。


---

## Sprint 4 — 修正ラウンド（Evaluator 指摘の是正）＋ 実機 E2E

**判定: pass（回帰確認）** — 取引安全性 8 → **9** に改善

| 基準 | スコア | 閾値 |
|---|---|---|
| 機能完成度 | 9 | 7 |
| バグ | 8 | 7 |
| 回帰 | **10** | 8 |
| 取引安全性 | **9** | 8 |

`make test` **21 passed / 0 failed**。`go vet` / `gofmt` はベースラインのみ。

### 実機 E2E（ユーザー承認のもとメイン会話が実行・2026-09-07）

条件: ETH_JPY / SELL / LIMIT / price=590,000（LTP 392,009 の 1.51倍）/ size=0.01 / `minute_to_expire`=10 / GTC

| OrderID | 結果 |
|---|---|
| `JRF20260907-010131-018801` | 発注 → キャンセル済み |
| `JRF20260907-010136-007913` | 再発注 → 10分で自動失効 |

いずれも `executed_size: 0.0` で未約定。拘束は残らず。

**得られた3つの確定事実:**

1. **キャンセル成功時のレスポンスは `HTTP 200 + 空ボディ（len=0）`** → スプリント1の `CancelOrder()` 検証ロジック（2xx + 空ボディのみ成功、パース不能は失敗側）が実機と一致することを確認。ずっと未確認だった箇所を解消
2. **キャンセル後の個別照会は `[]` を返す** → 「消滅＝キャンセル成立」判定が正しい
3. **キャンセル後に `executed_size` / `outstanding_size` を確認する手段は原理的に存在しない** → Evaluator が提案した「③で `executed > 0` を検出して skip」という後追い方式は**実装不可能**であることが判明。実機データがなければ動かない修正を入れていた

### 修正内容

**Medium: 部分約定の検出をキャンセル前に移動**

処理順序（`rolloverSellOrderJob.go` の行番号順で担保）:
```
:383 キャンセル前の個別照会（size / executed_size / outstanding_size を取得）
:336 DB の size を残数量へ補正
:435 CancelOrder 成功確認
:445 キャンセル後の個別照会で再確認
:479 残数量で再発注
:494 単一トランザクションで DB 更新
```

- `remainingSize()` は `outstanding_size` を使い、0以下なら `size - executed_size` で補う
- 残数量が最小取引単位未満なら**キャンセルせずスキップ + 通知**（キャンセルすると裸の保有になり再発注もできないため）
- **キャンセル前に DB の `size` を残数量へ補正**（`UpdateOrderSizeWithRemark`）。理由: キャンセル後は照会不能なので、再発注に失敗して `[ROLLOVER_PENDING]` で翌日に持ち越した際に残数量を再取得できない。DB に持たせることで**翌日リトライが元の過大数量で再発注する事故**を防ぐ。補正失敗時はキャンセルせず中止

**Low: 失効済みレコードをローリング対象から除外**
- `expire_date` あり: `now < expire_date < now + daysBefore日`
- `expire_date` NULL: `now - (fallbackDays + daysBefore)日 < timestamp < now - fallbackDays日`

**Info**: `"Old%s"` 連結の解消、`RowsAffected()` の `rowsErr != nil` 時のロールバック

### 損益計算への影響（Evaluator が実クエリで裏取り）

部分約定時の損益の扱いについて、Generator の設計判断が正しいことを実測で確認:

| ケース | 損益 |
|---|---|
| 通常ローリング | `+149.84`（1行のみ・二重計上なし） |
| **採用方式**（旧を CANCELLED） | `-4,919.58`（過小計上） |
| 代替案（同一 parentid で FILLED 2行） | `-9,839.17`（**買いコスト二重計上で歪みが約2倍**） |

→ **安全側（利益を過大に見せない）に倒す判断は妥当**。恒久対応（買い注文側の数量按分）は別課題。

### 残った issues（Phase 3 のコードレビューで判断）

| 深刻度 | 内容 |
|---|---|
| Low | `remainingSize()` の浮動小数誤差。`0.03 - 0.02 = 0.0099999999999999985` となり ETH 最小単位 0.01 を割る。フェイルセーフ側の誤判定だが、**正当なローリングがスキップされ続ける**。8桁丸め or イプシロン比較で解消 |
| Low | 取引所側で既に CANCELED 済みの注文が最小単位未満だった場合、通知文が「注文はそのまま残します」で実態（裸の保有）と食い違う |
| Info | ファイル冒頭のパッケージコメントの処理順序が旧仕様のまま（キャンセル前照会が書かれていない） |
| Info | キャンセル失敗が続くと `/ partially filled: ...` が毎日 remarks に追記される |
| Info | 「`Old%s` 連結の解消」は Slack 出力が1バイトも変わっていない（design-dock §6.2 が `OldOrderID={id}` と明記しているため文言維持は妥当。変えるなら設計書の更新が必要） |
| 申し送り | PlaceOrder レスポンス消失 → 翌日2本発注（Sprint 7 `reconcileJob` 待ち）／部分約定時の損益過小計上の恒久対応 |


---

## Sprint 6 — 利確幅の戦略別可変化

**判定: pass（1回目）** — 機能完成度9 / バグ9 / 回帰10 / 取引安全性9。`make test` 23件PASS。

### 実装内容

利確率のマッピングを `go/enums/strategy.go` の `sellProfitRates` 1箇所に集約（`models` 側の戦略値ハードコード分岐は完全除去）。

| 戦略 | 値 | 利確率 |
|---|---|---|
| StrategyLTP99 | 10001 | ×1.02 |
| StrategyLTP98 | 10002 | ×1.03 |
| StrategyLTP97 | 10004 | ×1.05 |
| StrategyLTP95 | 10003 | ×1.05 |
| 7日安値ブレンド | 20001/20002/20003 | ×1.05 |
| Stg3BtcLtp90 / Stg14EthLtp90 | 3 / 14 | ×1.03（既存挙動を維持） |
| 上記以外（99/127/90001/旧0,1,2,10-13/未定義） | — | ×1.015（フォールバック） |

**持ち越し対応**: `placeSellOrder.go` のループ内 `break` 4箇所を **`continue` 化**（1件の失敗で全売り注文が止まる問題を解消）。エラーパス5箇所すべてに Slack 通知を配置（従来無通知だった `UpdateFilledOrderWithBuyOrder` 失敗経路にも追加）。

### Evaluator の実データ試算

- 未約定売り注文0件・約定済みで売り未発注も0件 → **デプロイ直後の即時影響はゼロ**
- 利確幅が上がるのは未約定買い注文4件のみ。全部到達しても**追加実現益は概算 +625円**
- **戦略値が記録されている買い注文は4件のみ**（2025-12〜2026-05 は tinyint 飽和で127が472件、他は既定値99が208件）→ 効果は今後の約定にのみ現れる
- 運用注意: +5%化で30日以内の到達率が 80% → 56〜67% に下がり、**ローリング件数と資金拘束が増える**見込み。`max_sell_orders=60` の警告閾値の推移を要モニタ

### issues（Low 3件）

- `StrategyLTP95` のコメント「廃止」と実装の食い違い（`buyingBTCJobLTP95TEST` が `is_test=true` 時に現存）
- 旧戦略3/14 を×1.03 に据え置いた根拠の一部が実DBと不一致（約定実績0件）
- `continue` 化により恒常的失敗時に通知が件数分飛ぶ

---

## Sprint 7 — 日次リコンサイル＆アラート（reconcileJob）

**判定: pass（1回目）** — 機能完成度8 / バグ8 / 回帰9 / 取引安全性9。`make test` **27件PASS**。

### 実装内容

**新規 `go/app/bitflyerApp/reconcileJob.go`**（`trigger_time_07` 既定 06:15 JST）

| 検知項目 | 内容 |
|---|---|
| 注文突合 | 取引所 ACTIVE ⇔ DB `UNFILLED` を**売買方向ごと**に。「DBのみ」「取引所のみ」双方向。取得失敗ペアは突合をスキップ |
| 残高突合 | `GetBalance` ⇔ `GetExpectedHoldings()`（bot / naked / manual の3内訳） |
| 発注ゼロ検知 | `no_order_alert_days` 既定3日。`enums.IsBotStrategy()` で 99/127/90001 を除外 |
| `[ROLLOVER_PENDING]` 残留 | 件数＋OrderID/ParentID/price/size |
| スロット状況 | 件数と上限値（28/60）を毎回サマリ掲載 |

`models.RemarkManualHold = "[MANUAL_HOLD]"` を定数化。`enums.IsBotStrategy()` を Sprint 3 での削除以来、実際に必要になったこのスプリントで追加（`reconcileJob.go:345` から実使用）。

### 「鳴りっぱなし」対策の方針

主軸は**乖離の向き**。「実残高 < 想定」（不足＝売り注文が残高不足で失敗しうる）のみエラー通知、「実残高 > 想定」（余剰）はサマリに数値を載せるのみ。本番の未追跡余剰は常に余剰側なので**設定を変えなくても鳴らない**（Evaluator が本番実データで再現確認済み）。

### 申し送り5件の取捨選択

| # | 対応 | 理由 |
|---|---|---|
| 1 INSERT失敗で戦略値喪失 | 部分（検知のみ） | 手動発注と区別不能なためエラー通知にせず、直近24hの `strategy=90001` 件数をサマリ表示 |
| 2 オーファン注文 | **対応** | 「取引所のみ ACTIVE」で検知 |
| 3 `[ROLLOVER_PENDING]` 残留 | **対応** | 正常時は翌日解消され鳴りっぱなしにならない |
| 4 通知の集約 | 見送り | pass済みジョブの制御フロー改変で回帰リスク大。目的は日次1通の検知で達成済み |
| 5 再起動検知 | 見送り | 曜日別12ジョブの許容幅定義が必要で誤検知しやすい。代わりに直近24hのボット発注件数を可視化 |

### ★実データで検知した「裸の保有」

```
buy_orders.id=612  JRF20260505-012050-073720  ETH_JPY  367,270 × 0.01  2026-05-05
status = FILLED(SELL ORDER PLACED) なのに sell_orders に対応レコードが0件
```

Evaluator が検知ロジックの正しさ（誤検知でないこと）を確認。**今後の扱い**: `CheckFilledBuyOrders`（`status='FILLED'` 限定）/ `expireSweepJob`（UNFILLED 限定）/ `rolloverSellOrderJob`（sell UNFILLED 限定）の**いずれの対象にもならず放置される**。本番DBの更新が必要でユーザー対応事項。

### issues（Critical/High ゼロ）

| 深刻度 | 内容 |
|---|---|
| **Medium** | 手動保有を Expected に**加算**する方式のため、**ユーザーが手動保有を実際に売却した時点から毎日🚨不足アラートが鳴り続ける**構造。design-dock:616 の「Expected から除外」に対する実装差異 |
| **Medium** | 裸の保有 `id=612` に自動・手動いずれの復旧経路もなく永久にサマリ表示される |
| Low | Sprint 5 の申し送り「sell超過警告を Sprint 7 の通知設計で吸収」が未消化 |
| Low | `GetBalance` が対象通貨を含まない場合、map のゼロ値0を実残高として誤判定しうる |
| Low | `[ROLLOVER_PENDING]` レコードが「DBのみ」通知と残留通知で二重に鳴る |
| Low | `untracked_holding_*` が design-dock の設定一覧に未記載 |
| Low | `service.go` に EventBridge の EC2 稼働窓を根拠にした古いコメントが残存 → **メイン会話が修正済み** |


---

## Sprint 7 — 修正ラウンド（残高突合から手動保有を除外）

**判定: pass（回帰確認）** — 機能完成度9 / バグ9 / 回帰10 / 取引安全性9。`make test` 27件PASS。

### 修正の背景（ユーザー指摘）

実装は手動保有分を Expected に**加算**していたが、ユーザーの当初指示（「システムからは存在しないものとして無視してください」）および `design-dock.md:616`（「Expected から除外」）と**真逆**だった。

`[MANUAL_HOLD]` 65件はどのジョブも触らないため永久に固定値であり、**ユーザーが手動保有を実際に売却した時点から恒久的に🚨不足アラートが鳴り続ける**構造になっていた。

### 修正内容

```go
// 修正前
diff.Expected      = holding.Total() + target.Untracked        // Total() = Bot + Naked + Manual
// 修正後
diff.AlertExpected = holding.AlertTarget() + target.Untracked  // AlertTarget() = Bot + Naked
```

- `ExpectedHolding.AlertTarget()` を新設。`Total()` は表示・調査用として残し、doc に「⚠️ 乖離判定に使ってはならない」と明記
- `balanceDiff.Expected` → **`AlertExpected`** に改名
- Slack を「判定用DB想定 / 判定対象の内訳 / 参考(判定対象外) 手動保有」の3段構成に
- `design-dock.md` に「残高の乖離判定の式（確定仕様）」を新設、`untracked_holding_btc/eth` を設定一覧に追記

### Evaluator の実測（ローカルDB + httptest で実コードを実行）

`reconcileBalances()` をそのまま動かし、取引所APIとSlackを httptest に差し替えて検証。

| ケース | 実残高 | AlertExpected | Diff | 判定 | Slackエラー |
|---|---|---|---|---|---|
| A 手動保有あり | 0.34 | 0.02 | +0.32 | 余剰 | **0件** |
| B **手動売却後** | 0.02 | 0.02 | 0 | 乖離なし | **0件** ★ |
| C ボット分が不足 | 0.00 | 0.02 | −0.02 | 🚨不足 | 1件 |
| D 手動売却＋ボット分も不足 | 0.01 | 0.02 | −0.01 | 🚨不足 | 1件 |

同テスト内で修正前方式を再現すると B は `oldDiff=-0.32` で shortfall となり、恒久的に鳴り続ける構造だったことも確認。

### 本番実データでのドライラン（SELECT + 参照系GETのみ）

`reconcileJob()` を丸ごと1回実行（Slack は httptest）:
- 注文突合: BTC BUY 6/6・BTC SELL 0/0・ETH BUY 5/5・ETH SELL 0/0 → **乖離0件**
- 発注ゼロ検知: 直近24h 4件・手動取り込み0件・最終ボット発注 2026-09-06T22:18:46Z
- `[ROLLOVER_PENDING]` 0件 / スロット「未約定buy:11/28 未約定sell:0/60」/ `[MANUAL_HOLD]` 130件
- 残高突合: 両通貨とも **SURPLUS** → 通知なし
- **Slack 投稿は日次サマリ1通のみ、エラー投稿0件**

---

## 本番DB修正 — 裸の保有 `id=612` の復元（メイン会話が実施）

Sprint 7 の `reconcileJob` が実装当日に実データで検知した1件。

```
buy_orders.id=612  JRF20260505-012050-073720  ETH_JPY  367,270 × 0.01  2026-05-05
status = FILLED(SELL ORDER PLACED) なのに sell_orders に対応レコードが0件
```

**調査結果**: 取引所の COMPLETED 履歴を `before` ページングで遡ったところ、売り注文 `JRF20260506-063932-066706`（ETH_JPY / SELL / **372,779** × 0.01）が **2026-05-06T06:39:32Z に実際に約定済み**であることが判明。買値 367,270 × 1.015 = 372,779.05 と完全一致し、`executed_size=0.01`。

→ **「裸の保有」ではなく、DB の `sell_orders` レコードだけが欠落していた**ケース。ユーザー承認のもと（「注文キャンセルしてもよいし、データを書き換えてもよいよ。なんとか直して」）、欠落レコードを `status='FILLED'` で復元した（`remarks` に取引所APIで確認した根拠を記載）。

**結果**: 裸の保有 **0件**。失われていた利益 **55.09円** が損益レポートに計上。Evaluator が `getResultsPostgres()` で**二重計上が無いこと**（同一 parentid の FILLED が2件以上あるケースは全DBで0件、日付別集計 2026-05-06 は count=1 / profit=55.09）を確認済み。

---

## Phase 2 完了サマリー

**全7スプリント pass**（Sprint 4 と Sprint 7 は指摘修正後の回帰確認も pass）

| 指標 | 変更前 | 変更後 |
|---|---|---|
| `make test` | 16件 | **27件** |
| `go vet` の指摘 | 5件 | **1件**（ベースラインのみ） |
| 差分規模 | — | 変更18ファイル（+962/−215）＋ 新規9ファイル |

### Phase 3 に持ち越す issues

| 深刻度 | 内容 | 出所 |
|---|---|---|
| Low | `remainingSize()` の浮動小数誤差（`0.03−0.02=0.0099999999999999985`）で正当なローリングがスキップされる | S4 |
| Low | `expireSweepJob` 方式Aに「遡り限界より古いものは判定保留」ガードが無い | S3 |
| Low | `cancelBuyOrderJob` の `Timestamp` ゼロ値時の扱いが方式Bと非対称 | S3 |
| Low | `service.go` の `"22:45"` がハードコード（他は config 化） | S3 |
| Low | `GetUnfilledBuyOrderRecords` が exchange でフィルタしていない | S3 |
| Low | 既に CANCELED 済みの注文が最小単位未満のときの通知文が実態と食い違う | S4 |
| Low | sell 超過警告の未集約（通知が件数分飛ぶ） | S5/S6/S7 |
| Low | `GetBalance` に対象通貨が無い場合のゼロ値誤判定 | S7 |
| Low | `[ROLLOVER_PENDING]` の二重通知 | S7 |
| Low | サマリのラベル `余剰(DB未追跡分。アラート対象外)` が修正後の実態と食い違う | S7 |
| Info | `bitflyer.Ticker.DateTime()` が常にパース失敗（既存バグ・参照0件） | S2 |
| Info | 部分約定時の損益過小計上の恒久対応（買い注文側の数量按分） | S4 |
| 環境 | systemd の NTP 同期待ち（`After=time-sync.target`）。Pi に RTC が無く、再起動のたびに意図しない発注が起きうる | 実運用で発生 |
| 環境 | `scripts/migration/` の2ファイルに DBパスワードが平文でコミットされている | S2 |
