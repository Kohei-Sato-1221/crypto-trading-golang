package bitflyerApp

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/enums"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

/*
reconcileJob は取引所とDBの状態を日次で突合し、乖離をSlackへ通知するジョブ。

背景:
2026-05 の障害は「取引所には存在しない注文がDB上 UNFILLED のまま残り、スロットを食い潰して
買い注文が1本も出せなくなる」という状態が、5ヶ月間まったく気づかれずに続いたものだった。
個々のジョブの修正（失効検出・ローリング）だけでは同種の「無言の停止」を防ぎきれないため、
毎日1回だけ全体を突合して「気づける」ようにするのが本ジョブの目的である。

突合する内容:
 1. 注文: 取引所のACTIVE注文（BTC_JPY / ETH_JPY、ページング取得）と
    DBの UNFILLED（buy / sell）を売買方向ごとに突合し、「DBにのみ存在」「取引所にのみ存在」を検出する。
    「取引所にのみ存在」は、発注APIのレスポンス消失などで生じるオーファン注文の検出も兼ねる。
    ただし SELL 側の「取引所にのみ存在」はユーザーの手動売却と区別できないため、
    エラー通知はせず日次サマリへの掲載に留める（classifyOrderDiffs のコメントを参照）。
 2. 残高: 取引所のBTC / ETH残高とDB上の想定保有量を突合する。
 3. 発注ゼロ: ボットの買い注文が設定日数(no_order_alert_days)以上発生していないことを検出する。
 4. ローリング再試行待ち: RemarkRolloverPending が残り続けているレコード
    （＝現物を保有したまま売り注文が無い状態に近づいている）を検出する。
 5. スロット: 未約定 buy / sell の件数と上限値を毎回のサマリに含め、sell 側の超過を警告する。

本ジョブは検知と通知に徹し、DBの更新も取引所への発注・キャンセルも一切行わない
（誤検知が状態変更につながらないようにするため）。

残高突合が「鳴りっぱなし」にならないための設計:
  - 手動保有分(models.RemarkManualHold)は乖離判定から完全に除外する。ボットは一切関与せず
    ユーザーが任意のタイミングで手動売却するため、判定に含めると売却した瞬間から
    「不足」側の乖離が恒久的に残り、毎日アラートが鳴り続けてしまう。
    サマリには内訳として表示するが、あくまで参考値であり判定には使わない。
  - DBが追跡していない既知の保有量は設定値 untracked_holding_btc / untracked_holding_eth で
    ベースラインとして与えられる。これを判定用の想定保有量に加算することで、過去の手動取引由来の
    差分が毎日アラートになることを防ぐ。
  - 乖離の向きで扱いを変える。「実残高 < 判定用の想定保有量」（不足）はボットが売るべき現物が
    足りない＝売り注文が残高不足で失敗しうる危険な状態なのでエラー通知する。
    「実残高 > 判定用の想定保有量」（余剰）は取引の安全性を損なわないため、
    日次サマリに数値を載せるだけでエラー通知はしない。
*/

// reconcileTarget は突合対象の通貨ペアと、その残高判定に使うパラメータ。
type reconcileTarget struct {
	ProductCode string
	Currency    string
	Threshold   float64 // 乖離を通知する閾値
	Untracked   float64 // DBが追跡していない既知の保有量（ベースライン）
}

const (
	// reconcileMaxUnfilledRecords はDBから取得する未約定レコードの上限（1テーブル・1通貨ペアあたり）。
	// 上限に達した場合は models が truncated を返し、本ジョブがSlackへ通知する。
	reconcileMaxUnfilledRecords = 500

	// reconcileSampleSize はSlack通知に載せる代表 order_id の最大件数。
	reconcileSampleSize = 10

	// reconcileRecentBuyOrderScan は発注ゼロ検知のために遡って走査する buy_orders の件数。
	// 買い注文ジョブは1日最大12本のため、200件あれば no_order_alert_days(既定3日)の判定に十分な余裕がある。
	reconcileRecentBuyOrderScan = 200

	// reconcileRolloverPendingMax はローリング再試行待ちレコードの取得上限。
	reconcileRolloverPendingMax = 50
)

