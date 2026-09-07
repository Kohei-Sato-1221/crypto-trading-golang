package bitflyerApp

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/utils"
)

/*
rolloverSellOrderJob は売り注文を期限前に巻き直して実質無期限化する日次ジョブ。

背景:
Bitflyerの注文有効期限の上限は43200分(30日)で、無期限注文は作れない。
一方このボットの売り注文は「買値 + 利確幅」の指値であり、約定まで数ヶ月かかることもある。
放置すると30日で必ず失効し、現物だけが手元に残って売り注文が存在しない状態になる。
そこで期限の sell_rollover_days_before_expire 日前(既定3日 = 発注から27日目)に
キャンセル → 同条件で再発注し、売り注文を保持し続ける。

処理順序（順序そのものが安全性の要）:
  ① 事前に取得したCOMPLETED一覧と突合し、既に約定していればFILLEDに更新して対象から外す
     （約定済みの注文にキャンセルAPIを叩かない）
  ② CancelOrder() を実行し、成功を確認する（スプリント1でレスポンス検証済み。エラーなら再発注しない）
  ③ 個別照会で「消滅」または「CANCELED」を再確認する
     - COMPLETED が返った場合は キャンセル直前に約定していたということなので
       再発注せず FILLED に更新する（二重売りの防止）
     - ACTIVE のままならキャンセル未成立。再発注せず次のレコードへ
  ④ 同一 product_code / price / size で再発注する（価格の再計算は一切行わない）
  ⑤ 旧レコードのCANCELLED化と新レコードのINSERTを単一トランザクションで実行する
     （新レコードは parentid を引き継ぐため損益計算の紐付けが維持される）

  ※ models.RemarkRolloverPending 付きレコードの再試行では、④の前に取引所のACTIVE一覧から
    「同一 product_code / price / size / side=SELL で、DBに紐づかない注文」を探す。
    前回 PlaceOrder のリクエストは届いたがレスポンスを取りこぼした場合、取引所側にだけ
    注文が存在する（オーファン注文）。これを確認せずに再発注すると売り注文が2本並び、
    拘束されていない現物（手動保有分）まで売られる恐れがあるため、
    見つかった場合は再発注せずその order_id をDBへ取り込む（二重売りの防止）。

異常系:
  - キャンセル成功・再発注失敗は「現物を保有しているのに売り注文が無い（裸の保有）」状態になるため
    最優先でSlack通知し、models.RemarkRolloverPending マーカーを付けて次回ジョブでリトライする。
    このマーカーが付いたレコードは expireSweepJob の対象から除外される（勝手にCANCELLEDへ落とされない）。
  - 1件の失敗で全体を止めない。ループは break せず continue する。
  - 期限3日前から対象になるため日次ジョブで最大3回の再試行機会がある。
    3回とも失敗して売り注文が失効した場合、親の買い注文は FILLED(SELL ORDER PLACED) のまま戻さず、
    自動再発注も行わない。expireSweepJob がSlack通知を出し、ユーザーが手動で対応する（確定仕様）。

手動保有レコードの保護:
  抽出条件(models.GetSellOrdersToRollover)が status = 'UNFILLED' に限定されているため、
  手動保有へ移管した sell_orders 65件（status = 'CANCELLED'）は構造的に対象外になる。
  その親買い注文65件（status = 'FILLED(SELL ORDER PLACED)'）も本ジョブは一切参照しない。
*/

/*
rolloverMinOrderSize は通貨ペアごとのBitflyerの最小取引単位。

これを下回る size のレコードは再発注が取引所に拒否される。
キャンセルしてから拒否されると裸の保有になるため、キャンセルする前にスキップして通知する。
*/
var rolloverMinOrderSize = map[string]float64{
	"BTC_JPY": 0.001,
	"ETH_JPY": 0.01,
}

// rolloverAPIInterval は1レコード処理ごとのスリープ時間。
// 1件あたり最大4リクエスト(キャンセル前照会/キャンセル/キャンセル後照会/発注)のため、
// レート制限に対する余裕を作る。
const rolloverAPIInterval = 500 * time.Millisecond

// rolloverSellOrderSummary はジョブ全体の処理結果。
type rolloverSellOrderSummary struct {
	target    int // 抽出した対象件数
	succeeded int // 巻き直しに成功した件数
	filled    int // 約定が判明しFILLEDに更新した件数
	skipped   int // 最小取引単位未満・取引所に注文が無いなどで処理しなかった件数
	failed    int // キャンセル失敗・再発注失敗・DB更新失敗の件数

	partiallyFilled int // 部分約定を検出し、残数量で再発注した件数
	adopted         int // 前回のレスポンス取りこぼしで生まれたオーファン注文をDBに取り込んだ件数
}

// rolloverDaysBeforeExpire は「有効期限の何日前に巻き直すか」を返す。
func rolloverDaysBeforeExpire() int {
	if days := config.Config.BFSellRolloverDaysBeforeExpire; days > 0 {
		return days
	}
	return config.DefaultSellRolloverDaysBeforeExpire
}

