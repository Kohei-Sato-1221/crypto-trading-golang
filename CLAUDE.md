# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## プロジェクト概要

Goで実装された暗号資産自動売買ボット。Bitflyer（メイン）およびOKEX取引所でBTC_JPY/ETH_JPYの買い注文をスケジュール実行し、約定後に+1.5%の売り注文を自動発注する。現物取引のみ対応。

## ディレクトリ構成

- `go/` — Goアプリケーション本体（売買ロジック、API連携、モデル等）
- `terraform/` — AWSインフラ管理（VPC、RDS、EventBridge）
- `db/` — Atlas管理のDBマイグレーション。スキーマ定義は `db/crypto-trading-db/atlas/schema.hcl`
- `lambda/aws-scheduler/` — RDS起動・停止用のLambda関数

## ビルド・実行コマンド（ルートMakefile）

```bash
make run                      # トレーディングアプリ実行（go run）
make run-binary               # ビルド済みバイナリで実行
make run-save-price-history   # 価格履歴記録ジョブ単体実行
make run-send-results         # 日次損益レポートジョブ単体実行
make build                    # バイナリビルド（Linux amd64）→ go/bfTradingApp
```

## DBマイグレーション（db/ディレクトリで実行）

```bash
cd db
make up                       # Atlas導入 + マイグレーション適用
make atlas-status             # マイグレーション状態確認
make atlas-apply              # 未適用マイグレーション適用
make atlas-diff n=<timestamp>_<説明>  # schema.hclの差分からマイグレーション生成
make atlas-hash               # チェックサム更新（手動編集時に必要）
make reset-db                 # ⚠️ DB全削除・再作成
```

DBスキーマ変更の手順: `schema.hcl`を編集 → `make atlas-diff` → 生成SQLを確認 → `make atlas-apply`

## 開発ルール

- **ジョブ関数**: `bitflyerApp/`配下でジョブごとに別ファイルに分離すること
- **エラー処理**: エラーは必ずSlackに通知（`slackClient.PostMessage()`）。OrderID・Price・Size・Strategy等のコンテキストを含めること
- **タイムゾーン**: スケジューラの時刻はシステムTZに依存する。JST対象はUTCに変換（JST = UTC+9）するか、ジョブ内でJSTを明示チェック
- **機密情報**: `private_config.ini`、`terraform.tfvars`は絶対にコミットしないこと
- **取引所APIの実行**: 参照系は自由に叩いてよいが、発注・キャンセル等の更新系は事前にユーザーの許可を取ること（詳細は「取引所APIの動作確認ルール」を参照）

## 取引所APIの動作確認ルール

実装中および実装内容の検証時に、**実際の取引所API（Bitflyer / OKEX / bitbank）を叩いて挙動を確かめることは推奨する。** ドキュメントや推測だけで済ませず、実レスポンスで裏を取ること。ただしAPIの種別によって扱いを分ける。

### 参照系（読み取り）— 確認不要。自由に実行してよい

口座の状態を変化させないAPIは、**ユーザーへの確認なしに積極的に実行してよい**。

- 例: 板情報 `/v1/board`、Ticker `/v1/ticker`、約定履歴 `/v1/executions`、残高 `/v1/me/getbalance`、注文一覧 `/v1/me/getchildorders`、約定一覧 `/v1/me/getexecutions`
- 注意点:
  - レート制限に配慮し、必要最小限の回数に留める
  - レスポンスに含まれる残高・APIキー等の機密情報は、ログや成果物ファイルにそのまま残さない

### 更新系（発注・キャンセル等の書き込み）— 実行前に必ずユーザーの許可を取る

口座の状態を変化させるAPIは、**必要に応じて実行してよいが、無断で実行してはならない。実行前に必ずユーザーに相談し、明示的な許可を得ること。**

- 例: 新規注文 `/v1/me/sendchildorder`、注文キャンセル `/v1/me/cancelchildorder` / `/v1/me/cancelallchildorders`、親注文 `/v1/me/sendparentorder`