// orderDiff は1通貨ペア・1売買方向ぶんの注文突合結果。
type orderDiff struct {
	ProductCode  string
	Side         string
	Table        models.OrderTable
	DBOnly       []string // DBは UNFILLED だが取引所のACTIVE一覧に無い（幽霊レコードの疑い）
	ExchangeOnly []string // 取引所にACTIVEで存在するがDBに UNFILLED として無い（オーファン注文の疑い）
	// RolloverPending は DBOnly のうち [ROLLOVER_PENDING] が付いているもの。
	// 同じ事象が reconcileRolloverPending でも通知されるため、こちらでは重複してエラー通知しない
	RolloverPending []string
}

/*
buildOrderDiff は1通貨ペア・1売買方向ぶんの突合結果を作る（DB・APIに触れない純粋関数）。

[ROLLOVER_PENDING] が付いたレコードは「キャンセル成功・再発注失敗」の状態であり、
取引所側に注文が存在しないのは当然なので必ず「DBのみ UNFILLED」として現れる。
これを DBOnly に含めると、reconcileRolloverPending の残留通知と合わせて
同じ事象で毎日2通のエラー通知が飛ぶため、専用通知の側へ一本化する。
*/
func buildOrderDiff(productCode, side string, table models.OrderTable,
	dbOrderIDs []string, exchangeIDs map[string]bool, rolloverPendingIDs map[string]bool) orderDiff {

	diff := orderDiff{ProductCode: productCode, Side: side, Table: table}
	dbIDs := make(map[string]bool, len(dbOrderIDs))
	for _, orderID := range dbOrderIDs {
		dbIDs[orderID] = true
		if exchangeIDs[orderID] {
			continue
		}
		if rolloverPendingIDs[orderID] {
			diff.RolloverPending = append(diff.RolloverPending, orderID)
			continue
		}
		diff.DBOnly = append(diff.DBOnly, orderID)
	}
	for orderID := range exchangeIDs {
		if !dbIDs[orderID] {
			diff.ExchangeOnly = append(diff.ExchangeOnly, orderID)
		}
	}
	return diff
}

// rolloverPendingOrderIDs は [ROLLOVER_PENDING] 付きレコードの order_id 集合を返す。
func rolloverPendingOrderIDs(records []models.OrderRecord) map[string]bool {
	ids := make(map[string]bool, len(records))
	for _, record := range records {
		if record.OrderID != "" {
			ids[record.OrderID] = true
		}
	}
	return ids
}

/*
balanceDiff は1通貨ぶんの残高突合結果。

判定に使うのは AlertExpected（= Holding.AlertTarget() + Untracked）だけである。
Holding.Manual（手動保有分）は AlertExpected に含めず、参考値としてサマリに表示するのみ。
*/
type balanceDiff struct {
	Currency    string
	ProductCode string
	Exchange    float64                // 取引所の保有量（注文で拘束された分を含む総額）
	Holding     models.ExpectedHolding // DB上の想定保有量の内訳（Manualは判定対象外）
	Untracked   float64                // 設定値のベースライン（判定に含める）
	// AlertExpected は乖離判定に使う想定保有量。Holding.AlertTarget()(=Bot+Naked) + Untracked。
	// 手動保有分は含まない（ユーザーが売買しても判定結果が動かないようにするため）
	AlertExpected float64
	Diff          float64 // Exchange - AlertExpected（正なら余剰・負なら不足）
	Threshold     float64
}

// isShortfall は「実残高が想定保有量より閾値を超えて少ない」状態かを返す（危険側の乖離）。
func (d balanceDiff) isShortfall() bool {
	return -d.Diff > d.Threshold
}

// isSurplus は「実残高が想定保有量より閾値を超えて多い」状態かを返す（安全側の乖離）。
func (d balanceDiff) isSurplus() bool {
	return d.Diff > d.Threshold
}

// reconcileSummary はジョブ全体の結果。
type reconcileSummary struct {
	slotStatus      *models.BuyOrderSlotStatus
	orderDiffs      []orderDiff
	balanceDiffs    []balanceDiff
	lastBotOrder    *time.Time
	botOrders24h    int
	manualOrders24h int
	rolloverPending []models.OrderRecord
	diffDetails     orderDiffDetails
	errors          int
}

// dbOnlyCount はDBにのみ存在する注文の合計件数を返す。
func (s *reconcileSummary) dbOnlyCount() int {
	count := 0
	for _, diff := range s.orderDiffs {
		count += len(diff.DBOnly)
	}
	return count
}