// rolloverFallbackDays は expire_date 未設定の旧レコードを対象とみなす経過日数を返す。
func rolloverFallbackDays() int {
	if days := config.Config.BFSellRolloverFallbackDays; days > 0 {
		return days
	}
	return config.DefaultSellRolloverFallbackDays
}

// rolloverMaxPerRun は1回の実行で処理する上限件数を返す。
func rolloverMaxPerRun() int {
	if limit := config.Config.BFSellRolloverMaxPerRun; limit > 0 {
		return limit
	}
	return config.DefaultSellRolloverMaxPerRun
}

/*
rolloverSellMinuteToExpire は再発注する売り注文の有効期限(分)を返す。

Bitflyerの上限は43200分(30日)で、これを超える値を指定すると発注が拒否される。
ローリングでは「キャンセル成功・再発注失敗」がそのまま裸の保有になるため、
設定ミスで発注が弾かれないよう上限値(config.DefaultSellMinuteToExpire)で丸める。
*/
func rolloverSellMinuteToExpire() int {
	minutes := config.Config.BFSellMinuteToExpire
	if minutes <= 0 || minutes > config.DefaultSellMinuteToExpire {
		return config.DefaultSellMinuteToExpire
	}
	return minutes
}

/*
rolloverRecordContextWithPrefix は通知・ログに載せる共通のコンテキスト文字列を返す。

開発ルール（エラー通知にOrderID・ParentID・ProductCode・Price・Sizeを含める）に従うためのヘルパー。
prefix は OrderID のラベルにのみ付く（prefix="Old" なら "OldOrderID=..."）。
巻き直し後の通知で新旧のOrderIDを取り違えないようにするためのもの。
*/
func rolloverRecordContextWithPrefix(record models.SellOrderRecord, prefix string) string {
	return fmt.Sprintf("%sOrderID=%s ParentID=%s %s price=%.2f size=%v expire_date(UTC)=%s",
		prefix, record.OrderID, record.ParentID, record.ProductCode, record.Price, record.Size,
		formatSweepExpireDate(record.ExpireDate))
}

// rolloverRecordContext はラベルを付けないコンテキスト文字列を返す。
func rolloverRecordContext(record models.SellOrderRecord) string {
	return rolloverRecordContextWithPrefix(record, "")
}

/*
rolloverOrderIndex は通貨ペアごとに1回だけ取得した取引所の注文一覧。

completed は「処理前の約定」判定に使う（キャンセルAPIを約定済み注文に叩かないため）。
active は models.RemarkRolloverPending 付きレコードの再試行で、
前回のレスポンス取りこぼしによるオーファン注文を探すために使う。
activeOK は ACTIVE 一覧を取得できたかどうか。取得できていない場合は
オーファンの有無を判定できないため、再発注そのものを見送る（二重売りを避けるフェイルセーフ）。
*/
type rolloverOrderIndex struct {
	completed map[string]bool
	active    []bitflyer.Order
	activeOK  bool
}

/*
buildRolloverOrderIndex は通貨ペアごとのCOMPLETED / ACTIVE注文一覧を取得して索引を作る。

ループ前に、ローリング対象に実際に含まれる通貨ペアについてのみ1回だけ取得する
（不要な一覧取得でレート制限を消費しないため）。
COMPLETED の取得に失敗した通貨ペアは索引に載せず、そのレコードは処理をスキップする
（約定済みかどうか分からないままキャンセルAPIを叩かないためのフェイルセーフ）。
ACTIVE の取得だけが失敗した場合は activeOK=false として索引に載せる。
通常のローリング（個別照会でキャンセル可否を確認する経路）は続行できるが、
オーファン確認が必要な再試行経路だけは見送られる。
*/
func buildRolloverOrderIndex(apiClient *bitflyer.APIClient, productCodes []string,
	summary *rolloverSellOrderSummary) map[string]*rolloverOrderIndex {

	indexes := make(map[string]*rolloverOrderIndex, len(productCodes))
	for _, productCode := range productCodes {
		orders, _, err := apiClient.GetChildOrdersAll(productCode, "COMPLETED")
		if err != nil {
			summary.failed++
			msg := fmt.Sprintf("🚨【rolloverSellOrder】COMPLETED一覧の取得に失敗したため %s のローリングをスキップします: err=%v",
				productCode, err)
			log.Println(msg)
			slackClient.PostMessage(msg, true)
			continue
		}
		completed := make(map[string]bool, len(orders))
		for _, order := range orders {
			completed[order.ChildOrderAcceptanceID] = true
		}

		index := &rolloverOrderIndex{completed: completed}
		activeOrders, _, activeErr := apiClient.GetChildOrdersAll(productCode, "ACTIVE")
		if activeErr != nil {
			msg := fmt.Sprintf("🚨【rolloverSellOrder】ACTIVE一覧の取得に失敗: product_code=%s err=%v "+
				"→ %s 付きレコードの再発注は、オーファン注文の有無を確認できないため見送ります",
				productCode, activeErr, models.RemarkRolloverPending)
			log.Println(msg)
			slackClient.PostMessage(msg, true)
		} else {
			index.active = activeOrders
			index.activeOK = true
		}

		indexes[productCode] = index
		log.Printf("【rolloverSellOrder】product_code:%s completed:%d active:%d activeOK:%v",
			productCode, len(completed), len(index.active), index.activeOK)
	}
	return indexes
}