**相談時に必ず提示する内容:**

1. 叩くエンドポイントと目的（何を確認したいのか）
2. リクエストパラメータの実値（取引所・通貨ペア・売買方向・数量・価格・注文種別）
3. 想定される影響（約定する可能性、拘束される金額の目安）
4. リスクを下げる工夫（約定しにくい指値にする、最小取引単位にする 等）

そのうえで「この内容でAPIを投げてよいか」を尋ね、**ユーザーの明示的な許可を得てから実行する。**

**実行後の扱い:**

- レスポンス全文と OrderID をユーザーに報告する
- **発注した注文の後始末（動作確認後のキャンセル等）はユーザーがコンソール上で行う。** エージェント側で勝手にキャンセルAPIを叩いて片付けない（キャンセルAPI自体が、別途許可の必要な更新系操作である）
- 許可が得られなかった場合は実APIを叩かず、コード追跡や参照系APIで代替し、検証できなかった点を明記する

### ボット全体の起動について

`make run` / `make run-binary` / 発注系ジョブの直接実行は、**スケジューラが自動的に発注しうる**ため、上記の「個別に許可を取ってAPIを1回叩く」とは別扱いとする。検証目的で勝手に起動しない。必要な場合は、何がいつ発注されうるかを説明したうえでユーザーの許可を得る。

## 設定ファイル

- `go/config.ini` — アプリ設定（取引所、通貨ペア、予算基準、取引量、スケジュール）
- `go/private_config.ini` — APIキー、DB接続情報、Slack webhook（`[sample]private_config.ini`からコピー）
- `db/envs/.db.env` — Atlasマイグレーション用DB接続情報

## テスト

`go/tests/` に docker-compose の PostgreSQL を使った統合テストが少数ある。

```bash
make test           # PostgreSQL起動 → go test ./tests/ -v → 停止
make test-keep-db   # DBを起動したまま繰り返し実行
```

**テスト方針**: ユニットテストは最低限に留める。カバレッジのために機械的にテストを増やしたり、テストのために本体の設計を歪めたりしない。壊れても気づきにくい箇所（DB入出力、損益計算、価格計算）に絞って追加する。テストが無いこと自体は不具合として扱わない。
---

## 売り注文の部分約定が起きたときの対応

> Slack通知の「手順はルート CLAUDE.md の『売り注文の部分約定が起きたときの対応』を参照してください」から辿り着く先。
> **この見出しは `go/app/bitflyerApp/rolloverSellOrderJob.go` の定数 `partialFillDocSection` と一致させること。** 片方だけ変えると通知から手順に辿り着けなくなる。

### なぜシステムが対応していないのか

損益レポートは `go/models/events.go` の `getResultsPostgres()` / `getResultsMySQL()` が集計している。中核は次の1行で、**1つの買い注文に売り注文が1本だけぶら下がる**ことを前提にしている。

```sql
sum((a.price * a.size) - (b.price * b.size))
-- a = sell_orders (status='FILLED'), b = buy_orders, 結合条件 a.parentid = b.order_id
```

売り注文が部分約定するとこの前提が崩れる。ローリング（`rolloverSellOrderJob`）は残数量で再発注し、旧レコードを `CANCELLED` にするため、**既に約定した数量ぶんの売却額はどこにも残らない**。

例: 買い 0.03 @ `P_b` → 売り注文が 0.01 だけ @ `P_s1` で部分約定 → 残り 0.02 が後日 @ `P_s2` で約定

| | 計算式 |
|---|---|
| 真の損益 | `(P_s1 − P_b) × 0.01 + (P_s2 − P_b) × 0.02` |
| 現在の記録 | `(P_s2 × 0.02) − (P_b × 0.03)` |
| **不足額** | **`P_s1 × 0.01`**（＝部分約定ぶんの売却総額がまるごと欠落する） |