// exchangeOnlyCount は取引所にのみ存在する注文の合計件数を返す。
func (s *reconcileSummary) exchangeOnlyCount() int {
	count := 0
	for _, diff := range s.orderDiffs {
		count += len(diff.ExchangeOnly)
	}
	return count
}

// reconcileTargets は設定値を反映した突合対象を返す。
func reconcileTargets() []reconcileTarget {
	btcThreshold := config.Config.BalanceDiffThresholdBTC
	if btcThreshold <= 0 {
		btcThreshold = config.DefaultBalanceDiffThresholdBTC
	}
	ethThreshold := config.Config.BalanceDiffThresholdETH
	if ethThreshold <= 0 {
		ethThreshold = config.DefaultBalanceDiffThresholdETH
	}
	return []reconcileTarget{
		{ProductCode: "BTC_JPY", Currency: "BTC", Threshold: btcThreshold, Untracked: config.Config.UntrackedHoldingBTC},
		{ProductCode: "ETH_JPY", Currency: "ETH", Threshold: ethThreshold, Untracked: config.Config.UntrackedHoldingETH},
	}
}

// reconcileNoOrderAlertDays は発注ゼロを検知するまでの日数を返す。
func reconcileNoOrderAlertDays() int {
	days := config.Config.NoOrderAlertDays
	if days <= 0 {
		days = config.DefaultNoOrderAlertDays
	}
	return days
}

// notifyReconcileError はエラーをログとSlack（エラーチャンネル）へ通知し、件数を集計する。
func notifyReconcileError(summary *reconcileSummary, format string, args ...any) {
	summary.errors++
	msg := fmt.Sprintf("🚨【reconcile】"+format, args...)
	log.Println(msg)
	slackClient.PostMessage(msg, true)
}

// sampleOrderIDs は通知用に order_id を最大 reconcileSampleSize 件へ切り詰める。
func sampleOrderIDs(orderIDs []string) string {
	ids := orderIDs
	suffix := ""
	if len(ids) > reconcileSampleSize {
		ids = ids[:reconcileSampleSize]
		suffix = fmt.Sprintf(" ...他%d件", len(orderIDs)-reconcileSampleSize)
	}
	return strings.Join(ids, ", ") + suffix
}

/*
reconcileOrdersForProduct は1通貨ペアぶんの注文突合を行う。

取引所のACTIVE一覧の取得に失敗した場合は、その通貨ペアの突合自体をスキップする
（不完全な一覧で突合すると、実在する注文を「DBにのみ存在」と誤検知するため）。
売買方向ごとに突合するのは、買い注文(buy_orders)と売り注文(sell_orders)で
DB側のテーブルが分かれているためである。
*/
func reconcileOrdersForProduct(apiClient *bitflyer.APIClient, target reconcileTarget,
	rolloverPendingIDs map[string]bool, summary *reconcileSummary) {
	activeOrders, _, err := apiClient.GetChildOrdersAll(target.ProductCode, "ACTIVE")
	if err != nil {
		notifyReconcileError(summary, "ACTIVE注文一覧の取得に失敗したため注文突合をスキップします: product_code=%s err=%v",
			target.ProductCode, err)
		return
	}

	exchangeIDs := map[string]map[string]bool{
		"BUY":  make(map[string]bool),
		"SELL": make(map[string]bool),
	}
	for _, order := range activeOrders {
		side := strings.ToUpper(order.Side)
		if _, ok := exchangeIDs[side]; !ok {
			log.Printf("【reconcile】想定外のsideの注文を無視します: product_code=%s OrderID=%s side=%s",
				target.ProductCode, order.ChildOrderAcceptanceID, order.Side)
			continue
		}
		exchangeIDs[side][order.ChildOrderAcceptanceID] = true
	}

	sides := []struct {
		side  string
		table models.OrderTable
	}{
		{"BUY", models.TableBuyOrders},
		{"SELL", models.TableSellOrders},
	}

	for _, s := range sides {
		dbOrderIDs, truncated, err := models.GetUnfilledOrderIDs(s.table, target.ProductCode, reconcileMaxUnfilledRecords)
		if err != nil {
			notifyReconcileError(summary, "未約定レコードの取得に失敗: table=%s product_code=%s err=%v",
				s.table, target.ProductCode, err)
			continue
		}
		if truncated {
			// 打ち切りが起きると突合の前提が崩れる。「乖離なし」と通知されるのが最も危険なので必ず鳴らす
			notifyReconcileError(summary, "未約定レコードの取得が上限(%d件)で打ち切られました: table=%s product_code=%s。"+
				"突合結果が実態とずれている可能性があります（DBのみ/取引所のみの件数を鵜呑みにしないでください）",
				reconcileMaxUnfilledRecords, s.table, target.ProductCode)
		}

		diff := buildOrderDiff(target.ProductCode, s.side, s.table, dbOrderIDs, exchangeIDs[s.side], rolloverPendingIDs)
		log.Printf("【reconcile】注文突合 product_code:%s side:%s 取引所ACTIVE:%d件 DB UNFILLED:%d件 DBのみ:%d件 取引所のみ:%d件 ローリング再試行待ち:%d件",
			target.ProductCode, s.side, len(exchangeIDs[s.side]), len(dbOrderIDs),
			len(diff.DBOnly), len(diff.ExchangeOnly), len(diff.RolloverPending))
		summary.orderDiffs = append(summary.orderDiffs, diff)
	}
}