/*
rolloverSizeEpsilon / rolloverPriceEpsilon はオーファン注文の同一性判定に使う許容誤差。

size はDBの double precision とAPIのJSON数値を比較するため、表現誤差を吸収する幅を取る
（最小取引単位 BTC 0.001 / ETH 0.01 に対して十分小さい）。
price は円単位の指値なので1円未満の差は同一とみなす。
*/
const (
	rolloverSizeEpsilon  = 1e-9
	rolloverPriceEpsilon = 0.5
)

/*
matchesRolloverOrder は取引所の注文が、ローリング対象レコードの再発注結果とみなせるかを返す。

再発注は product_code / price / size / side="SELL" を機械的に引き継ぐため、
この4点が一致する ACTIVE 注文は「前回の再発注が実際には成立していた」可能性がある。
旧注文そのもの（同じ order_id）は除外する。
*/
func matchesRolloverOrder(order bitflyer.Order, record models.SellOrderRecord) bool {
	if order.ChildOrderAcceptanceID == "" || order.ChildOrderAcceptanceID == record.OrderID {
		return false
	}
	if order.Side != "SELL" {
		return false
	}
	if record.ProductCode != "" && order.ProductCode != "" && order.ProductCode != record.ProductCode {
		return false
	}
	if math.Abs(order.Price-record.Price) > rolloverPriceEpsilon {
		return false
	}
	return math.Abs(order.Size-record.Size) <= rolloverSizeEpsilon
}

// findRolloverOrphanCandidates は再発注結果とみなせるACTIVE注文を抽出する（DBは参照しない純粋関数）。
func findRolloverOrphanCandidates(active []bitflyer.Order, record models.SellOrderRecord) []bitflyer.Order {
	candidates := make([]bitflyer.Order, 0, 1)
	for _, order := range active {
		if matchesRolloverOrder(order, record) {
			candidates = append(candidates, order)
		}
	}
	return candidates
}

/*
rolloverOrphanExpireDate はオーファン注文の有効期限(UTC)を返す。

取引所が返す expire_date をパースできない場合は、再発注に使う予定だった期限で代用する
（DBの expire_date が未設定のままだと次回のローリング判定から漏れるため、必ず値を入れる）。
*/
func rolloverOrphanExpireDate(order bitflyer.Order, minuteToExpire int) time.Time {
	if expire, err := utils.ParseBitflyerTime(order.ExpireDate); err == nil {
		return expire
	}
	log.Printf("【rolloverSellOrder】オーファン注文の expire_date をパースできませんでした(値:%q)。now+%d分で代用します",
		order.ExpireDate, minuteToExpire)
	return time.Now().UTC().Add(time.Duration(minuteToExpire) * time.Minute)
}

/*
resolveRolloverOrphan は「前回の再発注が実は成立していた」注文を探す。

前回 PlaceOrder のリクエストは取引所に届いたがレスポンスを取りこぼした場合、
DBには記録が無いまま取引所にだけ売り注文が残る。旧 order_id の個別照会では
見つからないため、確認せずに再発注すると同一ポジションに売り注文が2本並ぶ。
拘束されていない現物（手動保有分）があると2本とも約定しうるため、必ず確認する。

戻り値:
  - order != nil : オーファン注文が見つかった。再発注してはならない
  - proceed=false: 判定できなかった。再発注を見送る（フェイルセーフ）
  - order == nil かつ proceed=true: オーファンは無い。再発注してよい
*/
func resolveRolloverOrphan(index *rolloverOrderIndex, record models.SellOrderRecord) (*bitflyer.Order, bool) {
	if index == nil || !index.activeOK {
		msg := fmt.Sprintf("🚨【rolloverSellOrder】ACTIVE一覧が無いためオーファン注文を確認できません: %s "+
			"→ 二重売りを避けるため再発注せず次回リトライします", rolloverRecordContext(record))
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		return nil, false
	}

	for _, candidate := range findRolloverOrphanCandidates(index.active, record) {
		exists, err := models.OrderIDExists(models.TableSellOrders, candidate.ChildOrderAcceptanceID)
		if err != nil {
			msg := fmt.Sprintf("🚨【rolloverSellOrder】オーファン注文の照合(DB参照)に失敗: %s CandidateOrderID=%s err=%v "+
				"→ 二重売りを避けるため再発注せず次回リトライします",
				rolloverRecordContext(record), candidate.ChildOrderAcceptanceID, err)
			log.Println(msg)
			slackClient.PostMessage(msg, true)
			return nil, false
		}
		if exists {
			// DBに紐づく注文は他レコードのもの。オーファンではない
			continue
		}
		orphan := candidate
		return &orphan, true
	}
	return nil, true
}

