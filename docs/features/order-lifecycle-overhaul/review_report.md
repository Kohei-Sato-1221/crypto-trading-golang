# コードレビュー結果: order-lifecycle-overhaul

対象: `feature/order-lifecycle-overhaul` のワーキングツリー全体（未コミット。変更18ファイル +962/−215 ＋ 新規9ファイル）
レビュー日: 2026-09-07

## サマリー

| 深刻度 | 件数 |
|---|---|
| Critical | 1 |
| High | 3 |
| Medium | 6 |
| Low | 19 |
| **合計** | **29** |

うち **既知の持ち越し issues の再評価が 14 件**（各項目に「既知」と明記）、**今回のレビューで新たに検出したものが 15 件**。

### ゲート（Reviewer が再実測）

| 項目 | 結果 |
|---|---|
| `make build` | OK |
| `go vet ./...` | 指摘1件（`bitbank/bitbank.go:40` のベースラインのみ。**新規0件**） |
| `gofmt -l .` | `okex/okex.go` のみ（ベースライン） |
| `make test` | **全件 PASS** |
| `terraform/` | 変更なし（`terraform/build/crypto-regular-purchase/` は本作業以前からの未追跡ディレクトリ） |
| 機密情報のコミット | `private_config.ini` / `terraform.tfvars` / `db/envs/.pg.env` はいずれも未コミット。`.pg.env` は `.gitignore` 追加済みで適切 ✅（ただし F1 を参照） |

### 開発ルールの遵守状況

| ルール | 判定 |
|---|---|
| ジョブ関数を `bitflyerApp/` 配下で1ジョブ1ファイルに分離 | ✅ 遵守。`expireSweepJob.go` / `rolloverSellOrderJob.go` / `reconcileJob.go` / `cancelBuyOrderJob.go` を新規分離。`service.go` のインラインクロージャも解消 |
| エラーを必ず Slack へ通知・コンテキストを含める | ⚠️ app 層はほぼ徹底されている（OrderID/ParentID/ProductCode/Price/Size/Strategy を網羅）。**models 層に log のみの経路が残る → F9** |
| タイムゾーン（DBはUTC保存・UTC比較） | ✅ 遵守。`ParseBitflyerTime` に一本化、全新規ジョブが冒頭で local/UTC 両方をログ出力。ただし**システム時計そのものの信頼性に穴 → F4** |
| 機密情報をコミットしない | ❌ **F1**（既存ファイルだが Critical） |
| テストは最低限 | ✅ 追加された3ファイルはいずれも DB入出力・損益・価格計算に限定されており方針に適合。テスト不足を理由とした指摘は挙げていない |
| Terraform 開発ルール | 該当なし（変更なし） |

---

## Critical

### 1. 本番 Supabase のパスワードが平文でコミットされている 〔既知・環境〕

- **対象**: `scripts/migration/import_to_supabase.sh:8`, `scripts/migration/reset_supabase.sh:4`
- **分類**: security
- **根拠**: ルート `CLAUDE.md` > 開発ルール > 機密情報 / `design-dock.md` §10.2
- **内容**: 本番 PostgreSQL の接続文字列がユーザー名・パスワード・ホストごと平文で、git のトラック対象になっている。とくに `reset_supabase.sh` は DB 初期化スクリプトであり、この1行だけで本番データを破壊できる。
- **影響**: リポジトリを閲覧できる誰もが本番 DB を読み書き・削除できる。今回のブランチで本番 PostgreSQL を Atlas マイグレーション管理下に置いたため、この認証情報の重要度はさらに上がっている。
- **提案**: ① パスワードを即時ローテーション ② 2ファイルの DSN を `db/envs/.pg.env` 参照に置換 ③ 履歴にも残るため、共有リポジトリなら履歴書き換えも検討。
- **実機確認**: 不要

---

## High

### 2. `[ROLLOVER_PENDING]` レコードが恒久的に滞留し、設計が謳う失効通知が鳴らない 〔新規〕

