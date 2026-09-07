# 製品仕様書: 注文ライフサイクル基盤の再構築（order-lifecycle-overhaul）

## 概要

Bitflyer 自動売買ボットの「注文ライフサイクル管理」を作り直す。取引所側で失効した注文を DB が検知できず `UNFILLED` のまま残り続ける（幽霊レコード）ことでスロットが枯渇し、2026-05-01 以降ボットが買い注文を1本も発注できなくなった。本機能では (1) 注文の有効期限を DB に持ち失効を確実に検知する、(2) 売り注文を27日周期でキャンセル→再発注して実質無期限化する、(3) 戦略メタデータの記録を復旧し戦略パラメータを実データに基づいて見直す、(4) 日次で取引所と DB を突合して乖離を Slack へ通知する、の4点を実現する。全7スプリントで構成する。

## 背景・目的

### 発生した障害

- `models.ShouldPlaceBuyOrder()` は「未約定 buy < `max_buy_orders`(15)」かつ「未約定 sell < `max_sell_orders`(25)」の両方を満たすときのみ発注を許可する。
- 2026-05 時点で未約定 buy 21件・未約定 sell 65件となりスロットが枯渇。以降すべての買い注文がスキップされた。
- そのレコードは取引所には存在しない「幽霊レコード」だった。売り注文は `MinuteToExpires: 43200`（30日）で失効していたが、DB 側に失効を検知する仕組みが一切なかった。

### 復旧作業（実施済み・本機能のスコープ外）

- 失効した買い注文14件・売り注文65件を `CANCELLED` に更新済み。
- その親買い注文65件は `FILLED(SELL ORDER PLACED)` のまま据え置き、`remarks` に手動保有への移管を示す日本語文言を付与済み。**この65ポジション（BTC 0.035 / ETH 0.320）はユーザーが手動で売却する。本機能で実装するジョブは一切関与してはならない。**
- バックアップテーブル `buy_orders_bk20260906` / `sell_orders_bk20260906` を取得済み。

### 前提条件（スプリント7の着手前に完了していること）

手動保有レコードを機械可読に識別するため、既存の 65（`buy_orders`）+ 65（`sell_orders`）レコードの `remarks` に **`[MANUAL_HOLD]` トークンを追記済み**であること。この付与作業はメイン会話がユーザー承認を得たうえで SQL で実施するものであり、**Generator の作業スコープ外**である。

- 現在の `buy_orders.remarks`: ` [20260906 手動保有へ移管:売り注文失効。今後システムは関与しない]`
- 現在の `sell_orders.remarks`: ` [20260906 手動整合:30日期限切れで失効。現物は手動保有へ]`
- 付与後は上記の末尾（または先頭）に ` [MANUAL_HOLD]` が付いた状態になる。

### 取引所 API の実測エビデンス（read-only GET で確認済み・設計の前提）

| # | 事実 | 設計への含意 |
|---|---|---|
| 1 | `getchildorders` の `count` は既定100・実効上限500。501以上を指定しても黙って500件で頭打ち。公式ドキュメントに上限記載なし | 500 をハードコードせず定数化し、`before` ページングを併用する |
| 2 | `before` パラメータによるページングは正常動作（2000件遡行を確認） | 全件走査が必要な照合はページングで実装可能 |
| 3 | **失効した注文は API から完全に消える**。`child_order_state=EXPIRED` は BTC/ETH とも0件。`child_order_acceptance_id` 個別指定でも0件。state 未指定で2000件遡っても発見不可（対照実験として ACTIVE/COMPLETED の個別指定は1件返るためフィルタ自体は正常） | **`child_order_state=EXPIRED` による失効検出は実装不可能。** 失効は「DB に持った有効期限の経過」で判定するのが主、「一覧からの消滅」は補助 |
| 4 | API は各注文に `expire_date` を返す（ACTIVE・COMPLETED いずれでも）。本作業でも実測: `expire_date=2026-10-05T22:41:05` / `child_order_date=2026-09-05T22:41:05`（タイムゾーンサフィックスなし＝UTC、秒精度） | `expire_date` を DB に保存でき、API 値で補正もできる |
| 5 | Bitflyer の注文有効期限の上限は30日（43200分）。無期限注文は作れない | 売り注文を保持し続けるには定期的なキャンセル→再発注（ローリング）が必須 |

