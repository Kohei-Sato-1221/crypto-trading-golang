package enums

// 20251213からはBuyPriceStrategyを利用
type BuyPriceStrategy int

const (
	StrategyLTP99 = 10001 // ltp * 0.99（-1%・毎日）
	StrategyLTP98 = 10002 // ltp * 0.98（-2%・火/土）
	// StrategyLTP95 は約定率が低く曜日割当から外したが、過去データに値が残っているため定数は残す
	StrategyLTP95 = 10003 // ltp * 0.95（-5%・廃止）
	StrategyLTP97 = 10004 // ltp * 0.97（-3%・月）

	StrategyLtpLowestIn7days5t5 = 20001 // ltp*0.5 + 7日安値*0.5（水）
	// StrategyLtpLowestIn7days2t8 は深すぎて約定しないため曜日割当から外したが、
	// 過去データに値が残っているため定数は残す
	StrategyLtpLowestIn7days2t8 = 20002 // ltp*0.2 + 7日安値*0.8（廃止）
	StrategyLtpLowestIn7days7t3 = 20003 // ltp*0.7 + 7日安値*0.3（日）
)

/*
戦略値として記録されるが、ボットの買い戦略ではない値。

戦略別の集計（約定率・実現利益）では StrategyUnknown / StrategySaturatedUnknown を除外する。
いずれも過去データの実値であり、遡及UPDATEによる書き換えは行わない（復元不能なため）。
*/
const (
	// StrategyUnknown はDBのデフォルト値。戦略が記録されていないレコード（2026-09-06 時点で208件）。
	StrategyUnknown = 99

	// StrategySaturatedUnknown は旧MySQL(RDS)時代の tinyint により
	// 戦略値 10001-20003 が127に飽和し、判別不能になったレコード（2025-12〜2026-05の472件）。
	// 現行の本番PostgreSQLでは integer のため飽和は発生しない。
	StrategySaturatedUnknown = 127

	// StrategyManual は取引所で手動発注され、syncBuyOrders が取り込んだ注文を表す。
	StrategyManual = 90001

	/*
		StrategyZeroValue は strategy カラムのゼロ値。

		旧戦略 Stg0BtcLtp3low7 の値(iota = 0)と一致してしまうため、
		「値が入っていない(NULL・スキャン失敗・DEFAULT未設定)」のか
		「旧戦略で発注された」のかを区別できない。ボット発注と誤認すると
		reconcileJob の発注ゼロ検知を取りこぼすため、ボット戦略としては扱わない。
	*/
	StrategyZeroValue = 0
)

// 以下は古い戦略2：0251213からはBuyPriceStrategyを利用
type BTCStrategy int

const TEST_STG = -1

const (
	Stg0BtcLtp3low7 = iota
	Stg1BtcLtp997
	Stg2BtcLtp98
	Stg3BtcLtp90
)

// 以下は古い戦略2：0251213からはBuyPriceStrategyを利用
type ETHStrategy int

const (
	Stg10EthLtp995 = iota + 10
	Stg11EthLtp98
	Stg12EthLtp97
	Stg13EthLtp3low7
	Stg14EthLtp90
)

/*
利確率（売り指値 = 買値 × 利確率）の戦略別マッピング。

買い指値が深い戦略ほど取得単価が安く、より大きな利確幅を狙えるため、
戦略ごとに利確率を変える。実データ（2025-12〜2026-05）による評価では
利確到達率（30日以内）は +1.5%:80% / +3%:71-73% / +5%:56-67% だった。

新旧すべての戦略値をこの1箇所に集約する。マッピングに存在しない戦略値
（StrategyUnknown=99 / StrategySaturatedUnknown=127 / StrategyManual=90001、
および旧戦略のうち浅い指値のもの）は、従来どおり SellProfitRateDefault にフォールバックする。
*/

// SellProfitRateDefault はマッピングに定義がない戦略値に適用する利確率（従来の挙動）。
const SellProfitRateDefault = 1.015