- **対象**: `go/models/orderLifecycle.go:480-481`（rollover の抽出下限）, `:249` / `:292`（sweep の除外）, `go/app/bitflyerApp/rolloverSellOrderJob.go:204-211`
- **分類**: trading-safety
- **根拠**: `design-dock.md` §6.2「expireSweepJob | ローリング失敗の末に売り注文が失効」/ §7.2 確定仕様 / `spec.md` 成功基準 1・3
- **内容**: `GetSellOrdersToRollover` は `expire_date > now` という**下限**を持つため、元の期限を過ぎたレコードはローリング対象から外れる。一方 `expireSweepJob` の2クエリは `remarks NOT LIKE '%[ROLLOVER_PENDING]%'` で**無条件に除外**している。したがってキャンセル成功・再発注失敗が3日続いて期限を過ぎたレコードは、**ローリングにも sweep にも拾われず `status='UNFILLED'` のまま永久に残る**。
- **影響**:
  - `max_sell_orders` のスロットを食い続ける幽霊レコードが再生産される（＝本機能が解消しようとした 2026-05 の障害と同じ構造）。
  - design-dock §6.2 が「最後の砦」と位置づける `🚨🚨【expireSweep】売り注文が失効しました`（親買い注文は戻さず手動対応を促す通知）が**原理的に発火しない**。
  - 救いは `reconcileJob` の `[ROLLOVER_PENDING]` 残留アラートが毎日鳴ること。気づけはするが、DB は自動では直らない。
- **提案**: sweep の除外条件を「`expire_date` がまだ将来のレコードのみ除外」に変える、もしくは rollover 側の下限を `[ROLLOVER_PENDING]` 付きレコードに限って外す。あわせて sweep が CANCELLED にする際に §6.2 の失効通知を必ず出す。
- **実機確認**: 不要（コード追跡で確定）

### 3. `PlaceOrder` のレスポンス消失時に、翌日2本目の売り注文を発注する 〔既知（S4申し送り）・優先度を引き上げて再提示〕

- **対象**: `go/app/bitflyerApp/rolloverSellOrderJob.go:395-411`（`alreadyCancelled` 経路）, `:479-490`
- **分類**: trading-safety
- **根拠**: `design-dock.md` §7.1「二重売りの防止」/ `spec.md` リスク表「二重売り」
- **内容**: 再発注のリクエストが取引所に届いたのにレスポンスを受け取れなかった場合、`placeRolloverSellOrder` は error を返し `[ROLLOVER_PENDING]` を付けて終了する。翌日の再試行では旧 order_id の個別照会が「見つからない」ため `alreadyCancelled=true` の経路に入り、**確認なしでもう1本発注する**。事前の COMPLETED 突合は旧 order_id しか見ないので、前日に生まれたオーファン注文には気づけない。
- **影響**: 同一ポジションに対して売り注文が2本並ぶ。通常は残高不足で弾かれるが、**手動保有 65 件（BTC 0.035 / ETH 0.320）ぶんの拘束されていない現物が口座にある**ため、2本とも約定して「保有していないぶんの売却」または「ユーザーが手動売却する予定だった現物の勝手な売却」が起きうる。`reconcileJob` の「取引所のみ ACTIVE」で事後検知はできるが、翌 06:15 までのタイムラグがある。
- **提案**: `[ROLLOVER_PENDING]` 経路の再発注直前に、当該 product_code の ACTIVE 一覧（既に取得済みのものを流用可）を引き、同一 `price` / `size` / `side=SELL` で DB に紐づかない注文があれば再発注せず、その order_id を DB に取り込む（または通知して skip）。
- **実機確認**: 不要

### 4. systemd ユニットが時刻同期を待たない（Pi に RTC が無い） 〔既知・環境／実際に発生済み〕

- **対象**: `bfTradingApp.service:3`（`After=network.target mysql.service`）
- **分類**: timezone-schedule
- **根拠**: ルート `CLAUDE.md` > 開発ルール > タイムゾーン / `design-dock.md` §5.1
- **内容**: `time-sync.target` / `systemd-timesyncd` への依存が無い。Raspberry Pi には RTC が無いため起動直後のシステム時刻は不正確で、`scheduler.Every().Day().At(...)` が誤った現在時刻から次回実行時刻を計算する。NTP で時刻が前方へ飛んだ瞬間に買い注文ジョブが即時発火しうる。
- **影響**: 再起動のたびに意図しない発注が起きうる（**2026-09-07 に実際に発生**）。本設計は全ジョブがシステム TZ 依存なので、時計の正しさ＝発注の正しさになる。
- **提案**: `After=time-sync.target systemd-timesyncd.service` / `Wants=time-sync.target` を追加。あわせてアプリ側でも起動直後 N 分は発注系ジョブを抑止するガードを検討。なお `bfTradingApp.service` は現在**未追跡ファイル**なので、コミットするか運用手順として管理するかもあわせて判断が必要。
- **実機確認**: 不要