### 現状コードの既知の欠陥

| 箇所 | 欠陥 |
|---|---|
| `go/bitflyer/bitflyer.go:294` `CancelOrder()` | HTTP 通信エラーしか返さず、Bitflyer のエラー JSON を検証していない。**キャンセル成否が判定できない** |
| `go/bitflyer/bitflyer.go:154` `GetActiveBuyOrders()` | `count` 未指定でデフォルト100件のみ。名前に反して売り注文も全状態も返す |
| `go/models/events.go:211-215` `CheckFilledBuyOrders()` | SQL で `strategy` を SELECT・Scan しているのに `BuyOrderInfo` へ代入していない。結果 `Strategy` が常に 0 になり `CalculateSellOrderPrice()` の戦略分岐が死んでいる |
| `go/models/events.go:269` `SyncBuyOrders()` | `strategy` を INSERT していない（DB デフォルト99になる） |
| `buy_orders.strategy` の過去データ | MySQL(RDS) 時代の `tinyint` により戦略値 10001〜20002 が **127に飽和**し、2025-12〜2026-05 の470件が区別不能。**現行の本番 PostgreSQL では `smallint`（上限32767）であり飽和はすでに解消済み**だが、過去データは復元不能のまま残っている |
| `go/app/bitflyerApp/placeSellOrder.go` | ループ内エラーで `break` するため、1件の失敗で以降の全売り注文が止まる |
| `go/app/bitflyerApp/filledCheckJob.go` | `GetActiveBuyOrders` 経由で100件しか取得しない。`log.Println` にフォーマット引数を渡す `go vet` 指摘あり（既存ベースライン） |

### 実データに基づく戦略評価（設計の根拠）

買い指値の深さ別の約定率（2025-12-13〜2026-05-01、ボット発注468件）:

| 割引 | 該当ジョブ | 発注数 | 約定率 | 実現利益 |
|---|---|---|---|---|
| −1% | LTP99（毎日） | 275 | 77.5% | 20,920円 |
| −2% | LTP98（火・土） | 132 | 67.4% | 8,656円 |
| −5% | LTP95（月） | 61 | **21.3%** | 934円 |
| −6.5%以深 | 7日安値ブレンド（水・日） | 5 | **0%** | 0円 |

疑似バックテスト（price_histories 全期間、BTC/ETH）:

- 買い指値到達率 — 期限2.5日: −1%=51/53%, −2%=34/41%, −3%=22/31%, −5%=9/15%。期限7日: −1%=70/70%, −2%=59/60%, −3%=44/50%, −5%=27/37%
- 利確到達率（30日以内）: +1.5%=80/80%, +3%=71/73%, +5%=56/67%

→ **深い指値は期限が短すぎて機能していない**（期限を2.5日→7日にすると −3% でも到達率が2倍になる）。**利確幅は +1.5% に固定する必然性がなく、浅く買ったものは +2%、深く買ったものは +5% を狙える。**

## 機能一覧

| # | 機能名 | 説明 | 優先度 |
|---|---|---|---|
| 1 | Bitflyer API クライアントの堅牢化 | `CancelOrder()` のレスポンス検証、`count` 指定、`before` ページング、個別注文照会、命名是正 | 必須 |
| 2 | スキーマ変更 | `buy_orders` / `sell_orders` への `expire_date` 追加とインデックス追加。`schema.hcl`(MySQL) 側の `strategy` 型拡張 | 必須 |
| 3 | 有効期限の記録 | 発注時・同期時に `expire_date` を保存し、API 値で補正する | 必須 |
| 4 | 戦略メタデータの記録復旧 | `CheckFilledBuyOrders()` の代入漏れ修正、`SyncBuyOrders()` への戦略値付与、手動注文用の戦略値定義 | 必須 |
| 5 | 失効検出ジョブ（expireSweepJob） | 期限切れかつ未約定のレコードを `CANCELLED` に落とす。`expire_date` が NULL の旧レコードは一覧からの消滅で補助判定 | 必須 |
| 6 | 約定チェック・キャンセルジョブの整合 | `filledCheckJob` の取得件数を500へ。`cancelBuyOrderJob` の判定基準を実際の注文期限と整合させる | 必須 |
| 7 | 売り注文の27日ローリング（rolloverSellOrderJob） | 期限3日前にキャンセル→同条件で再発注し、売り注文を実質無期限化する | 必須 |
| 8 | 買い指値・期限・スロット設計の見直し | 月曜を −3% へ、日曜ブレンド比率を浅く、買い注文期限を7日へ、`max_buy_orders`=28、`max_sell_orders`=60（警告のみ） | 重要 |
| 9 | 利確幅の戦略別可変化 | LTP99:+2% / LTP98:+3% / それ以深:+5%。未知戦略は現行の +1.5% を維持 | 重要 |
| 10 | 日次リコンサイル＆アラート（reconcileJob） | 取引所の ACTIVE 注文・残高と DB を突合。乖離／発注ゼロ継続／スロット警告を Slack 通知 | 重要 |