// sellProfitRates は戦略値 -> 利確率のマッピング。
// 追加・変更はこのマップだけを編集すること。
var sellProfitRates = map[int]float64{
	// 現行の買い戦略（BuyPriceStrategy）
	StrategyLTP99:               1.02, // ltp*0.99（-1%）→ +2%
	StrategyLTP98:               1.03, // ltp*0.98（-2%）→ +3%
	StrategyLTP97:               1.05, // ltp*0.97（-3%）→ +5%
	StrategyLTP95:               1.05, // ltp*0.95（-5%・廃止済みだが過去データに残る）→ +5%
	StrategyLtpLowestIn7days5t5: 1.05, // ltp*0.5 + 7日安値*0.5 → +5%
	StrategyLtpLowestIn7days7t3: 1.05, // ltp*0.7 + 7日安値*0.3 → +5%
	StrategyLtpLowestIn7days2t8: 1.05, // ltp*0.2 + 7日安値*0.8（廃止済みだが過去データに残る）→ +5%

	// 旧戦略（BTCStrategy / ETHStrategy）。ltp*0.90 の深指値のみ従来から +3% を適用しており、
	// その挙動を維持する。それ以外の旧戦略は浅い指値のため既定値にフォールバックさせる。
	Stg3BtcLtp90:  1.03, // ltp*0.90（-10%）
	Stg14EthLtp90: 1.03, // ltp*0.90（-10%）
}

/*
SellProfitRate は戦略値に対応する利確率を返す。
第2返り値はマッピングに定義があったかを示し、false の場合は
SellProfitRateDefault にフォールバックしている（ログ・通知での識別に使う）。
*/
func SellProfitRate(strategy int) (float64, bool) {
	if rate, ok := sellProfitRates[strategy]; ok {
		return rate, true
	}
	return SellProfitRateDefault, false
}

/*
botStrategies はボットが発注した買い注文の戦略値の集合。

reconcileJob の「N日間ボットの発注が0件」検知で、手動発注(StrategyManual)や
戦略未記録(StrategyUnknown / StrategySaturatedUnknown)のレコードを
ボット発注と誤認しないために使う。新旧すべてのボット戦略を列挙する。
*/
var botStrategies = map[int]bool{
	// 現行の買い戦略（BuyPriceStrategy）
	StrategyLTP99:               true,
	StrategyLTP98:               true,
	StrategyLTP95:               true,
	StrategyLTP97:               true,
	StrategyLtpLowestIn7days5t5: true,
	StrategyLtpLowestIn7days2t8: true,
	StrategyLtpLowestIn7days7t3: true,

	// 旧戦略（BTCStrategy / ETHStrategy）。過去データにのみ存在する。
	// Stg0BtcLtp3low7 は値が 0（iota）で strategy カラムのゼロ値と区別できないため
	// 意図的に含めない（詳細は IsBotStrategy のコメントを参照）
	Stg1BtcLtp997:    true,
	Stg2BtcLtp98:     true,
	Stg3BtcLtp90:     true,
	Stg10EthLtp995:   true,
	Stg11EthLtp98:    true,
	Stg12EthLtp97:    true,
	Stg13EthLtp3low7: true,
	Stg14EthLtp90:    true,
}

/*
IsBotStrategy はボットが発注した戦略値かどうかを返す。

StrategyUnknown(99) / StrategySaturatedUnknown(127) / StrategyManual(90001) は
いずれも「ボットの買い戦略として記録された値」ではないため false を返す。

0(StrategyZeroValue) も false を返す。旧戦略 Stg0BtcLtp3low7 の値と一致するが、
strategy カラムのゼロ値（未設定・NULL・スキャン失敗）とも区別がつかない。
ゼロ値を「ボット発注」と数えると reconcileJob の発注ゼロ検知を取りこぼし、
ボットが止まっていることに気づけなくなる。取りこぼすより鳴らす側へ倒す。
本番データ上の strategy=0 は2021年頃の5件のみで、発注ゼロ検知が走査する
直近200件には含まれないため、この除外による判定の変化はない。
*/
func IsBotStrategy(strategy int) bool {
	if strategy == StrategyZeroValue {
		return false
	}
	return botStrategies[strategy]
}