/*
adoptRolloverOrphan はオーファン注文をDBへ取り込み、再発注せずにローリングを完了させる。

旧レコードのCANCELLED化と新レコードのINSERTは通常の巻き直しと同じトランザクションで行うため、
parentid が引き継がれ損益計算の紐付けも維持される。
*/
func adoptRolloverOrphan(record models.SellOrderRecord, orphan bitflyer.Order,
	minuteToExpire int, summary *rolloverSellOrderSummary) {

	newExpire := rolloverOrphanExpireDate(orphan, minuteToExpire)
	if err := models.RolloverSellOrder(record, orphan.ChildOrderAcceptanceID, record.Price, newExpire); err != nil {
		summary.failed++
		msg := fmt.Sprintf("🚨🚨【rolloverSellOrder】オーファン注文のDB取り込みに失敗: %s OrphanOrderID=%s err=%v "+
			"→ 取引所には売り注文が存在します。再発注はしていません。手動での確認をお願いします",
			rolloverRecordContextWithPrefix(record, "Old"), orphan.ChildOrderAcceptanceID, err)
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		return
	}

	summary.adopted++
	msg := fmt.Sprintf("🚨【rolloverSellOrder】前回の再発注は実際には成立していました（レスポンスの取りこぼし）。"+
		"二重売りを避けるため再発注せずDBへ取り込みます: %s OrphanOrderID=%s expire_date(UTC)=%s。"+
		"※この注文が手動で発注したものでないか、取引所のコンソールでご確認ください",
		rolloverRecordContextWithPrefix(record, "Old"), orphan.ChildOrderAcceptanceID, newExpire.Format(time.RFC3339))
	log.Println(msg)
	slackClient.PostMessage(msg, true)
}

// rolloverTargetProductCodes はローリング対象に含まれる通貨ペアを重複なく返す。
// COMPLETED一覧の取得対象を必要な通貨ペアだけに絞り、無駄なAPIリクエストを減らすために使う。
func rolloverTargetProductCodes(records []models.SellOrderRecord) []string {
	seen := make(map[string]bool, len(records))
	productCodes := make([]string, 0, len(records))
	for _, record := range records {
		if record.ProductCode == "" || seen[record.ProductCode] {
			continue
		}
		seen[record.ProductCode] = true
		productCodes = append(productCodes, record.ProductCode)
	}
	return productCodes
}

// markRolloverOrderFilled は約定が判明したレコードを FILLED に更新する。
func markRolloverOrderFilled(record models.SellOrderRecord, reason string, summary *rolloverSellOrderSummary) {
	if err := models.UpdateFilledOrder(record.OrderID); err != nil {
		summary.failed++
		msg := fmt.Sprintf("🚨【rolloverSellOrder】約定判明レコードのFILLED更新に失敗: %s reason=%s err=%v",
			rolloverRecordContext(record), reason, err)
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		return
	}
	summary.filled++
	msg := fmt.Sprintf("【rolloverSellOrder】%s: %s → FILLEDに更新（再発注せず）", reason, rolloverRecordContext(record))
	log.Println(msg)
	slackClient.PostMessage(msg, false)
}

/*
markRolloverPending は「キャンセルは成功したが再発注できていない」レコードにマーカーを付ける。

マーカーが付いたレコードは expireSweepJob の対象から外れるため、
再発注待ちの状態で勝手にCANCELLEDへ落とされることがなくなり、次回ジョブで再試行される。
*/
func markRolloverPending(record models.SellOrderRecord) {
	if err := models.AppendOrderRemark(models.TableSellOrders, record.OrderID, " "+models.RemarkRolloverPending); err != nil {
		msg := fmt.Sprintf("🚨【rolloverSellOrder】%s マーカーの付与に失敗: %s err=%v",
			models.RemarkRolloverPending, rolloverRecordContext(record), err)
		log.Println(msg)
		slackClient.PostMessage(msg, true)
	}
}

/*
rolloverOrderSnapshot は個別照会で得た注文の状態。

found=false は「取引所に注文が存在しない」ことを表す。
失効した注文もキャンセルした注文もBitflyerのAPIから完全に消えるため
（実測確認済み: キャンセル直後の個別照会は空配列 [] を返す）、found=false だけでは
失効なのかキャンセル成立なのかを区別できない。文脈と組み合わせて解釈すること。
*/
type rolloverOrderSnapshot struct {
	found           bool
	state           string  // ACTIVE / COMPLETED / CANCELED など
	size            float64 // 発注数量
	executedSize    float64 // 約定済み数量
	outstandingSize float64 // 未約定の残数量
}

/*
rolloverSizeScale は数量を丸める尺度（小数8桁）。

Bitflyer の数量は最小取引単位 0.001 BTC / 0.01 ETH で、APIが扱う精度も8桁で十分足りる。
float64 の減算で生じる表現誤差（例: 0.03 - 0.02 = 0.009999999999999998）が
最小取引単位を僅差で下回り、正当なローリングが毎日スキップされ続けるのを防ぐために使う。
*/
const rolloverSizeScale = 1e8