## スプリント計画

全7スプリント。1スプリントずつ順番に実施する。

### スプリント 1: Bitflyer API クライアント層の堅牢化（機能1）

- **ゴール**: 注文のキャンセル成否を判定でき、注文一覧を100件制限なしに取得できる状態にする。他レイヤの挙動は変えない。
- **対象範囲**: go
- **機能**:
  - [ ] `CancelOrder()` のレスポンス検証: `go/bitflyer/bitflyer.go` を読み、HTTP ステータスコードとレスポンスボディの `status` / `error_message` を判定して成否を返していることを確認できる。空ボディ＋2xx を成功、それ以外を失敗として扱い、パース不能なレスポンスは**失敗側に倒す**こと（コード追跡で確認）。
  - [ ] `count` パラメータ対応: `getchildorders` を叩くメソッドが `count` を受け取り、上限値が `MaxChildOrdersCount = 500` として定数定義されていること（マジックナンバーが残っていないことを grep で確認）。
  - [ ] `before` ページング: 指定ページ数まで `before` を用いて遡って取得するメソッドが存在し、`before` に前ページ最小 `id` を渡していることをコード上で確認できる。無限ループ防止の上限（最大ページ数）が実装されていること。
  - [ ] 個別照会メソッド: `child_order_acceptance_id` を指定して1件取得するメソッドが追加され、**「生きている注文の存在確認」専用**である旨のコメントがあること（失効判定に使えないことを明記）。
  - [ ] 命名是正: `GetActiveBuyOrders` が実態に合う名前（`GetChildOrders`）に変更され、呼び出し元 `go/app/bitflyerApp/syncBuyOrders.go` / `filledCheckJob.go` が更新されていること。
  - [ ] `make build` が通り、`go vet ./bitflyer/...` が本スプリントで変更したファイルについて新規指摘を出さないこと。

### スプリント 2: スキーマ変更と記録の正常化（機能2・3・4）

- **ゴール**: 注文の有効期限と戦略値が正しく DB に記録される。
- **対象範囲**: db / go
- **前提**: ユーザーによるマイグレーション適用（本番 PostgreSQL への `expire_date` 追加とインデックス追加）が完了していること。適用の実行はユーザーが行う。
- **機能**:
  - [ ] スキーマ差分: `db/crypto-trading-db/atlas/schema.hcl` に `buy_orders.expire_date` / `sell_orders.expire_date`（timestamp, NULL 許容）と2本のインデックスが追加され、`buy_orders.strategy` が `int` に変更されていること。
  - [ ] 本番 PostgreSQL 用 DDL が `db/crypto-trading-db-postgres/migrations/` に追加されていること（`expire_date` 2本 + インデックス2本が必須。`strategy` の `INTEGER` 化は任意扱いで明示的にコメントされていること）。
  - [ ] `db/crypto-trading-db-postgres/init/001_init.sql` にも同じ定義（`expire_date` 追加 / `strategy` を **`INTEGER`** に変更 / インデックス2本）が反映され、`make db-up && make test` が通ること。**本番 Supabase は 2026-09-06 に適用済みのため、新規マイグレーションファイルの作成は不要。**
  - [ ] `sell_orders.remarks` の長さ検証結果が design-dock に記載されていること（結論: MySQL `TEXT` = 65,535バイト / PostgreSQL `TEXT` = 実質無制限 のため**変更不要**）。
  - [ ] 買い注文発注時の期限記録: `placeBuyOrder` が `expire_date`（UTC）を計算して INSERT に含めていることをコード追跡で確認できる。
  - [ ] 売り注文発注時の期限記録: `placeSellOrder` が同様に `expire_date` を保存していること。`models.OrderEvent.SellOrder()` が `remarks` と `expire_date` を渡せるよう拡張され、**既存の呼び出し（`go/tests/integration_test.go:103`）がコンパイルできること**。
  - [ ] 同期時の期限補正: `syncBuyOrders` が API の `expire_date` をパースして保存・上書きすること。パーサが `2006-01-02T15:04:05` 形式（および小数秒付き）を UTC として扱うことをコード追跡で確認できる。
  - [ ] 戦略値の代入漏れ修正: `models.CheckFilledBuyOrders()` が `BuyOrderInfo.Strategy` に SELECT した値を代入していること。
  - [ ] 手動注文の戦略値: `enums` に `StrategyManual` が定義され、`models.SyncBuyOrders()` が INSERT でその値を渡していること。
  - [ ] 過去データの扱い: `enums.StrategySaturatedUnknown = 127` / `enums.StrategyUnknown = 99` が定義され、**データの書き換えを一切行っていない**こと。
  - [ ] `make build` と `make test` が通ること。