---

## Medium

### 5. `cancelBuyOrderJob` が現行設定では構造上1件もキャンセルできない 〔新規〕

- **対象**: `go/app/bitflyerApp/cancelBuyOrderJob.go:61-69`
- **分類**: correctness
- **根拠**: `spec.md` スプリント3 / `design-dock.md` §4.4
- **内容**: 出荷設定は `buy_minute_to_expire=10080`(=7日) と `buy_order_cancel_days=7`。`placeBuyOrder` は `expire_date = timestamp + 7日` を記録するため、キャンセル条件「`timestamp <= now-7日`」と、その手前で評価される skip 条件「`expire_date <= now`」が**完全に同じ境界**になる。キャンセル条件を満たすレコードは必ず直前の skip に吸収され、実際にキャンセルされるのは `expire_date` が NULL の旧レコードだけ。
- **影響**: 金銭的な害はない（`expireSweepJob` が翌朝 DB を掃除する）が、ジョブのコメント（:20-21「期限内でも7日を超えて約定しないものはキャンセルする」）と実挙動が食い違い、スロットの早期解放という当初の意図も効いていない。設定値の意味を誤解したまま将来チューニングされる恐れがある。
- **提案**: 意図を活かすなら `buy_order_cancel_days` を `buy_minute_to_expire/1440` より小さく（例: 5）。「期限切れは sweep 任せ」でよいなら、その旨をコメントと design-dock に明記し、設定値の整合注意を残す。
- **実機確認**: 不要

### 6. `placeBuyOrder` が `GetTicker` のエラーを捨てて発注処理を続行する 〔新規・既存コード〕

- **対象**: `go/app/bitflyerApp/placeBuyOrder.go:80, 84, 97, 101`
- **分類**: trading-safety
- **根拠**: ルート `CLAUDE.md` > 開発ルール > エラー処理 / `design-dock.md` §6.1「フェイルセーフの方向」
- **内容**: 発注価格の算出直前で `ticker, _ := apiClient.GetTicker(productCode)` と error を捨てており、失敗時は `ticker` が nil のまま `ticker.Ltp` を参照して panic する。`bitbank.GetBBTicker()` も nil チェックが無い。既存コードだが、本ブランチで変更したファイルであり、開発ルールに正面から反する箇所。
- **影響**: 取引所 API の一時障害でプロセスが落ちる。`Restart=always` で再起動 → **F4（時刻同期）と組み合わさると意図しない発注に発展しうる**。
- **提案**: `err != nil || ticker == nil` なら Slack 通知（productCode / strategy / size 付き）のうえ `return`。bitbank 側も同様。
- **実機確認**: 不要

### 7. reconcileJob の「取引所のみ ACTIVE(SELL)」が、手動売却の開始とともに毎日鳴り続ける 〔新規〕

- **対象**: `go/app/bitflyerApp/reconcileJob.go:241-249, 266-269`
- **分類**: trading-safety（通知設計）
- **根拠**: `design-dock.md` §6「鳴りっぱなしにしない」/ `spec.md`「この65ポジションはユーザーが手動で売却する」
- **内容**: 注文突合は取引所の ACTIVE 一覧と DB の UNFILLED を売買方向ごとに比較するが、`sell_orders` は取引所から同期される仕組みが無い（`syncBuyOrders` は `side=="BUY"` のみ取り込む）。ユーザーが手動保有 65 ポジションを売るために取引所へ SELL 指値を置くと、その注文は約定するまで毎日 `🚨【reconcile】取引所のみ ACTIVE ... /SELL` としてエラー通知される。
- **影響**: 残高突合は手動保有を除外して鳴りっぱなしを回避したのに、注文突合側に同じ穴が残っている。本物の乖離が埋もれる。
- **提案**: (a) 「取引所のみ ACTIVE」の SELL 側はサマリ掲載に留めエラー通知しない / (b) 許容件数を設定値で持つ / (c) sell 側も取り込む。判断はユーザー。
- **実機確認**: 不要