// roundRolloverSize は数量を小数8桁へ丸め、float64 の減算で生じる桁ノイズを落とす。
func roundRolloverSize(size float64) float64 {
	return math.Round(size*rolloverSizeScale) / rolloverSizeScale
}

/*
remainingSize はまだ売りに出せる数量を返す。

ACTIVE な注文では outstanding_size がそのまま残数量になるが、
CANCELED の注文では outstanding_size が0になり残数量は cancel_size 側に移る。
状態によらず正しい値を得るため、outstanding_size が0以下のときは size - executed_size で補う。

戻り値は必ず小数8桁へ丸める。size - executed_size は float64 の減算なので
0.03 - 0.02 = 0.009999999999999998 のような値になり、丸めないと
ETH の最小取引単位 0.01 を僅差で下回って「残数量が最小取引単位未満」と誤判定される
（実損は無いがローリングが永久にスキップされ、いずれ期限切れで裸の保有になる）。
*/
func (s rolloverOrderSnapshot) remainingSize() float64 {
	if s.outstandingSize > 0 {
		return roundRolloverSize(s.outstandingSize)
	}
	return roundRolloverSize(s.size - s.executedSize)
}

/*
lookupRolloverOrder は個別照会で注文の現在の状態と数量を取得する。

数量まで取得するのは部分約定を検出するためである。
**この照会はキャンセルを実行する前に行わなければならない。**
キャンセルした注文はAPIから即座に消えるため、キャンセル後に executed_size /
outstanding_size を確認する手段は原理的に存在しない（実測確認済み）。
*/
func lookupRolloverOrder(apiClient *bitflyer.APIClient, productCode, orderID string) (rolloverOrderSnapshot, error) {
	order, err := apiClient.GetChildOrderByAcceptanceID(productCode, orderID)
	if err != nil {
		return rolloverOrderSnapshot{}, err
	}
	if order == nil {
		return rolloverOrderSnapshot{}, nil
	}
	return rolloverOrderSnapshot{
		found:           true,
		state:           order.ChildOrderState,
		size:            order.Size,
		executedSize:    order.ExecutedSize,
		outstandingSize: order.OutstandingSize,
	}, nil
}

/*
placeRolloverSellOrder は旧レコードと同一条件で売り注文を再発注する。

product_code / price / size / side="SELL" を旧レコードから機械的に引き継ぎ、価格の再計算は行わない。
部分約定していた場合の size は、キャンセル前に残数量へ補正済みのものが渡ってくる。
成功した場合は新しい child_order_acceptance_id を返す。
*/
func placeRolloverSellOrder(apiClient *bitflyer.APIClient, record models.SellOrderRecord, minuteToExpire int) (string, error) {
	sellOrder := &bitflyer.Order{
		ProductCode:     record.ProductCode,
		ChildOrderType:  "LIMIT",
		Side:            "SELL",
		Price:           record.Price,
		Size:            record.Size,
		MinuteToExpires: minuteToExpire,
		TimeInForce:     "GTC",
	}
	log.Printf("【rolloverSellOrder】再発注: %s minute_to_expire=%d", rolloverRecordContext(record), minuteToExpire)

	res, err := apiClient.PlaceOrder(sellOrder)
	if err != nil {
		return "", fmt.Errorf("failed to call PlaceOrder: %w", err)
	}
	if res == nil {
		return "", fmt.Errorf("no response from PlaceOrder")
	}
	if res.Status != 0 || res.OrderId == "" {
		return "", fmt.Errorf("PlaceOrder rejected. status=%d errorMessage=%s", res.Status, res.ErrorMessage)
	}
	return res.OrderId, nil
}

/*
adjustSizeForPartialFill は部分約定していた売り注文の数量を残数量へ補正する。

方針: 残っている数量だけを売りに出す（既に売れた分を重複して売りに出さない）。
残数量が最小取引単位を下回る場合は、キャンセルしても再発注できず裸の保有になるため
キャンセルを実行せずスキップする。

戻り値 proceed が false のときは、呼び出し側はこのレコードの処理を中止すること。
補正はキャンセルを実行する前にDBへ反映する。再発注に失敗して翌日へ持ち越された場合、
キャンセル済みの注文はAPIから消えていて残数量を再取得できないためである。
*/
/*
partialFillSkipMessage は「部分約定で残数量が最小取引単位を下回るためローリングしない」通知文を作る。

注文の現状は snapshot の状態で分岐させる。この関数は取引所側で ACTIVE な注文からも、
既に CANCELED / EXPIRED / REJECTED になった注文からも呼ばれるためである。

  - ACTIVE: キャンセルしなければ売り注文は生きている。「注文はそのまま残す」が実態
  - それ以外: 取引所側に注文はもう存在せず、現物だけが手元に残る「裸の保有」が実態。
    ここで「注文はそのまま残します」と通知すると、ユーザーが
    「まだ売り注文が生きている」と誤認して対応が遅れる
*/
func partialFillSkipMessage(record models.SellOrderRecord, snapshot rolloverOrderSnapshot,
	minSize, originalSize, remaining float64) string {

	if snapshot.found && snapshot.state == "ACTIVE" {
		return fmt.Sprintf("🚨【rolloverSellOrder】部分約定により残数量が最小取引単位(%v)未満のためローリングしません: "+
			"%s 元size=%v executed=%v outstanding=%v 残数量=%v。"+
			"キャンセルすると裸の保有になり再発注もできないため、注文はそのまま残します（手動での対応をお願いします）",
			minSize, rolloverRecordContext(record), originalSize, snapshot.executedSize, snapshot.outstandingSize, remaining)
	}
	return fmt.Sprintf("🚨🚨【rolloverSellOrder】部分約定により残数量が最小取引単位(%v)未満のためローリングしません: "+
		"%s 元size=%v executed=%v outstanding=%v 残数量=%v。"+
		"取引所側の注文は既に存在しません(found=%v state=%q)。売り注文が無いまま現物だけが残る「裸の保有」の状態で、"+
		"残数量が最小取引単位未満のため再発注もできません（手動での対応をお願いします）",
		minSize, rolloverRecordContext(record), originalSize, snapshot.executedSize, snapshot.outstandingSize, remaining,
		snapshot.found, snapshot.state)
}