### スプリント 3: 失効検出（機能5・6）

- **ゴール**: 幽霊レコードが自動的に `CANCELLED` に落ち、スロットが枯渇しない。
- **対象範囲**: go
- **機能**:
  - [ ] 新規ジョブファイル `go/app/bitflyerApp/expireSweepJob.go` が追加されていること（ジョブ1つ＝1ファイルの開発ルール準拠）。
  - [ ] 方式A（主）: `expire_date` が経過（猶予時間つき）かつ `status='UNFILLED'` のレコードを `CANCELLED` に更新すること。**更新前に COMPLETED 一覧と突合し、約定済みなら `FILLED` に更新して sweep 対象から除外する**ことをコード追跡で確認できる。
  - [ ] 方式B（補助）: `expire_date` が NULL の `UNFILLED` レコードについて、ACTIVE 一覧にも COMPLETED 一覧にも存在しない `order_id` を失効とみなすこと。**COMPLETED 一覧で遡れた最古の `child_order_date` より古いレコードは判定保留とし、Slack に保留件数を通知する**（遡り不足による誤判定を防ぐフェイルセーフ）。
  - [ ] ローリング再試行待ちマーカー（`[ROLLOVER_PENDING]`）が付いたレコードを sweep の対象から除外すること（スプリント4との競合防止。マーカー文字列は定数化）。
  - [ ] `filledCheckJob` が500件取得に変更され、`log.Println` へのフォーマット引数渡し（`go vet` 指摘）が解消されていること。
  - [ ] `cancelBuyOrderJob` が固定日数（`utils.BfCancelCriteria = -3`）ではなく `expire_date` と設定値 `buy_order_cancel_days` に基づいて判定していること。
  - [ ] ジョブが `service.go` に 06:05 JST でスケジュール登録されていること。
  - [ ] `make build` / `make test` が通り、変更ファイルに `go vet` の新規指摘がないこと。

### スプリント 4: 売り注文の27日ローリング（機能7）

- **ゴール**: 30日で失効するはずの売り注文が、27日目にキャンセル→再発注され保持され続ける。
- **対象範囲**: go
- **機能**:
  - [ ] 新規ジョブファイル `go/app/bitflyerApp/rolloverSellOrderJob.go` が追加され、05:30 JST に1日1回スケジュール登録されていること。
  - [ ] 対象抽出: `status='UNFILLED'` かつ `expire_date - 3日 < now()`。`expire_date` が NULL の旧レコードは `timestamp + 27日 < now()` でフォールバックすること。1回あたりの処理上限（`sell_rollover_max_per_run`、既定20）があること。
  - [ ] 手動保有65件（`sell_orders.status='CANCELLED'`）が抽出条件により自動的に対象外であることが design-dock に明記されていること。
  - [ ] 処理順序: ①約定確認 → ②`CancelOrder()` 実行と**成功の確認** → ③個別照会で消滅（または COMPLETED）を再確認 → ④同一 `product_code` / `price` / `size` で再発注、の順で実装されていること。③で `COMPLETED` が返った場合は**再発注せず `FILLED` に更新する**（二重売り防止）。
  - [ ] DB 更新: 旧レコードを `CANCELLED` にして `remarks` に再発注先を追記し、新レコードを INSERT する処理が**単一トランザクション**であること。
  - [ ] 新レコードの `parentid` が元の買い注文 ID を引き継いでいること（損益計算の紐付け維持）。
  - [ ] 新レコードの `remarks` に **`{旧order_id}の売り注文の再注文`** という文言が記録されること。
  - [ ] 新レコードに `expire_date` が保存されること。
  - [ ] 異常系: キャンセル成功・再発注失敗時に DB を変更せず、Slack へ即時通知し、`[ROLLOVER_PENDING]` マーカーを付けて次回ジョブでリトライされること。**1件の失敗で全体を止めない**（`break` ではなく `continue`）ことをコード追跡で確認できる。
  - [ ] 3日連続で失敗して売り注文が失効した場合、**親買い注文は `FILLED(SELL ORDER PLACED)` のまま戻さず、Slack 通知のみ行う**こと（自動再発注は行わない）。
  - [ ] Slack 通知に OrderID・ParentID・ProductCode・Price・Size・エラー内容が含まれること。
  - [ ] `make build` / `make test` が通ること。実発注を伴う E2E 検証はユーザー許可が必要なため、design-dock の「更新系APIでの確認」に手順を記載すること。