### 8. `expireSweepJob` 方式Aに「遡り限界より古いものは判定保留」ガードが無い 〔既知〕

- **対象**: `go/app/bitflyerApp/expireSweepJob.go:202-222`
- **分類**: trading-safety
- **根拠**: `spec.md` リスク表「誤 CANCELLED」「遡り不足による誤判定」
- **内容**: 方式B（`:264`）には `index.oldestCompleted` との比較による判定保留があるが、方式A（`expire_date` 経過）には無い。COMPLETED 一覧は最大 500件×10ページ = 5000件で頭打ちのため、取引量が増えて遡り範囲が短くなると、範囲外で約定した注文を「一覧に無い＝失効」と誤判定して CANCELLED に落としうる。
- **影響**: 誤 CANCELLED になった買い注文は `placeSellOrder` の対象から外れ、保有した現物が DB 上追跡不能（裸の保有）になる。現状は `oldestCompleted` が BTC=2022-06 / ETH=2023-10 と十分古く実害なし（S3 の Evaluator が本番データで確認済み）。
- **提案**: 方式Aにも `oldestCompleted` 比較を入れ、遡り限界より古いレコードは pending として通知に留める（方式Bと共通ヘルパー化できる）。
- **実機確認**: 不要

### 9. models 層のエラーが Slack に届かない（開発ルール未充足） 〔新規〕

- **対象**: `go/models/events.go:325`（CheckFilledBuyOrders）, `:413`（SyncBuyOrders の INSERT 失敗）, `:423`（UpdateOrderExpireDate 失敗）
- **分類**: slack-notification
- **根拠**: ルート `CLAUDE.md` / `go/CLAUDE.md` > 開発ルール「エラーは必ず Slack に通知」
- **内容**: 本ブランチで変更した3経路がログ出力のみ。
  - `SyncBuyOrders` の INSERT 失敗 → ボット発注の記録が落ちたまま無言で進む。
  - `UpdateOrderExpireDate` 失敗 → `expire_date` が入らず失効検出が効かなくなるが誰も気づけない。
  - `CheckFilledBuyOrders` の Scan 失敗 → nil を返すため `placeSellOrder` が「売る対象なし」と解釈し、**約定済み買い注文への売り注文発注が無言でスキップ**される。
- **影響**: 本機能の目的である「無言の停止が起きない」に反する経路が残る。
- **提案**: models 側は error を返し、`app/bitflyerApp` 側で通知する形に寄せる。最小対応は `CheckFilledBuyOrders` / `SyncBuyOrders` の error 返却化。
- **実機確認**: 不要

### 10. `cancelBuyOrderJob` が `Timestamp` ゼロ値をキャンセル側に倒す 〔既知〕

- **対象**: `go/app/bitflyerApp/cancelBuyOrderJob.go:67`（+ `go/models/orderLifecycle.go:214-216`）
- **分類**: trading-safety
- **根拠**: `design-dock.md` §6.1「フェイルセーフの方向」
- **内容**: `order.Timestamp.After(threshold)` は、`Timestamp` がゼロ値（DB 値のパース失敗 / NULL）のとき「十分古い」と解釈されキャンセルへ進む。`scanOrderRecords` の `toUTCTime` は変換失敗を黙ってゼロ値のまま通すため、パース失敗が実在する注文のキャンセルに直結する。方式B（sweep）は同じケースを「判定保留」にしており非対称。
- **影響**: 想定より早く買い注文がキャンセルされる（金銭的損失ではないが機会損失）。
- **提案**: `Timestamp.IsZero()` なら `continue` し、件数を Slack 通知。あわせて `toUTCTime` の失敗を呼び出し側に伝える。
- **実機確認**: 不要

---

## Low

### 11. `GetUnfilledBuyOrderRecords` / `GetSellOrdersToRollover` が exchange でフィルタしていない 〔既知〕
- **対象**: `go/models/orderLifecycle.go:325, 490`
- **内容/影響**: `buy_orders` は OKEX サービスと共用テーブル（`cmds/main.go` で `okex.TableName = "buy_orders"`）。OKEX 由来の UNFILLED 行が残っていると、`cancelBuyOrderJob` が Bitflyer の CancelOrder を OKEX の product_code で叩き、rollover は存在しない通貨ペアで一覧取得を試みる。誤ったキャンセルにはならない（acceptance_id が一致しない）が、毎日の Slack エラーが恒常化する。旧実装も同様で新規劣化ではない。
- **提案**: WHERE に `exchange = 'bitflyer'` を追加。