/*
orderDiffDetails は注文突合の結果を「エラー通知するもの」と「日次サマリに載せるだけのもの」に
分けて保持する。

sell_orders は取引所から同期する仕組みが無い（syncBuyOrders は side=="BUY" のみ取り込む）。
ユーザーは手動保有ポジションを自分のタイミングで売却するため、そのために取引所へ置いた
SELL 指値は約定するまで必ず「取引所のみ ACTIVE」として現れる。これをエラー通知にすると
売却を始めた日から毎日鳴り続け、本物の乖離が埋もれる。
手動保有はシステムから存在しないものとして扱う方針（残高突合が Manual を判定から
除外しているのと同じ考え方）に合わせ、SELL 側の「取引所のみ ACTIVE」は
エラー通知せず日次サマリへの掲載に留める。

ボットが発注した売り注文の取りこぼし（再発注レスポンスの消失によるオーファン注文）は、
rolloverSellOrderJob の再発注前オーファン検出と [ROLLOVER_PENDING] 残留通知で検知する。
*/
type orderDiffDetails struct {
	Alerts     []string // エラー通知に載せる明細
	Info       []string // 日次サマリにのみ載せる明細
	AlertCount int      // エラー通知の対象件数
	InfoCount  int      // サマリ掲載のみの件数
}

/*
classifyOrderDiffs は突合結果をエラー通知用・サマリ掲載用に振り分ける。

  - DBのみ UNFILLED（buy / sell 両方） → エラー通知。失効の取りこぼしでスロットを食う
  - 取引所のみ ACTIVE / BUY → エラー通知。ボットの買い注文はDBに必ず記録されるため、
    記録されていない ACTIVE な買い注文は取りこぼし（オーファン）の疑いがある
  - 取引所のみ ACTIVE / SELL → サマリ掲載のみ。ユーザーの手動売却と区別できないため
*/
func classifyOrderDiffs(diffs []orderDiff) orderDiffDetails {
	details := orderDiffDetails{}
	for _, diff := range diffs {
		if len(diff.DBOnly) > 0 {
			details.Alerts = append(details.Alerts, fmt.Sprintf("DBのみ %s/%s(%s) %d件: [%s]",
				diff.ProductCode, diff.Side, diff.Table, len(diff.DBOnly), sampleOrderIDs(diff.DBOnly)))
			details.AlertCount += len(diff.DBOnly)
		}
		if len(diff.RolloverPending) > 0 {
			// [ROLLOVER_PENDING] は reconcileRolloverPending が専用通知を出すため、ここでは重複通知しない
			details.Info = append(details.Info, fmt.Sprintf("DBのみ %s/%s(%s) %d件(%s。専用通知に一本化): [%s]",
				diff.ProductCode, diff.Side, diff.Table, len(diff.RolloverPending),
				models.RemarkRolloverPending, sampleOrderIDs(diff.RolloverPending)))
			details.InfoCount += len(diff.RolloverPending)
		}
		if len(diff.ExchangeOnly) == 0 {
			continue
		}
		if strings.ToUpper(diff.Side) == "SELL" {
			details.Info = append(details.Info, fmt.Sprintf("取引所のみ %s/SELL %d件(手動売却の可能性。アラート対象外): [%s]",
				diff.ProductCode, len(diff.ExchangeOnly), sampleOrderIDs(diff.ExchangeOnly)))
			details.InfoCount += len(diff.ExchangeOnly)
			continue
		}
		details.Alerts = append(details.Alerts, fmt.Sprintf("取引所のみ %s/%s %d件: [%s]",
			diff.ProductCode, diff.Side, len(diff.ExchangeOnly), sampleOrderIDs(diff.ExchangeOnly)))
		details.AlertCount += len(diff.ExchangeOnly)
	}
	return details
}