func adjustSizeForPartialFill(record *models.SellOrderRecord, snapshot rolloverOrderSnapshot,
	summary *rolloverSellOrderSummary) (proceed bool) {

	if !snapshot.found || snapshot.executedSize <= 0 {
		return true
	}

	originalSize := record.Size
	remaining := snapshot.remainingSize()
	minSize, hasMinSize := rolloverMinOrderSize[record.ProductCode]

	if remaining <= 0 || (hasMinSize && remaining < minSize) {
		summary.skipped++
		msg := partialFillSkipMessage(*record, snapshot, minSize, originalSize, remaining)
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		return false
	}

	// キャンセル前にDBの size を残数量へ合わせる。以降の再発注・INSERTはこの値を使う
	remark := fmt.Sprintf(" / partially filled: size %v -> %v (executed=%v) at %s",
		originalSize, remaining, snapshot.executedSize, time.Now().UTC().Format(time.RFC3339))
	if err := models.UpdateOrderSizeWithRemark(models.TableSellOrders, record.OrderID, remaining, remark); err != nil {
		summary.failed++
		msg := fmt.Sprintf("🚨【rolloverSellOrder】部分約定の数量補正(DB更新)に失敗: %s 元size=%v executed=%v 残数量=%v err=%v "+
			"→ キャンセルせず次回リトライします",
			rolloverRecordContext(*record), originalSize, snapshot.executedSize, remaining, err)
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		return false
	}
	record.Size = remaining

	summary.partiallyFilled++
	msg := fmt.Sprintf("🚨【rolloverSellOrder】部分約定を検出。残数量で再発注します: %s 元size=%v executed=%v outstanding=%v 再発注size=%v",
		rolloverRecordContext(*record), originalSize, snapshot.executedSize, snapshot.outstandingSize, remaining)
	log.Println(msg)
	slackClient.PostMessage(msg, true)
	return true
}