### 12. `remainingSize()` の浮動小数誤差で正当なローリングがスキップされる 〔既知〕
- **対象**: `go/app/bitflyerApp/rolloverSellOrderJob.go:236-241, 322`
- **内容/影響**: `0.03 - 0.02 = 0.009999999999999998` が ETH 最小単位 0.01 を僅差で下回り、「残数量が最小取引単位未満」として毎日スキップされ続ける。フェイルセーフ側の誤りだが、放置すると期限切れ→裸の保有になる。影響するのは `outstanding_size == 0`（取引所側で既に CANCELED）の経路のみ。
- **提案**: 8桁丸め（`math.Round(x*1e8)/1e8`）またはイプシロン比較。

### 13. 既に CANCELED 済みの注文が最小単位未満のときの通知文が実態と食い違う 〔既知〕
- **対象**: `go/app/bitflyerApp/rolloverSellOrderJob.go:322-331`
- **内容/影響**: 通知文が「注文はそのまま残します」固定。`default` 分岐（取引所側で既に CANCELED）から入った場合、注文はもう存在せず実態は裸の保有。ユーザーが「まだ売り注文が生きている」と誤認する。
- **提案**: `snapshot.state` / `found` で文言を分岐する。

### 14. `service.go` の `"22:45"` がハードコード 〔既知〕
- **対象**: `go/app/bitflyerApp/service.go:274`（`"01:20"` も同様）
- **内容/影響**: 他の日次ジョブは `trigger_time_01`〜`07` で config 化されており不整合。時刻変更に再ビルドが必要。
- **提案**: `trigger_time_08`（既定 22:45）として config 化。

### 15. `GetBalance` に対象通貨が無い場合、map のゼロ値0を実残高として誤判定しうる 〔既知〕
- **対象**: `go/app/bitflyerApp/reconcileJob.go:302-312`
- **内容/影響**: レスポンスに BTC/ETH が含まれないと「実残高 0」として扱われ、🚨不足アラートが誤発報。方向は安全側だが原因不明の誤アラートになる。
- **提案**: `amount, ok := exchangeAmounts[...]` で存在確認し、無ければ突合をスキップして通知。

### 16. `[ROLLOVER_PENDING]` が「DBのみ」通知と残留通知で二重に鳴る 〔既知〕
- **対象**: `go/app/bitflyerApp/reconcileJob.go:237-239, 402-427`
- **内容/影響**: 同じ事象で毎日2通のエラー通知。
- **提案**: 注文突合の `DBOnly` から `[ROLLOVER_PENDING]` 付きを除外し、専用通知に一本化。

### 17. サマリのラベル `余剰(DB未追跡分。アラート対象外)` が修正後の実態と食い違う 〔既知〕
- **対象**: `go/app/bitflyerApp/reconcileJob.go:462`
- **内容/影響**: Sprint 7 の修正で手動保有を `AlertExpected` から除外したため、余剰の主因は「手動保有 130 レコード」であって「DB 未追跡分」ではない。`untracked_holding_*` の設定漏れと誤解される。
- **提案**: `余剰(手動保有・未追跡分を含む。アラート対象外)` 等に修正。

### 18. sell 超過警告の未集約（恒常的失敗時に通知が件数分飛ぶ） 〔既知・S5→S7 で未消化〕
- **対象**: `go/app/bitflyerApp/placeBuyOrder.go:69-74`, `go/app/bitflyerApp/reconcileJob.go:507-510`
- **内容/影響**: sell 超過は解消まで数週間続きうるのに、buy ジョブごと（最大4回/日）＋ reconcile 1通で毎日5通の 🚨 が流れる。`placeSellOrder` / rollover の `continue` 化により恒常的失敗時も件数ぶん通知が飛ぶ。本物の通知が埋もれる。
- **提案**: sell 超過の通知は reconcileJob の日次1通に一本化。ループ内の失敗はジョブ末尾サマリ＋最初のN件のみ個別通知。