// notifyOrderDiffs は注文の乖離をSlackへ通知する（エラー通知対象が無ければ何もしない）。
// サマリ掲載のみの明細は notifyReconcileResult が日次サマリに載せる。
func notifyOrderDiffs(summary *reconcileSummary) {
	summary.diffDetails = classifyOrderDiffs(summary.orderDiffs)
	if summary.diffDetails.AlertCount == 0 {
		return
	}

	msg := fmt.Sprintf("🚨【reconcile】注文の乖離を検出: DBのみ UNFILLED:%d件 / 取引所のみ ACTIVE(BUY):%d件\n%s\n"+
		"※DBのみ=失効の取りこぼし（翌日の expireSweep で解消するか確認）／取引所のみ=DBに記録されていない注文（オーファン注文の疑い）",
		summary.dbOnlyCount(), summary.diffDetails.AlertCount-summary.dbOnlyCount(),
		strings.Join(summary.diffDetails.Alerts, "\n"))
	log.Println(msg)
	slackClient.PostMessage(msg, true)
}

/*
reconcileBalances は取引所の残高とDB上の想定保有量を突合する。

取引所側は Amount（注文で拘束された分を含む総保有量）を使う。
Available は未約定の売り注文に拘束された分が引かれており、
「売り注文が生きている＝現物を保有している」ぶんを含むDB側の想定保有量とは意味が合わないため。

乖離判定に使う想定保有量は Bot + Naked + Untracked であり、手動保有(Manual)は含めない。
手動保有はユーザーが任意のタイミングで売却するため、判定に含めると売却後に
「不足」側の乖離が恒久的に残り、アラートが鳴り続けてしまう。
*/
/*
lookupExchangeBalance は残高レスポンス(/v1/me/getbalance)から対象通貨の保有量を取り出す。

第2返り値の found は「レスポンスに対象通貨のエントリが存在したか」を表す。
map のゼロ値をそのまま使うと、レスポンスに BTC / ETH が含まれていない場合に
「実残高 0」として扱われ、必ず「不足」側の🚨アラートになってしまう
（方向は安全側だが、原因の分からない誤アラートが毎日鳴り続ける）。
呼び出し側は found=false のとき突合をスキップして通知すること。

Amount（拘束分を含む総保有量）を返す。Available は未約定の売り注文に拘束された分が
引かれており、DB側の想定保有量とは意味が合わないため使わない。
*/
func lookupExchangeBalance(balances []bitflyer.Balance, currency string) (float64, bool) {
	for _, balance := range balances {
		if balance.CurrentCode == currency {
			return balance.Amount, true
		}
	}
	return 0, false
}

/*
balanceCurrencyList は残高レスポンスに含まれる通貨コードを通知用に連結する。

対象通貨が見つからなかった原因（レスポンス形式の変更なのか、一時的な欠落なのか）を
切り分けられるようにするための情報。金額は含めない（機密情報をログ・通知に残さないため）。
件数が多いので reconcileSampleSize 件で切り詰める。
*/
func balanceCurrencyList(balances []bitflyer.Balance) string {
	codes := make([]string, 0, len(balances))
	for i, balance := range balances {
		if i >= reconcileSampleSize {
			break
		}
		codes = append(codes, balance.CurrentCode)
	}
	text := strings.Join(codes, ", ")
	if len(balances) > reconcileSampleSize {
		text += fmt.Sprintf(" ...他%d件", len(balances)-reconcileSampleSize)
	}
	return text
}