### スプリント 5: 買い指値・期限・スロット設計の見直し（機能8）

- **ゴール**: 約定しない深指値と短すぎる期限を是正し、買い注文が実際に約定する設計にする。売り注文の滞留でボットが止まらないようにする。
- **対象範囲**: go
- **機能**:
  - [ ] 月曜の戦略が −5%（`StrategyLTP95`）から −3%（`StrategyLTP97`）に変更され、`utils.CalculateBuyPrice()` に `Round(ltp * 0.97)` の分岐が追加されていること。
  - [ ] 日曜の `StrategyLtpLowestIn7days2t8`（`ltp*0.2 + low7*0.8`）が `StrategyLtpLowestIn7days7t3`（`ltp*0.7 + low7*0.3`）に置換されていること。**旧戦略定数は DB に値が残っているため削除せず enums に残す**こと。
  - [ ] `models.CalculateMinuteToExpire()` の既定値が 3600（2.5日）から 10080（7日）に変更され、値が `config.ini` の `buy_minute_to_expire` から設定できること（未設定時は 10080）。
  - [ ] `config.ini` の `max_buy_orders` が **28** に変更されていること。
  - [ ] `config.ini` の `max_sell_orders` が **60** に変更されていること。
  - [ ] `ShouldPlaceBuyOrder()` 相当のスロット判定が、**buy 側の超過では発注をブロックし、sell 側の超過では発注をブロックせず Slack 警告のみ出す**仕様に変更されていること。既存テスト `TestShouldPlaceBuyOrder`（`go/tests/integration_test.go:158`）がコンパイル・パスすること。
  - [ ] **価格履歴の記録頻度は1日2回（`trigger_time_03` / `trigger_time_04`）から変更しないこと。** `savePriceHistoryJob.go` および `service.go` の該当スケジュール登録、`config.go` の `TriggerTime03` / `TriggerTime04` に差分が無いこと（`git diff` で確認）。
  - [ ] `budget_criteria` の値を変更していないこと（ユーザーが環境側で管理する）。
  - [ ] `make build` / `make test` が通ること。

### スプリント 6: 利確幅の戦略別可変化（機能9）

- **ゴール**: 買い指値の深さに応じて利確幅を変え、深く買ったポジションの期待利益を上げる。
- **対象範囲**: go
- **機能**:
  - [ ] `models.BuyOrderInfo.CalculateSellOrderPrice()` が戦略別に利確率を返すこと（LTP99:×1.02 / LTP98:×1.03 / それ以深:×1.05）。
  - [ ] 新旧戦略の定数マッピングが1箇所（マップ）に集約され、未知の戦略値（`99` / `127` / `90001`）は**現行の ×1.015 にフォールバック**すること。
  - [ ] スプリント2の `Strategy` 代入漏れ修正が効いていることを前提に、`placeSellOrder` のログに戦略値と適用利確率が出ること。
  - [ ] `make build` / `make test` が通ること。

### スプリント 7: 日次リコンサイル＆アラート（機能10）