/*
rolloverOneSellOrder は1件の売り注文を巻き直す。

エラーは呼び出し側へ伝播させず、この関数の中でSlack通知と集計まで行う。
ジョブ全体を止めないため（1件の失敗で他のレコードを巻き添えにしない）。
*/
func rolloverOneSellOrder(apiClient *bitflyer.APIClient, record models.SellOrderRecord,
	index *rolloverOrderIndex, minuteToExpire int, summary *rolloverSellOrderSummary) {

	// ① 処理前の約定確認: 既に約定していればキャンセルAPIを叩かずFILLEDに更新する
	if index.completed[record.OrderID] {
		markRolloverOrderFilled(record, "処理前に約定を検出", summary)
		return
	}

	// DBに桁ノイズ付きの数量（過去の部分約定補正の残り等）が入っていても、
	// 最小取引単位の判定と再発注の数量が誤らないよう小数8桁へ丸めておく。
	// record は値渡しなので、以降の再発注・INSERTにもこの値が使われる
	record.Size = roundRolloverSize(record.Size)

	// 最小取引単位を下回る注文は再発注が拒否される。キャンセルする前に弾いて裸の保有を作らない
	if minSize, ok := rolloverMinOrderSize[record.ProductCode]; ok && record.Size < minSize {
		summary.skipped++
		msg := fmt.Sprintf("🚨【rolloverSellOrder】最小取引単位(%v)未満のためローリングしません: %s",
			minSize, rolloverRecordContext(record))
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		return
	}

	// ②の前に個別照会を行い、状態と数量（executed_size / outstanding_size）を取得する。
	// キャンセルした注文はAPIから即座に消えるため、部分約定の検出は「キャンセル前」でしか行えない
	alreadyCancelled := strings.Contains(record.Remarks, models.RemarkRolloverPending)
	snapshot, err := lookupRolloverOrder(apiClient, record.ProductCode, record.OrderID)
	if err != nil {
		summary.failed++
		msg := fmt.Sprintf("🚨【rolloverSellOrder】キャンセル前の個別照会に失敗: %s err=%v "+
			"→ キャンセルも再発注も行わず次回リトライします", rolloverRecordContext(record), err)
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		return
	}

	skipCancel := false
	switch {
	case !snapshot.found:
		if !alreadyCancelled {
			// 取引所に注文が存在しない。失効または取引所側での手動キャンセルが考えられる。
			// キャンセルできない以上ローリングもできないため、DBの後始末は expireSweepJob に委ねる
			summary.skipped++
			msg := fmt.Sprintf("🚨【rolloverSellOrder】取引所に注文が存在しないためローリングしません: %s "+
				"（失効または取引所側でのキャンセルの可能性。DBの後始末は expireSweepJob に委ねます）",
				rolloverRecordContext(record))
			log.Println(msg)
			slackClient.PostMessage(msg, true)
			return
		}
		// 前回のキャンセルは成立済み。再発注へ直行する。
		// 数量は前回のキャンセル前に残数量へ補正済みなのでDBの値をそのまま使う
		skipCancel = true
		log.Printf("【rolloverSellOrder】%s 付きレコード。キャンセル成立済みのため再発注へ進みます: %s",
			models.RemarkRolloverPending, rolloverRecordContext(record))
	case snapshot.state == "COMPLETED":
		markRolloverOrderFilled(record, "キャンセル前の個別照会で約定を検出", summary)
		return
	case snapshot.state == "ACTIVE":
		if !adjustSizeForPartialFill(&record, snapshot, summary) {
			return
		}
	default:
		// CANCELED / EXPIRED / REJECTED 等。キャンセルは不要だが部分約定の可能性は確認する
		if !adjustSizeForPartialFill(&record, snapshot, summary) {
			return
		}
		skipCancel = true
		log.Printf("【rolloverSellOrder】取引所側で既にキャンセル済み(state=%q)。再発注へ進みます: %s",
			snapshot.state, rolloverRecordContext(record))
	}

	if !skipCancel {
		// ② キャンセルを実行し、成功を確認する（CancelOrderはレスポンスを検証して成否をerrorで返す）
		cancelParam := &bitflyer.Order{
			ProductCode:            record.ProductCode,
			ChildOrderAcceptanceID: record.OrderID,
		}
		if err := apiClient.CancelOrder(cancelParam); err != nil {
			summary.failed++
			msg := fmt.Sprintf("🚨【rolloverSellOrder】CancelOrder 失敗: %s err=%v （再発注せず次回リトライします）",
				rolloverRecordContext(record), err)
			log.Println(msg)
			slackClient.PostMessage(msg, true)
			return
		}

		// ③ 個別照会で消滅（またはCOMPLETED）を再確認する
		afterCancel, err := lookupRolloverOrder(apiClient, record.ProductCode, record.OrderID)
		if err != nil {
			// キャンセルは成功しているため裸の保有になっている可能性がある。
			// 状態を確認できないまま再発注すると二重売りのリスクがあるので、ここでは発注しない
			summary.failed++
			markRolloverPending(record)
			msg := fmt.Sprintf("🚨🚨【rolloverSellOrder】キャンセル後の個別照会に失敗: %s err=%v "+
				"→ 再発注せず %s を付けて次回リトライします（現物が裸の保有になっている可能性があります）",
				rolloverRecordContext(record), err, models.RemarkRolloverPending)
			log.Println(msg)
			slackClient.PostMessage(msg, true)
			return
		}
		switch {
		case afterCancel.found && afterCancel.state == "COMPLETED":
			// キャンセル直前に約定していた。再発注すると二重売りになるため発注しない
			markRolloverOrderFilled(record, "キャンセル直前に約定", summary)
			return
		case afterCancel.found && afterCancel.state == "ACTIVE":
			// キャンセルAPIは成功を返したが注文が生存している。再発注すると二重の売り注文になる
			summary.failed++
			msg := fmt.Sprintf("🚨【rolloverSellOrder】キャンセル未成立（個別照会でACTIVE）: %s "+
				"→ 再発注せず次回リトライします", rolloverRecordContext(record))
			log.Println(msg)
			slackClient.PostMessage(msg, true)
			return
		default:
			// 消滅（実測: キャンセル直後の個別照会は []）または CANCELED。キャンセル成立とみなす
			log.Printf("【rolloverSellOrder】キャンセル成立を確認(found=%v state=%q): %s",
				afterCancel.found, afterCancel.state, rolloverRecordContext(record))
		}
	}

	// 再試行経路（前回キャンセル成功・再発注失敗）では、④の前に必ずオーファン注文を確認する。
	// 前回の PlaceOrder がレスポンスを取りこぼしただけで成立していた場合、
	// 確認せずに再発注すると同一ポジションに売り注文が2本並ぶ（二重売り）
	if alreadyCancelled {
		orphan, proceed := resolveRolloverOrphan(index, record)
		if !proceed {
			summary.failed++
			return
		}
		if orphan != nil {
			adoptRolloverOrphan(record, *orphan, minuteToExpire, summary)
			return
		}
	}

	// ④ 同一 product_code / price / size（部分約定時は残数量）で再発注する
	newOrderID, err := placeRolloverSellOrder(apiClient, record, minuteToExpire)
	if err != nil {
		// キャンセル成功・再発注失敗 = 現物を保有しているのに売り注文が無い「裸の保有」
		summary.failed++
		markRolloverPending(record)
		msg := fmt.Sprintf("🚨🚨【rolloverSellOrder】裸の保有が発生: %s err=%v "+
			"→ DBは変更せず %s を付けて次回リトライします",
			rolloverRecordContextWithPrefix(record, "Old"), err, models.RemarkRolloverPending)
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		return
	}

	// ⑤ 旧レコードのCANCELLED化と新レコードのINSERTを単一トランザクションで実行する
	newExpire := time.Now().UTC().Add(time.Duration(minuteToExpire) * time.Minute)
	if err := models.RolloverSellOrder(record, newOrderID, record.Price, newExpire); err != nil {
		summary.failed++
		msg := fmt.Sprintf("🚨🚨【rolloverSellOrder】再発注は成功しましたがDB更新に失敗: %s NewOrderID=%s err=%v "+
			"→ 取引所には新しい売り注文が存在します。手動での確認をお願いします",
			rolloverRecordContextWithPrefix(record, "Old"), newOrderID, err)
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		return
	}

	summary.succeeded++
	msg := fmt.Sprintf("【rolloverSellOrder】巻き直し成功: %s → NewOrderID=%s expire_date(UTC)=%s",
		rolloverRecordContextWithPrefix(record, "Old"), newOrderID, newExpire.Format(time.RFC3339))
	log.Println(msg)
	slackClient.PostMessage(msg, false)
}