func reconcileBalances(apiClient *bitflyer.APIClient, summary *reconcileSummary) {
	balances, err := apiClient.GetBalance()
	if err != nil {
		notifyReconcileError(summary, "残高の取得に失敗したため残高突合をスキップします: err=%v", err)
		return
	}
	holdings, err := models.GetExpectedHoldings()
	if err != nil {
		notifyReconcileError(summary, "DB上の想定保有量の取得に失敗したため残高突合をスキップします: err=%v", err)
		return
	}

	for _, target := range reconcileTargets() {
		// 対象通貨がレスポンスに含まれていない場合、ゼロ値0を「実残高0」として扱うと
		// 必ず「不足」側の🚨アラートになる。原因不明の誤発報を避けるため突合自体を見送る
		amount, found := lookupExchangeBalance(balances, target.Currency)
		if !found {
			notifyReconcileError(summary,
				"取引所の残高レスポンスに %s が含まれていないため、%s の残高突合をスキップします "+
					"（取得できた通貨:%d件 [%s]）。残高0とみなすと必ず不足アラートになるため判定しません",
				target.Currency, target.ProductCode, len(balances), balanceCurrencyList(balances))
			continue
		}

		holding := holdings[target.ProductCode]
		diff := balanceDiff{
			Currency:    target.Currency,
			ProductCode: target.ProductCode,
			Exchange:    amount,
			Holding:     holding,
			Untracked:   target.Untracked,
			Threshold:   target.Threshold,
		}
		// 判定に使うのは Bot + Naked + Untracked のみ。手動保有(Manual)は含めない
		diff.AlertExpected = holding.AlertTarget() + target.Untracked
		diff.Diff = diff.Exchange - diff.AlertExpected
		summary.balanceDiffs = append(summary.balanceDiffs, diff)

		log.Printf("【reconcile】残高突合 %s bot:%v naked:%v untracked:%v alertExpected:%v diff:%v threshold:%v "+
			"(参考: manual:%v は判定対象外)",
			target.Currency, holding.Bot, holding.Naked, target.Untracked,
			diff.AlertExpected, diff.Diff, target.Threshold, holding.Manual)

		if diff.isShortfall() {
			// 実残高が判定用の想定保有量より少ない＝ボットが売るべき現物が足りず、
			// 売り注文が残高不足で失敗しうる危険な状態
			msg := fmt.Sprintf("🚨【reconcile】%s の残高がボットの想定保有量を下回っています: 取引所:%v 判定用DB想定:%v (差分:%v 閾値:%v)\n"+
				"判定対象の内訳 bot:%v 裸の保有:%v 既知の未追跡分:%v / 参考(判定対象外) 手動保有:%v\n"+
				"※売り注文が残高不足で失敗する可能性があります",
				target.Currency, diff.Exchange, diff.AlertExpected, diff.Diff, diff.Threshold,
				holding.Bot, holding.Naked, target.Untracked, holding.Manual)
			log.Println(msg)
			slackClient.PostMessage(msg, true)
		}
	}
}

/*
reconcileBotOrderActivity はボットの買い注文の発注状況を集計する。

直近 reconcileRecentBuyOrderScan 件の買い注文を新しい順に走査し、
enums.IsBotStrategy() が真のレコードの最終発注時刻を求める。
戦略値が StrategyManual(90001) / StrategyUnknown(99) / StrategySaturatedUnknown(127) の
レコードはボット発注ではないため、発注ゼロの判定には数えない。

あわせて直近24時間の「ボット発注件数」と「手動取り込み件数」を数え、日次サマリに載せる。
ボット発注のDB INSERTに失敗した注文は、90秒周期の syncBuyOrders が手動注文(strategy=90001)として
取り込むため、通常0件であるはずの手動取り込み件数が増えていれば記録の取りこぼしに気づける。
*/
func reconcileBotOrderActivity(now time.Time, summary *reconcileSummary) {
	records, err := models.GetRecentBuyOrders(reconcileRecentBuyOrderScan)
	if err != nil {
		notifyReconcileError(summary, "直近の買い注文の取得に失敗したため発注ゼロ検知をスキップします: err=%v", err)
		return
	}

	dayAgo := now.Add(-24 * time.Hour)
	for _, record := range records {
		isBot := enums.IsBotStrategy(record.Strategy)
		if isBot && (summary.lastBotOrder == nil || record.Timestamp.After(*summary.lastBotOrder)) {
			timestamp := record.Timestamp
			summary.lastBotOrder = &timestamp
		}
		if !record.Timestamp.After(dayAgo) {
			continue
		}
		if isBot {
			summary.botOrders24h++
		} else if record.Strategy == enums.StrategyManual {
			summary.manualOrders24h++
		}
	}

	alertDays := reconcileNoOrderAlertDays()
	threshold := now.AddDate(0, 0, -alertDays)
	if summary.lastBotOrder == nil {
		msg := fmt.Sprintf("🚨【reconcile】ボットの買い注文が確認できません。最終発注: 直近%d件のbuy_ordersにボット発注なし（スキャン件数:%d）",
			reconcileRecentBuyOrderScan, len(records))
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		return
	}
	if summary.lastBotOrder.Before(threshold) {
		msg := fmt.Sprintf("🚨【reconcile】ボットの買い注文が %d日間 0件です。最終発注(UTC): %s",
			alertDays, summary.lastBotOrder.Format(time.RFC3339))
		log.Println(msg)
		slackClient.PostMessage(msg, true)
	}
}