ずれは常に**過小**方向（利益を過大に見せない安全側）で実損は無い。恒久対応（買い注文側の数量按分）は買い注文のデータモデルに手を入れる設計変更で規模が他と桁違いであり、部分約定の発生実績もまだ1件も無いため、**スコープ外と判断した**（レビュー指摘 F27）。発生したときは以下の手順で手作業で補う。

### パターン①とパターン②

`rolloverSellOrderJob` は部分約定を検出したとき、残数量が最小取引単位（BTC_JPY: 0.001 / ETH_JPY: 0.01）以上かどうかで分岐する。どちらもSlackに通知される。

| | 残数量 | ボットの挙動 | 現物の行方 | 対応 |
|---|---|---|---|---|
| **パターン①** | 最小取引単位**以上** | DBの `size` を残数量へ補正 → キャンセル → 残数量で再発注 | 売り注文は継続する | 損益の補正のみ（下記手順） |
| **パターン②** | 最小取引単位**未満** | キャンセルも再発注もせず中止（キャンセルすると再発注できず裸の保有になるため） | 取引所の注文をそのまま残す → **期限（最長30日）で失効** → 売り注文の無い「裸の保有」 | 損益の補正に加えて**取引所側の処理が必要**（後述） |

### パターン①の手動対応手順

#### 使う値（すべてSlack通知に載っている）

| プレースホルダ | Slack通知の項目 | 例 |
|---|---|---|
| `:buy_order_id` | 親買い注文ID | `JRF20260801-101010-000001` |
| `:sell_order_id` | 売り注文ID | `JRF20260907-010131-018801` |
| `:product_code` | product_code | `ETH_JPY` |
| `:avg_price` | 平均約定価格(average_price) | `511000` |
| `:executed_size` | 約定数量 | `0.01` |
| `:remaining_size` | 残数量 | `0.02` |

`:avg_price` は**キャンセル後に取引所APIから二度と取得できない**（キャンセルした注文はAPIから即座に消える。実測確認済み）。Slack通知が唯一の記録なので、通知を消さずに残すこと。

#### ⚠️ やってはいけないこと

**「約定ぶんの売り注文を `sell_orders` に FILLED で1行 INSERT するだけ」では直らない。かえって悪化する。**
同一 `parentid` に FILLED の売りが2行並ぶと、集計式の `b.price * b.size`（買いコスト**全量**）が行ごとに引かれ、買いコストが二重計上される。ローカル検証では対応前 `-4,747.87` が `-14,626.99` へ悪化した（正しい値は `+356.51`）。

**買い注文側も同じ比率で分割する**必要がある。

#### 手順（PostgreSQL）

`buy_orders.order_id` / `sell_orders.order_id` には UNIQUE 制約（`orderId` インデックス）があるため、派生レコードには `-PARTIAL` を付けた別IDを使う。`varchar(50)` に対して Bitflyer の注文IDは25文字なので収まる。同じ注文で2回目の部分約定が起きた場合は `-PARTIAL2` のように連番にする。

```sql
BEGIN;

-- 手順1: 既存の買い注文を「残数量ぶん」へ按分する（0.03 -> 0.02）
UPDATE buy_orders
   SET size    = :remaining_size,
       remarks = COALESCE(remarks, '') || ' / partial fill manual fix: size -> ' || :remaining_size
 WHERE order_id = :buy_order_id;

-- 手順2: 部分約定ぶんに対応する買い注文を派生IDで作る（price は元の買値のまま。size だけ約定数量）
--        product_code / side / price / exchange / status / strategy / timestamp は元レコードから引き継ぐ
INSERT INTO buy_orders (order_id, product_code, side, price, size, exchange, status, strategy, remarks, timestamp)
SELECT :buy_order_id || '-PARTIAL',
       product_code, side, price, :executed_size, exchange, status, strategy,
       COALESCE(remarks, '') || ' / partial fill manual fix: split from ' || order_id,
       timestamp
  FROM buy_orders
 WHERE order_id = :buy_order_id;

-- 手順3: 部分約定ぶんの売り注文を、手順2で作った買い注文に紐づけて作る
--        price は必ず「平均約定価格(average_price)」であって指値ではない
--        updatetime が日次損益レポートの計上日になる（通知を受け取った日でよい）
INSERT INTO sell_orders (parentid, order_id, product_code, side, price, size, exchange, status, remarks, updatetime)
VALUES (:buy_order_id || '-PARTIAL',
        :sell_order_id || '-PARTIAL',
        :product_code, 'SELL', :avg_price, :executed_size, 'bitflyer', 'FILLED',
        'partial fill manual fix', NOW() AT TIME ZONE 'UTC');

-- ここで下記の検証クエリを流し、問題なければ COMMIT
COMMIT;
```