/*
rolloverSellOrderJob はスケジューラから呼ばれるエントリポイント。

スケジューラの時刻指定はシステムTZ（本番はJST）に依存するため、
TZの誤設定を運用で検知できるようジョブ冒頭でローカル時刻とUTCの双方をログ出力する。
期限の判定・保存はすべてUTCで行う。
*/
func rolloverSellOrderJob(apiClient *bitflyer.APIClient) {
	now := time.Now().UTC()
	log.Printf("【rolloverSellOrder】start of job now(local):%s now(UTC):%s",
		time.Now().Format(time.RFC3339), now.Format(time.RFC3339))

	daysBefore := rolloverDaysBeforeExpire()
	fallbackDays := rolloverFallbackDays()
	maxPerRun := rolloverMaxPerRun()
	minuteToExpire := rolloverSellMinuteToExpire()
	summary := &rolloverSellOrderSummary{}

	records, err := models.GetSellOrdersToRollover(now, daysBefore, fallbackDays, maxPerRun)
	if err != nil {
		msg := fmt.Sprintf("🚨【rolloverSellOrder】ローリング対象の取得に失敗: daysBefore=%d fallbackDays=%d limit=%d err=%v",
			daysBefore, fallbackDays, maxPerRun, err)
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		log.Println("【rolloverSellOrder】end of job as error")
		return
	}
	summary.target = len(records)
	log.Printf("【rolloverSellOrder】対象:%d件 daysBefore:%d fallbackDays:%d limit:%d minuteToExpire:%d",
		summary.target, daysBefore, fallbackDays, maxPerRun, minuteToExpire)

	if summary.target == 0 {
		log.Println("【rolloverSellOrder】ローリング対象なし")
		log.Println("【rolloverSellOrder】end of job")
		return
	}

	// 処理前の約定確認とオーファン確認に使う注文一覧を、対象に含まれる通貨ペアぶんだけ取得する
	indexes := buildRolloverOrderIndex(apiClient, rolloverTargetProductCodes(records), summary)

	for _, record := range records {
		index, ok := indexes[record.ProductCode]
		if !ok {
			// COMPLETED一覧を取得できなかった通貨ペアは約定済みか判定できないためキャンセルしない
			summary.skipped++
			log.Printf("【rolloverSellOrder】COMPLETED一覧未取得のためスキップ: %s", rolloverRecordContext(record))
			continue
		}
		// 1件の失敗で全体を止めない（breakせずcontinueする）
		rolloverOneSellOrder(apiClient, record, index, minuteToExpire, summary)
		// レート制限に配慮して1件ごとに間隔をあける
		time.Sleep(rolloverAPIInterval)
	}

	msg := fmt.Sprintf("【rolloverSellOrder】対象:%d 巻き直し成功:%d 約定判明:%d 部分約定(残数量で再発注):%d オーファン取込:%d スキップ:%d 失敗:%d",
		summary.target, summary.succeeded, summary.filled, summary.partiallyFilled, summary.adopted,
		summary.skipped, summary.failed)
	log.Println(msg)
	slackClient.PostMessage(msg, false)
	log.Println("【rolloverSellOrder】end of job")
}