/*
reconcileRolloverPending はローリング再試行待ちのまま残っている売り注文を検出する。

RemarkRolloverPending は「キャンセルは成功したが再発注に失敗した」ことを示すマーカーで、
翌日のローリングで再試行されて解消されるのが正常な流れである。これが残り続けている場合は
ローリングが繰り返し失敗しており、現物を保有したまま売り注文が存在しない
（＝利確機会を失う）状態に近づいているため通知する。
*/
func reconcileRolloverPending(summary *reconcileSummary) {
	records, err := models.GetUnfilledOrdersWithRemark(
		models.TableSellOrders, models.RemarkRolloverPending, reconcileRolloverPendingMax)
	if err != nil {
		notifyReconcileError(summary, "ローリング再試行待ちレコードの取得に失敗: err=%v", err)
		return
	}
	if len(records) == 0 {
		return
	}
	summary.rolloverPending = records

	details := make([]string, 0, len(records))
	for i, record := range records {
		if i >= reconcileSampleSize {
			break
		}
		details = append(details, fmt.Sprintf("OrderID=%s ParentID=%s %s price=%.2f size=%v",
			record.OrderID, record.ParentID, record.ProductCode, record.Price, record.Size))
	}
	msg := fmt.Sprintf("🚨【reconcile】ローリング再試行待ち(%s)のまま残っている売り注文が %d件あります。"+
		"再発注が繰り返し失敗している可能性があります（現物を保有したまま売り注文が無い状態に近づいています）\n%s",
		models.RemarkRolloverPending, len(records), strings.Join(details, "\n"))
	log.Println(msg)
	slackClient.PostMessage(msg, true)
}

/*
balanceStateLabel は残高突合の状態ラベルを返す。

余剰の主因は手動保有(models.RemarkManualHold)である。手動保有は AlertExpected から
除外されているため、そのぶんは必ず「実残高 > 判定用のDB想定」の側に出る。
以前のラベル「余剰(DB未追跡分。アラート対象外)」は untracked_holding_btc /
untracked_holding_eth の設定漏れと誤解されるため、実態に合わせて手動保有にも言及する。
*/
func balanceStateLabel(diff balanceDiff) string {
	if diff.isShortfall() {
		return "🚨不足"
	}
	if diff.isSurplus() {
		return "余剰(手動保有・未追跡分を含む。アラート対象外)"
	}
	return "乖離なし"
}

// formatReconcileTime は通知用に時刻(UTC)を文字列化する。未取得は "なし"。
func formatReconcileTime(t *time.Time) string {
	if t == nil {
		return "なし"
	}
	return t.UTC().Format(time.RFC3339)
}