- 手順2の `status` は元レコードから引き継ぐので `FILLED(SELL ORDER PLACED)` になる。この値であることが重要で、`FILLED` にすると `placeSellOrder` が新しい売り注文を出してしまう
- 手順1・2で買い数量の**合計は変わらない**（0.02 + 0.01 = 0.03）。分割であって水増しではない
- 手順3の売りは `FILLED` なので、`rolloverSellOrderJob` / `filledCheckJob` / `expireSweepJob`（いずれも `UNFILLED` のみを対象にする）には拾われない。取引所に存在しない `-PARTIAL` IDへAPIを叩きにいくことはない

#### 検証（COMMIT 前に流す）

```sql
-- ① 買い数量の合計が元の数量と一致すること（分割であって水増しではない）
SELECT SUM(size) FROM buy_orders WHERE order_id IN (:buy_order_id, :buy_order_id || '-PARTIAL');
--    -> 元の size（例 0.03）と一致すること

-- ② 1つの買い注文に FILLED の売りが2行以上ぶら下がっていないこと（買いコストの二重計上の検知）
SELECT parentid, COUNT(*) FROM sell_orders WHERE status = 'FILLED' GROUP BY parentid HAVING COUNT(*) > 1;
--    -> 0件であること

-- ③ 当該ポジションの損益が期待値と一致すること
SELECT SUM((a.price * a.size) - (b.price * b.size)) AS profit
  FROM sell_orders a, buy_orders b
 WHERE a.parentid = b.order_id AND a.status = 'FILLED'
   AND b.order_id IN (:buy_order_id, :buy_order_id || '-PARTIAL');
--    -> (P_s1 - P_b) * 約定数量 + (P_s2 - P_b) * 残数量 と一致すること
--       （日次レポートの表示値はこれに手数料率 0.9989 を掛けたもの）
```

COMMIT 後は `make run-send-results` を使わず（ボットの起動系は不用意に叩かない）、上記③のクエリで確認すれば十分。

残高照合（`GetExpectedHoldings`）への影響も無い。派生した買い注文は `FILLED(SELL ORDER PLACED)` かつ `FILLED` の売りが紐づくため、`bot` にも `naked`（裸の保有）にも計上されない。

### パターン②の対応方針

**DB操作だけでは解決しない。** 残数量が最小取引単位未満のため、その端数はボットからも取引所からも売れない。

1. **取引所側**: 端数は他のポジションと合算するか、成行で処理するか、そのまま保有するかを人が判断する。ボットは以後この端数に触らない
2. **DB側**: 約定した数量ぶんの損益は、上記パターン①と同じ手順1〜3で補正できる（`:remaining_size` には端数をそのまま入れる）
3. パターン②で取引所の注文が失効した場合、`expireSweepJob` がDBの売り注文を後始末する。その後この買い注文は「裸の保有」として残高照合に計上される（想定どおりの挙動）

### 検証状況

この手順は `go/tests/partial_fill_manual_fix_test.go` でローカルの docker PostgreSQL に対して検証済み（`make test`）。買い 0.03 @500,000 / 部分約定 0.01 @511,000 / 残り 0.02 @512,345 のシナリオで:

| | `GetResults()` の Total |
|---|---|
| 手動対応なし（現状） | `-4,747.87` |
| 売り注文だけ INSERT（アンチパターン） | `-14,626.99` |
| **上記手順で対応** | **`+356.51`**（真の損益と一致） |

**手順を変更するときは `go/tests/partial_fill_manual_fix_test.go` も併せて更新すること。**

---

## ハーネスエンジニアリング

> **発動条件**: ユーザーが「ハーネスエンジニアリングを実行して」と言った場合のみ、このセクションのルールに従って動作する。それ以外の通常の会話では、このセクションは無視すること。

### このリポジトリ固有の前提（全エージェント共通）

- **バックエンドのみのアプリケーション**。UI・フロントエンドは存在しない。ブラウザ操作による評価（Playwright 等）は一切行わない。
- **ユニットテストは最低限**。テストの追加・網羅はスプリントの合格条件にしない。動作確認は「ビルド・静的検査・既存統合テスト・参照系APIの実行・コード追跡」で行う（→ Evaluator）。
- **静的検査にはベースラインがある**。既存コードには変更前から `go vet` / `gofmt` の指摘が存在する（`go/bitbank/bitbank.go`、`go/app/bitflyerApp/filledCheckJob.go`、`okex/okex.go`）。**今回の差分で変更・追加したファイルの指摘のみ**を合否の対象とし、既存分は指示がない限り触らない。
- **取引所APIの実行は「取引所APIの動作確認ルール」に従う**。参照系は確認不要で積極的に叩いてよい。更新系（発注・キャンセル）はユーザーの許可が必要（→ 後述の「更新系APIの承認フロー」）。`make run` / `make run-binary` によるボット全体の起動は行わない。
- **`make apply` / `make atlas-apply` などインフラ・DBを変更するコマンドは、エージェントが自律的に実行しない**。実行はユーザーに依頼する（`make plan` / `make atlas-diff` などの参照系は可）。

### 更新系APIの承認フロー

サブエージェント（Generator / Evaluator / Reviewer）は**ユーザーと直接やり取りできない**。そのため、発注・キャンセル等の更新系APIで動作確認したい場合は次の導線を取る。

```
サブエージェント: 実行せずに state ファイルの pending_user_approval へ記載して終了
    │  （エンドポイント / 目的 / パラメータ実値 / 想定される影響 / リスク低減策）
    ▼
メイン会話: 内容をユーザーに提示し、「このAPIを投げてよいか」を確認する
    │
    ├─ 許可あり → メイン会話が実行、または「実行許可済み」としてプロンプトに含めエージェントを再起動
    │              → レスポンス全文と OrderID をユーザーに報告
    │              → 発注した注文の後始末（キャンセル）はユーザーがコンソール上で行う
    │
    └─ 許可なし → 実APIを叩かず、コード追跡・参照系APIで代替。未確認である旨を明記して続行
```

**実機未確認であることだけを理由に、スプリントを不合格にしない。**

### 概要

4つのサブエージェント（Planner → Generator → Evaluator → Reviewer）を自律的に連携させ、ユーザーの短いプロンプトから機能実装を完了するパイプライン。