### 19. `IsBotStrategy(0)` が true になる（`Stg0BtcLtp3low7 = iota` = 0） 〔新規〕
- **対象**: `go/enums/strategy.go:45, 125`, `go/models/orderLifecycle.go:804-819`
- **内容/影響**: `GetRecentBuyOrders` は strategy を `sql.NullInt64` で読み、NULL / 読み取り不能なら `int(0)` を入れる。0 は `botStrategies` に含まれるため「ボット発注」と数えられ、**発注ゼロアラートを取りこぼす**可能性がある。
- **提案**: `strategy.Valid == false` のとき `enums.StrategyUnknown` を入れる。

### 20. `http.Client` にタイムアウトが無い 〔新規・既存コード〕
- **対象**: `go/bitflyer/bitflyer.go:30`
- **内容/影響**: rollover は「キャンセル成功 → 再発注」の間に HTTP 往復があり、ここで無期限ブロックすると裸の保有のまま復帰しない。`gracefulShutdown` は最大5分待って `os.Exit(0)` するため、ハングしたジョブごとプロセスが落ちる。
- **提案**: `&http.Client{Timeout: 30 * time.Second}`。

### 21. `bitflyer.Ticker.DateTime()` が常にパース失敗 〔既知・main にも存在する既存バグ・参照0件〕
- **対象**: `go/bitflyer/bitflyer.go:323-329`
- **内容/影響**: RFC3339 でパースするが Bitflyer の timestamp はサフィックス無し形式。常に失敗しゼロ値を返しつつエラーログを吐く。参照0件のため実害なし。
- **提案**: `utils.ParseBitflyerTime` に差し替えるか、未使用なので削除。

### 22. `MarkOrderCancelledWithRemark` だけ `RowsAffected()` エラー時に成功扱い 〔新規〕
- **対象**: `go/models/orderLifecycle.go:375-379`
- **内容/影響**: `RolloverSellOrder`（:576-581）と `UpdateOrderSizeWithRemark`（:656-659）は同条件で失敗側に倒しているのに、ここだけ nil を返す。sweep が「CANCELLED にできた」と誤って集計・通知しうる。
- **提案**: `rowsErr != nil` をエラーとして返す（sweep は失敗通知し翌日再試行）。

### 23. design-dock §3.4 の DDL と実マイグレーションが食い違う 〔新規・ドキュメント〕
- **対象**: `db/crypto-trading-db-postgres/migrations/20260906000000_add_expire_date.sql:17` vs `design-dock.md` §3.4
- **内容/影響**: 設計書では `ALTER TABLE buy_orders ALTER COLUMN strategy TYPE INTEGER;` が「任意」としてコメントアウトされているが、実ファイルでは有効な実行文（本番適用済み）。実装が悪いのではなく設計書が実態に追随していない。将来この設計書を根拠に別環境へ適用すると差異が生じる。
- **提案**: design-dock §3.4 の DDL ブロックを実ファイルに合わせて更新し「適用済み」と注記。

### 24. `StrategyLTP95` のコメント「廃止」と実装の食い違い 〔既知〕
- **対象**: `go/enums/strategy.go:9-10`, `go/app/bitflyerApp/service.go:149-154`
- **内容/影響**: `is_test=true` のとき動く `buyingBTCJobLTP95TEST` / `buyingETHJobLTP95TEST` が現に使用している。テストモードで −5% の深指値が出ることに気づきにくい。
- **提案**: コメントを実態に合わせるか、テストジョブを `StrategyLTP97` に寄せる。

### 25. `rolloverSellOrderJob.go` の冒頭コメントが旧処理順序のまま 〔既知〕
- **対象**: `go/app/bitflyerApp/rolloverSellOrderJob.go:24-34`
- **内容/影響**: Sprint 4 修正の要である「キャンセル前の個別照会（部分約定の検出）」が記載されていない。このコメントはジョブの安全性の根拠そのものなので、ずれていると誤読を招く。
- **提案**: ①約定確認 →②キャンセル前照会（数量取得・DBのsize補正）→③CancelOrder →④キャンセル後照会 →⑤再発注 →⑥単一トランザクション、に更新。

### 26. `utils.BfCancelCriteria` が未使用のまま残っている 〔新規〕
- **対象**: `go/utils/util.go:12`
- **内容/影響**: 実コードからの参照が0件（`cancelBuyOrderJob.go` のコメント内の言及のみ）。`OkexCancelCriteria` は使用中。
- **提案**: 削除、または「未使用。cancelBuyOrderJob は `config.BFBuyOrderCancelDays` を使う」とコメント。