/*
notifyReconcileResult は日次サマリを通常チャンネルへ通知する。

乖離の有無にかかわらず毎日1通投稿する。「通知が来ていない＝ジョブが動いていない」ことを
運用で判別できるようにするためであり、無言の停止を防ぐという本ジョブの目的そのものにあたる。
残高は「余剰（実残高 > DB想定）」であればここに数値を載せるだけでエラー通知はしない。
*/
func notifyReconcileResult(summary *reconcileSummary) {
	slotText := "スロット状況: 取得失敗"
	if summary.slotStatus != nil {
		slotText = summary.slotStatus.Message
		if summary.slotStatus.ShouldSkip {
			slotText += "（🚨買い注文が上限到達）"
		}
		if summary.slotStatus.SellWarning {
			slotText += "（🚨売り注文が上限超過）"
		}
	}

	balanceTexts := make([]string, 0, len(summary.balanceDiffs))
	for _, diff := range summary.balanceDiffs {
		state := balanceStateLabel(diff)
		balanceTexts = append(balanceTexts, fmt.Sprintf(
			"%s %s 取引所:%v 判定用DB想定:%v 差分:%v（判定対象の内訳 bot:%v 裸の保有:%v 既知の未追跡分:%v / 閾値:%v）"+
				"（参考・判定対象外 手動保有:%v）",
			diff.Currency, state, diff.Exchange, diff.AlertExpected, diff.Diff,
			diff.Holding.Bot, diff.Holding.Naked, diff.Untracked, diff.Threshold, diff.Holding.Manual))
	}
	balanceText := "残高突合: 実行できず"
	if len(balanceTexts) > 0 {
		balanceText = "残高突合:\n" + strings.Join(balanceTexts, "\n")
	}

	// 取引所のみ ACTIVE(SELL) はエラー通知しない代わりに、必ずここへ明細を載せる
	// （通知を消してしまうと気づけなくなるため）
	orderText := ""
	if len(summary.diffDetails.Info) > 0 {
		orderText = "\n注文突合(アラート対象外):\n" + strings.Join(summary.diffDetails.Info, "\n")
	}

	msg := fmt.Sprintf("【reconcile】%s / 注文突合 DBのみ:%d件 取引所のみ:%d件(うちアラート対象外 SELL:%d件) / "+
		"ボット発注 直近24h:%d件 手動取り込み:%d件 最終発注(UTC):%s / ローリング再試行待ち:%d件 / エラー:%d件\n%s%s",
		slotText, summary.dbOnlyCount(), summary.exchangeOnlyCount(), summary.diffDetails.InfoCount,
		summary.botOrders24h, summary.manualOrders24h, formatReconcileTime(summary.lastBotOrder),
		len(summary.rolloverPending), summary.errors, balanceText, orderText)
	log.Println(msg)
	slackClient.PostMessage(msg, false)
}

/*
reconcileJob はスケジューラから呼ばれるエントリポイント。

スケジューラの時刻指定はシステムTZ（本番はJST）に依存するため、
TZの誤設定を運用で検知できるようジョブ冒頭でローカル時刻とUTCの双方をログ出力する。
時刻の比較はすべてUTC同士で行う。
*/
func reconcileJob(apiClient *bitflyer.APIClient) {
	now := time.Now().UTC()
	log.Printf("【reconcile】start of job now(local):%s now(UTC):%s",
		time.Now().Format(time.RFC3339), now.Format(time.RFC3339))

	summary := &reconcileSummary{}

	// スロット状況（上限値つき）。sell側の超過はここで警告するが買い注文はブロックしない
	slotStatus, err := models.GetBuyOrderSlotStatus(apiClient.Max_buy_orders, apiClient.Max_sell_orders)
	if err != nil {
		notifyReconcileError(summary, "スロット状況の取得に失敗: err=%v", err)
	} else {
		summary.slotStatus = slotStatus
		if slotStatus.ShouldSkip {
			notifyReconcileError(summary, "買い注文スロットが上限に達しています: %s。買い注文はスキップされます", slotStatus.Message)
		}
		if slotStatus.SellWarning {
			notifyReconcileError(summary, "売り注文が上限超過: %s。買い注文はブロックされませんが、"+
				"現物の積み上がりとJPY残高の歯止め(budget_criteria)を確認してください", slotStatus.Message)
		}
	}

	// ローリング再試行待ちを先に取得する。[ROLLOVER_PENDING] 付きレコードは必ず
	// 「DBのみ UNFILLED」として現れるため、注文突合から除外して通知を専用の1通へ一本化する。
	// 取得は reconcileRolloverPendingMax 件で打ち切られるため、それを超えた分は
	// 従来どおり「DBのみ UNFILLED」としてエラー通知される（見落とすより鳴らす側に倒す）
	reconcileRolloverPending(summary)
	pendingIDs := rolloverPendingOrderIDs(summary.rolloverPending)

	for _, target := range reconcileTargets() {
		reconcileOrdersForProduct(apiClient, target, pendingIDs, summary)
	}
	notifyOrderDiffs(summary)

	reconcileBalances(apiClient, summary)
	reconcileBotOrderActivity(now, summary)

	notifyReconcileResult(summary)
	log.Println("【reconcile】end of job")
}