- **ゴール**: 同種の障害が起きたときに、無言で止まるのではなく必ず気づける。
- **対象範囲**: go
- **前提**: 手動保有130レコードへの `[MANUAL_HOLD]` トークン付与が完了していること（「前提条件」セクション参照）。
- **機能**:
  - [ ] 新規ジョブファイル `go/app/bitflyerApp/reconcileJob.go` が追加され、06:15 JST に1日1回スケジュール登録されていること。
  - [ ] 取引所の ACTIVE 注文（BTC_JPY / ETH_JPY、ページング取得）と DB の `UNFILLED`（buy / sell）を突合し、「DB にのみ存在」「取引所にのみ存在」の件数と代表 order_id を Slack 通知すること。
  - [ ] 取引所の BTC / ETH 残高と DB 上の想定保有量を突合し、閾値を超える乖離があれば Slack 通知すること。**手動保有分は `remarks LIKE '%[MANUAL_HOLD]%'` で識別し、内訳として別枠表示のうえ乖離アラートの対象から除外する**こと。
  - [ ] マーカー文字列が定数 `RemarkManualHold = "[MANUAL_HOLD]"` として定義され、日本語散文でのマッチを行っていないこと。
  - [ ] 「N日間ボットの発注が0件」を検知して Slack 通知すること（N は `no_order_alert_days`、既定3）。
  - [ ] スロット状況（未約定 buy / sell の件数と上限値）が毎回のリコンサイル通知に含まれ、sell 側が上限超過の場合は警告として表示されること。
  - [ ] `make build` / `make test` が通ること。

## 対象外（やらないこと）

- **相場レジーム対応（7日移動平均による深指値ジョブの停止、長期未約定売り注文のトレーリング）。** 今回のスコープから完全に除外する。実装しないこと。
- **手動保有65ポジション（`sell_orders.status='CANCELLED'` + 親 `buy_orders.status='FILLED(SELL ORDER PLACED)'`、BTC 0.035 / ETH 0.320）への一切の自動処理。** 抽出条件から自動的に外れることを設計で担保し、リコンサイルでは内訳表示のみ行う。
- **手動保有レコードへの `[MANUAL_HOLD]` トークン付与 SQL の実行。** メイン会話がユーザー承認を得て別途実施する前提条件であり、Generator の作業ではない。
- **ER図（`db/er-diagram.dio`）の更新。** 本プロジェクトではファイル管理していないため対象外。
- **過去データ（`strategy=127` 470件 / `99` 163件）の遡及 UPDATE。** 定数定義と集計からの除外のみ行い、データは書き換えない。
- **`budget_criteria` の変更。** JPY 残高の下限はユーザーが環境側で管理する。
- **`make atlas-apply` および本番 PostgreSQL（Supabase）への DDL 適用の実行。** マイグレーション SQL の作成・提示までがスコープで、適用はユーザーが行う。
- **MySQL(RDS) と PostgreSQL(Supabase) 間のデータ移行・切り戻し。**
- **OKEX / OKJ / bitbank 側のロジック変更。** `go/app/okextasks.go` / `okjTasks.go` / `bitbank/` は触らない（bitbank は BTC 買値の参照元としてのみ現状維持）。
  - ただし **Phase 3 のレビュー指摘 F6 の対応として、`bitbank.GetBBTicker()` の nil 参照 panic の修正のみ例外的に実施した**（`(*ReturnTicker, error)` を返す形に変更）。買値の算出ロジックそのものは変更していない。シグネチャ変更に伴い `okjTasks.go` の2箇所の呼び出しにも「取得失敗時は発注しない」ガードを追加している。
- **ボット全体の起動（`make run` / `make run-binary`）による E2E 検証。** 自動発注が走るためエージェントは実行しない。
- **ユニットテストの網羅的追加。** 既存の `go/tests/` 統合テストが壊れないことのみ担保する。
- **UI・画面の追加。** 本プロジェクトにフロントエンドは存在しない。出力面は Slack 通知・ログ・DB レコードのみ。

## 成功基準