### 27. 部分約定時の損益過小計上（恒久対応が未実施） 〔既知・申し送り〕
- **対象**: `go/models/orderLifecycle.go:529-612`, `go/models/events.go:483-522`（`getResultsPostgres`）
- **内容/影響**: 部分約定した売り注文をローリングすると旧レコードが CANCELLED になり、既約定ぶんの利益が損益集計（`a.parentid = b.order_id AND a.status='FILLED'`）から外れる（S4 実測で −4,919.58 の過小計上）。買いコスト二重計上を避ける安全側の選択として妥当と評価済みだが、恒久対応（買い注文側の数量按分）は未実施。損益レポートが実態より控えめに出続ける。
- **提案**: 別課題化するか今回対応するかの判断。対応する場合は `buy_orders` 側にも按分用の数量を持たせる設計変更になりスコープは小さくない。
- **補足**: なお `getResultsPostgres()` の `a.parentid = b.order_id` 結合自体は本ブランチで破壊されていないことを確認済み（`RolloverSellOrder` が `parentid` を引き継ぎ、旧行は CANCELLED になるため二重計上なし。`make test` の損益テストも PASS）。

### 28. `GetUnfilledOrderIDs` の limit 到達がローカルログ止まり 〔新規〕
- **対象**: `go/models/orderLifecycle.go:718-720`, `go/app/bitflyerApp/reconcileJob.go:226-231`
- **内容/影響**: 500件の打ち切りが `[ERROR]` ログのみで reconcileJob に伝わらず、突合結果が実態とずれたまま「乖離なし」と通知されうる。「無言の停止を防ぐ」という本ジョブの目的に照らすと、この打ち切りこそ通知すべき事象。
- **提案**: 戻り値に truncated フラグを追加し、reconcileJob から Slack 通知。

### 29. `GetBalance` / `GetTicker` がレスポンス全文をログに残す 〔新規・既存コード〕
- **対象**: `go/bitflyer/bitflyer.go:100, 334`
- **内容/影響**: `trading.log` に口座残高が平文で蓄積する。今回 `reconcileJob` が毎日 `GetBalance` を呼ぶため記録頻度が上がる。ログは `StandardOutput=append` で無期限に追記される。
- **提案**: レスポンス全文ログを削除するか通貨コードのみに絞る。`clear-trading-log.sh` によるローテーション運用とあわせて判断。

---

## 補足: 良かった点（指摘ではありません）

- `CancelOrder()` のレスポンス検証（2xx + 空ボディのみ成功／パース不能は失敗側）が実機 E2E で裏取りされており、フェイルセーフの方向が正しい。
- ローリングの処理順序（約定確認 → キャンセル前照会 → キャンセル → キャンセル後照会 → 再発注 → 単一トランザクション）は二重売り防止の観点で妥当。`RolloverSellOrder` が `status='UNFILLED'` 条件付き UPDATE ＋ `RowsAffected==0` でロールバックする作りも良い。
- `OrderTable` 限定型によるテーブル名連結の安全化、`RemarkManualHold` / `RemarkRolloverPending` の定数化（日本語散文マッチの排除）は設計どおり実装されている。
- 手動保有 130 レコードは `status='UNFILLED'` 条件により全ジョブから構造的に除外されており、`GetExpectedHoldings()` でも `AlertTarget()`（Bot+Naked）から除外されている。
- 新規ジョブがすべて1ジョブ1ファイルに分離され、`service.go` の `goto` を使ったインライン実装も解消された。

---

## ご判断のお願い

**上記 29 件の各指摘について「直す」／「不要」をご判断ください。** 「直す」と判断されたものだけを Generator に修正フィードバックとして渡します。

判断の目安として、Reviewer から見た優先順位は次のとおりです（採否の決定はユーザーが行います）:

1. **F1**（認証情報の平文コミット）— コードとは独立に、まず対処が要る事項
2. **F2**（`[ROLLOVER_PENDING]` の恒久滞留）— 本機能が解消しようとした障害の再発経路
3. **F3**（レスポンス消失後の2本目発注）— 手動保有の現物があるため実際に約定しうる
4. **F4**（時刻同期）— 既に事故が発生している
5. **F5 / F6**（no-op ジョブ・ticker の nil 参照）— 挙動と設計意図の食い違い