```
ユーザー: 「ハーネスエンジニアリングを実行して」+ 要件（1〜4行）
    │
    ▼
Phase 1: Planner（仕様書・技術設計書生成）
    │  → docs/features/{feature-name}/spec.md, docs/features/{feature-name}/design-dock.md に出力
    │  → ユーザーに仕様書・デザインドックを提示して承認を得る【必須】
    │
    ▼
Phase 2: Sprint Loop（全スプリント完了まで繰り返し）
    │
    ├─► Generator（スプリント N を実装）
    │     → docs/features/{feature-name}/state/current_sprint.json に出力
    │
    ├─► Evaluator（スプリント N を検証・評価）
    │     → docs/features/{feature-name}/state/evaluation_result.json に出力
    │
    └─► 判定
          ├─ pass → 次のスプリントへ
          └─ fail → Generator に修正フィードバックを渡して再実行（最大3回）
                     3回失敗 → ユーザーに報告して判断を仰ぐ
    │
    ▼
Phase 3: Code Review（全スプリント完了後に1回）
    │
    ├─► Reviewer（変更全体をレビュー＋各ディレクトリ CLAUDE.md の開発ルール遵守チェック）
    │     → docs/features/{feature-name}/state/review_result.json
    │     → docs/features/{feature-name}/review_report.md に出力
    │
    ├─► ユーザーに指摘一覧を提示【必須ヒューマン・イン・ザ・ループ】
    │     → ユーザーが各指摘の採否（直す／不要）を判断
    │
    └─► 「直す」と判断された指摘のみ Generator に渡して修正
          → 修正後に Evaluator で回帰確認
          → 指摘 0 件 or 全て「不要」判断なら修正はスキップして完了へ
```

### エージェント定義

| エージェント | ファイル | 役割 |
|---|---|---|
| Planner | `.claude/agents/planner.md` | 要件を製品仕様書に展開。技術詳細には踏み込まず「何を作るか」に集中 |
| Generator | `.claude/agents/generator.md` | 仕様書のスプリントを1つずつ実装。自己評価後に状態ファイルを更新 |
| Evaluator | `.claude/agents/evaluator.md` | ビルド・`go vet`・既存統合テスト・参照系API・コード追跡で検証。4基準×閾値で合格/不合格を判定 |
| Reviewer | `.claude/agents/reviewer.md` | 全スプリント完了後に変更全体をコードレビュー。汎用観点＋各ディレクトリ `CLAUDE.md` の開発ルール準拠をチェックし、指摘一覧を出力（採否判断は行わない） |

### 状態管理

エージェント間のデータは `docs/features/{feature-name}/state` で受け渡す。

| ファイル | 書き込み元 | 内容 |
|---|---|---|
| `docs/features/{feature-name}/spec.md`, `docs/features/{feature-name}/design-dock.md` | Planner | 製品仕様書（機能一覧・スプリント計画・成功基準）・デザインドック（技術設計書） |
| `docs/features/{feature-name}/state/current_sprint.json` | Generator | 現在のスプリント状態（実装内容・自己評価・変更ファイル・更新系API承認待ち） |
| `docs/features/{feature-name}/state/evaluation_result.json` | Evaluator | 評価結果（スコア・検証結果・バグ・修正指示・更新系API承認待ち） |
| `docs/features/{feature-name}/state/review_result.json` | Reviewer | コードレビュー指摘一覧（構造化。severity・分類・根拠・提案・decision） |
| `docs/features/{feature-name}/review_report.md` | Reviewer | コードレビュー指摘一覧（ユーザーが採否判断するための人間向け一覧） |
| `docs/features/{feature-name}/iteration_log.md` | メイン会話 | 全スプリントの実行履歴（合格/不合格・試行回数） |

### 実行ルール

#### Phase 1: Planner 起動

1. Agent toolで `.claude/agents/planner.md` を起動する
2. プロンプトにユーザーの要件を含める
3. Plannerは `docs/features/{feature-name}/spec.md`, `docs/features/{feature-name}/design-dock.md` に仕様書・デザインドックを書き出す
4. **仕様書・デザインドックをユーザーに提示し、承認を得てからPhase 2に進む**（承認なしに実装を開始しない）
5. DB定義の変更が含まれる場合は、設計承認とあわせて **Atlasマイグレーションの承認** を得る。手順は以下:
   - Plannerが `design-dock.md` に `db/crypto-trading-db/atlas/schema.hcl` の変更差分（追加・変更・削除するテーブル／カラム／インデックス）を明記する
   - メイン会話がユーザーに「`schema.hcl` を更新のうえ `make atlas-diff n=<timestamp>_<説明>` で生成されるマイグレーションSQLを確認・承認してほしい」と依頼する
   - **`make atlas-apply` はエージェントが実行しない**。適用可否とタイミングはユーザーが判断する
   - ユーザーからマイグレーション内容の承認を受け取るまで、Phase 2 を開始しない