1. **買い注文が止まらない**: `max_buy_orders` のカウント対象に幽霊レコードが含まれず、毎日の買い注文ジョブが発注に到達する。売り注文の滞留は発注をブロックしない。
2. **売り注文が失効しない**: `UNFILLED` の売り注文はすべて `expire_date` を持ち、期限3日前にローリングされる。30日経過による失効が発生しない。
3. **失効が検知される**: 何らかの理由で失効した注文は、翌日の sweep ジョブで `CANCELLED` になり、件数が Slack に通知される。
4. **戦略が記録される**: スキーマ適用日以降の買い注文は `buy_orders.strategy` に正しい戦略値を持ち、戦略別の約定率・実現利益を SQL で集計できる。
5. **無言の停止が起きない**: DB と取引所の乖離、N日間の発注ゼロ、スロットの逼迫のいずれもが Slack に通知される。
6. **既存が壊れない**: `make build` が通り、`make test`（既存統合テスト）が全件パスする。変更したファイルに `go vet` / `gofmt` の新規指摘がない。

## リスク・注意点

| リスク | 影響 | 緩和策 |
|---|---|---|
| **裸の保有**: ローリングでキャンセルは成功したが再発注に失敗する | 現物を保有したまま売り注文が存在しない状態になる（損失ではないが利確機会を失う） | DB を変更せず次回ジョブでリトライ。Slack 即時通知。`[ROLLOVER_PENDING]` マーカーで sweep との競合を防止。3日連続失敗時は親を戻さず通知のみ（ユーザーが手動対応） |
| **二重売り**: キャンセル要求と同時に約定し、再発注してしまう | 保有していない数量の売り注文＝残高不足エラー、または他ポジションの売り注文と重複 | キャンセル後に個別照会し、`COMPLETED` が返った場合は再発注せず `FILLED` に更新（フェイルセーフ） |
| **誤 CANCELLED**: 実際は約定した注文を失効とみなして `CANCELLED` にする | 保有した現物が DB 上追跡不能になり、売り注文が発注されない | sweep は必ず COMPLETED 一覧との突合を先に行う。方式Bは遡り範囲外のレコードを判定保留にする |
| **遡り不足による誤判定**: COMPLETED 一覧が500件×ページ数で古いレコードまで届かない | 約定済みを失効と誤判定 | 取得できた最古の `child_order_date` より古いレコードは判定保留＋通知 |
| **タイムゾーンずれ**: `expire_date` を JST で保存し UTC と比較する | 9時間ぶんの誤判定（早すぎるキャンセル／遅すぎる失効検知） | DB へは必ず UTC で保存。比較も UTC で行う。API の `expire_date` はサフィックスなしの UTC 表記としてパースする |
| **スケジューラの TZ 依存**: `scheduler.Every().Day().At()` はシステム TZ | 意図しない時刻に発注・キャンセルが走る | サーバ TZ = JST 前提を design-dock に明記し、ジョブ冒頭で現在時刻を JST/UTC 両方ログ出力する |
| **Pi の停止時間帯**: 実行環境は Raspberry Pi 上の systemd サービス（`Restart=always`）で原則24時間稼働だが、**毎日 01:30〜02:45 JST に Pi 自体が停止する**（ユーザー運用）。アプリ内の `gracefulShutdown`(01:20) は停止10分前に新規ジョブをブロックする意図した設計 | 停止時間帯にスケジュールしたジョブが実行されない | 新規ジョブは **01:20〜02:45 JST を避けて**配置する。`terraform` の EventBridge は RDS 時代の名残でアプリ実行環境とは無関係 |
| **本番 DB は PostgreSQL(Supabase)**: `schema.hcl` は MySQL 用で、Atlas マイグレーションは本番に効かない | DDL 未適用のまま新カラムに INSERT してランタイムエラー | schema.hcl 差分と PostgreSQL 用 DDL の両方を用意し、適用完了をユーザーに確認してからスプリント2以降に進む |
| **レート制限**: ローリング1件あたり cancel + 個別照会 + place = 3リクエスト | Private API 制限（5分あたり）に抵触 | `sell_rollover_max_per_run=20` で制限し、ループ内にスリープを入れる |
| **現物の積み上がり**: sell 側チェックが発注をブロックしなくなる | 未約定売り注文が増え続け、現物の保有量と拘束資金が単調増加する | 最終的な歯止めは `budget_criteria`（JPY 残高の下限）であり、**その値の管理はユーザー責務**。ボット側は `max_sell_orders=60` 超過を毎日 Slack 警告し、リコンサイルで残高を可視化する |
| **`[MANUAL_HOLD]` トークン未付与**: 前提条件が満たされないままスプリント7を実施する | 手動保有分が乖離アラートとして毎日誤検知される | スプリント7の着手前にトークン付与の完了を確認する |