> **【重大ルール】承認は必ずユーザーの明示的な承認発言をもって成立する。以下を AI が勝手に判断してはならない:**
> - **ユーザーが指摘・質問・修正依頼を出したこと ≠ チェック完了/承認。** ユーザーが指摘事項を挙げても、それが「すべての指摘を出し切った」ことを意味しない。ユーザーはまだレビューの途中である可能性が常にある。AI 側の判断で「指摘に対応したから承認された」「もう指摘は無いはず」と**勝手にチェック完了・承認済みと見なして Phase 2（実装）へ進んではならない。**
> - **承認の成立条件は、ユーザーが「承認する」「OK、この方針で実装を進めて」等の明示的な GO サインを出したときのみ。** 曖昧な相槌（「OK」「なるほど」等）が直前の指摘への同意なのか、全体承認なのか区別がつかない場合は、実装に進む前に「これで承認＝Sprint 開始してよいか」を確認する。
> - **DB 定義変更を伴う場合は、上記の設計承認に加えてマイグレーション内容の承認を受け取ること。** どちらか一方でも欠けていれば Phase 2 に進まない。
> - 設計レビュー中に新たな指摘が出たら、修正 → 再提示 → 再度の承認確認、を繰り返す。指摘対応のたびに承認を取り直す。


#### Phase 2: Sprint Loop

以下を全スプリント完了まで繰り返す:

**Step 1 — Generator 起動**
1. Agent toolで `.claude/agents/generator.md` を起動する
2. プロンプトに以下を含める:
   - 対象スプリント番号
   - `docs/features/{feature-name}/spec.md`, `docs/features/{feature-name}/design-dock.md`を読んで実装すること
   - 修正時は前回の evaluation_result.json の issues を含める
   - 更新系APIの実行許可をユーザーから得ている場合は、その旨と許可された内容を明記する
3. Generatorは `docs/features/{feature-name}/state/current_sprint.json` を更新して終了する

**Step 2 — Evaluator 起動**
1. Agent toolで `.claude/agents/evaluator.md` を起動する
2. プロンプトに以下を含める:
   - 対象スプリント番号
   - `docs/features/{feature-name}/state/current_sprint.json` と `docs/features/{feature-name}/spec.md` を読んで検証すること
   - 更新系APIの実行許可を得ている場合は、その旨と許可された内容を明記する
3. Evaluatorは `docs/features/{feature-name}/state/evaluation_result.json` を更新して終了する

**Step 3 — 判定**
1. `docs/features/{feature-name}/state/evaluation_result.json` を読む
2. `pending_user_approval` に項目があれば、**メイン会話がユーザーに提示して更新系APIの実行可否を確認する**（「更新系APIの承認フロー」参照）
3. `status` が `"pass"` → 以下を実行してから次のスプリントへ（Step 1に戻る）
   - `current_sprint.json` の `log_entry` を読み取り、**メイン会話が** `docs/features/{feature-name}/iteration_log.md` に追記する
4. `status` が `"fail"` → retry_count をインクリメント
   - retry_count < 3 → evaluation_result.json の issues をフィードバックとしてStep 1に戻る
   - retry_count >= 3 → **エスカレーション**: ユーザーに状況を報告し、判断を仰ぐ

#### Phase 3: Code Review

全スプリントが pass した後に **1 回だけ** 実行する。変更全体を対象にコードレビューを行い、指摘の採否をユーザーに判断してもらう。

**Step 1 — Reviewer 起動**
1. Agent toolで `.claude/agents/reviewer.md` を起動する
2. プロンプトに以下を含める:
   - `git diff main...HEAD` で機能ブランチの変更全体をレビューすること
   - `docs/features/{feature-name}/spec.md` / `design-dock.md` / `state/current_sprint.json` を読んで実装意図を把握すること
   - 変更が触れた各ディレクトリの `CLAUDE.md` の「開発ルール」に準拠しているか必ず照合すること
3. Reviewerは `docs/features/{feature-name}/state/review_result.json` と `docs/features/{feature-name}/review_report.md` を出力して終了する

**Step 2 — ユーザーへの指摘提示【必須ヒューマン・イン・ザ・ループ】**
1. `review_report.md` の指摘一覧を **メイン会話がユーザーに提示する**
2. ユーザーに各指摘の採否（「直す」／「不要」）を判断してもらう
3. **ユーザーの明示的な判断を受け取るまで先に進まない。** AI が勝手に採否を決めたり、全件修正したりしない

**Step 3 — 修正反映**
1. ユーザーが「直す」と判断した指摘 **のみ** を Generator に修正フィードバックとして渡す（`.claude/agents/generator.md` を修正モードで起動）
2. Generator 修正後、Evaluator を起動して回帰確認する（既存ジョブ・スプリント成果が壊れていないか）
3. 指摘が 0 件、またはユーザーが全て「不要」と判断した場合は、修正をスキップして完了へ進む

#### 完了

Phase 3 まで完了したら:
1. `docs/features/{feature-name}/iteration_log.md` に最終サマリー（レビュー指摘件数・採用/見送り件数を含む）を記録
2. ユーザーに完了報告を行う

### 制約

- **このルールは「ハーネスエンジニアリングを実行して」と言われた時のみ適用する**
- Planner完了後は必ずユーザー承認を取る（仕様に問題があるまま実装を進めない）
- **ユーザーの指摘・質問・修正依頼を「チェック完了/承認」と勝手に解釈しない。** 承認はユーザーの明示的な GO サインでのみ成立する。指摘が出ても他にも指摘が残っている可能性を前提とし、AI 判断で Phase 2 に進まない（詳細は Phase 1 の重大ルール参照）。DB 変更時は設計承認＋マイグレーション承認の両方が揃うまで進まない。
- 1スプリントずつ順番に実行する（並列実行しない）
- 各エージェントは必ず所定の状態ファイルを更新してから終了する
- Generator は仕様書にない機能を勝手に追加しない
- Evaluator は閾値を甘くして合格にしない
- **ユニットテストの不足を理由に不合格にしない。** テストは最低限でよく、Evaluator/Reviewer はテスト追加を強制しない（ただし壊れると気づきにくい箇所については「提案」としての指摘は可）
- **参照系APIは確認なしで積極的に使う。** 推測で実装・判定するより実レスポンスで裏を取ることを優先する
- **更新系API（発注・キャンセル）はユーザーの許可なく実行しない。** サブエージェントは `pending_user_approval` に記載して終了し、メイン会話がユーザーに確認する。ボット全体の起動（`make run` 等）、`make apply` / `make atlas-apply` も同様にエージェントは実行せず、ユーザーに依頼する
- 3回失敗したら無理に進めずユーザーに相談する
- **Phase 3（コードレビュー）は全スプリント完了後に必ず実行する。** Reviewer の指摘はそのまま自動修正せず、**必ずユーザーに提示して採否を判断してもらう（ヒューマン・イン・ザ・ループ必須）。** AI が勝手に採否を決めたり、ユーザー判断を待たずに Generator へ修正を渡してはならない。
- Reviewer は指摘の洗い出しに専念し、修正の要否を判断しない。「直す」と判断された指摘のみ Generator に渡す。
- `go/`, `terraform/` の修正を行う際は、それぞれのディレクトリ配下の `CLAUDE.md` に「開発ルール」セクションが存在するかを確認し、存在していれば遵守しつつ開発すること。ルート `CLAUDE.md` の「開発ルール」は常に適用する。`db/` `lambda/` には現時点で `CLAUDE.md` が無いため、ルートの規約に従う。
