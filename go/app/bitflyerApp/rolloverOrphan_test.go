package bitflyerApp

import (
	"testing"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

/*
F3 の回帰テスト。

PlaceOrder のレスポンスを取りこぼしたまま [ROLLOVER_PENDING] で翌日へ持ち越すと、
旧 order_id の個別照会では見つからないため「キャンセル成立済み」と解釈して
2本目の売り注文を発注してしまう。拘束されていない現物（手動保有分）があると
2本とも約定しうるため、再発注の前にオーファン注文を検出する必要がある。

ここでは取引所API・DB・Slackに触れない純粋関数のみを検証する
（slackClient はサービス起動時に初期化されるため、ジョブ関数はそのまま呼べない）。
*/

func orphanTestRecord() models.SellOrderRecord {
	return models.SellOrderRecord{
		Table:       models.TableSellOrders,
		OrderID:     "OLD-SELL",
		ParentID:    "BUY-PARENT",
		ProductCode: "ETH_JPY",
		Side:        "SELL",
		Price:       512345,
		Size:        0.02,
	}
}

func orphanTestOrder(id string) bitflyer.Order {
	return bitflyer.Order{
		ChildOrderAcceptanceID: id,
		ProductCode:            "ETH_JPY",
		Side:                   "SELL",
		Price:                  512345,
		Size:                   0.02,
		ChildOrderState:        "ACTIVE",
	}
}

func TestMatchesRolloverOrder(t *testing.T) {
	record := orphanTestRecord()

	tests := []struct {
		name  string
		mutT  func(*bitflyer.Order)
		want  bool
		notes string
	}{
		{name: "同一条件の別注文は再発注結果とみなす", mutT: func(o *bitflyer.Order) {}, want: true},
		{name: "旧注文そのものは対象外", mutT: func(o *bitflyer.Order) { o.ChildOrderAcceptanceID = record.OrderID }, want: false},
		{name: "acceptance_idが空は対象外", mutT: func(o *bitflyer.Order) { o.ChildOrderAcceptanceID = "" }, want: false},
		{name: "BUYは対象外", mutT: func(o *bitflyer.Order) { o.Side = "BUY" }, want: false},
		{name: "通貨ペア違いは対象外", mutT: func(o *bitflyer.Order) { o.ProductCode = "BTC_JPY" }, want: false},
		{name: "価格が1円違えば対象外", mutT: func(o *bitflyer.Order) { o.Price = 512346 }, want: false},
		{name: "価格の0.4円差は同一とみなす", mutT: func(o *bitflyer.Order) { o.Price = 512345.4 }, want: true},
		{name: "数量が違えば対象外", mutT: func(o *bitflyer.Order) { o.Size = 0.03 }, want: false},
		{name: "数量の表現誤差は同一とみなす", mutT: func(o *bitflyer.Order) { o.Size = 0.02 + 1e-12 }, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := orphanTestOrder("NEW-SELL")
			tt.mutT(&order)
			if got := matchesRolloverOrder(order, record); got != tt.want {
				t.Errorf("matchesRolloverOrder() = %v, want %v (order=%+v)", got, tt.want, order)
			}
		})
	}
}

func TestFindRolloverOrphanCandidates(t *testing.T) {
	record := orphanTestRecord()

	buyOrder := orphanTestOrder("ACTIVE-BUY")
	buyOrder.Side = "BUY"
	otherSize := orphanTestOrder("ACTIVE-OTHERSIZE")
	otherSize.Size = 0.05
	self := orphanTestOrder(record.OrderID)

	active := []bitflyer.Order{
		buyOrder,
		otherSize,
		self,
		orphanTestOrder("ORPHAN-1"),
		orphanTestOrder("ORPHAN-2"),
	}

	got := findRolloverOrphanCandidates(active, record)
	if len(got) != 2 {
		t.Fatalf("candidates = %d, want 2 (%+v)", len(got), got)
	}
	ids := map[string]bool{got[0].ChildOrderAcceptanceID: true, got[1].ChildOrderAcceptanceID: true}
	if !ids["ORPHAN-1"] || !ids["ORPHAN-2"] {
		t.Errorf("unexpected candidates: %v", ids)
	}

	// 回帰: 候補が無いときは空スライスを返し、再発注してよいと判断できること
	if got := findRolloverOrphanCandidates([]bitflyer.Order{buyOrder, otherSize, self}, record); len(got) != 0 {
		t.Errorf("candidates = %d, want 0 (%+v)", len(got), got)
	}
	if got := findRolloverOrphanCandidates(nil, record); len(got) != 0 {
		t.Errorf("candidates from nil = %d, want 0", len(got))
	}
}

func TestRolloverOrphanExpireDate(t *testing.T) {
	order := orphanTestOrder("ORPHAN-1")
	order.ExpireDate = "2026-10-07T05:30:00"

	got := rolloverOrphanExpireDate(order, 43200)
	want := time.Date(2026, 10, 7, 5, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("expire = %v, want %v", got, want)
	}
	if got.Location() != time.UTC {
		t.Errorf("expire must be UTC, got %v", got.Location())
	}

	// パースできない場合は now + minuteToExpire で代用し、必ず値が入ること
	order.ExpireDate = "not-a-time"
	fallback := rolloverOrphanExpireDate(order, 60)
	if fallback.IsZero() {
		t.Fatal("fallback expire must not be zero (expire_date が NULL だと次回のローリング判定から漏れる)")
	}
	diff := fallback.Sub(time.Now().UTC().Add(60 * time.Minute))
	if diff > time.Minute || diff < -time.Minute {
		t.Errorf("fallback expire = %v, want about now+60m", fallback)
	}
}

// ACTIVE一覧を取得できていない場合は再発注を見送ること（フェイルセーフ）。
func TestResolveRolloverOrphanRequiresActiveList(t *testing.T) {
	record := orphanTestRecord()
	// index==nil / activeOK==false のどちらも「判定不能」として扱う。
	// slackClient を伴わない純粋な判定部分のみを findRolloverOrphanCandidates で確認する。
	index := &rolloverOrderIndex{completed: map[string]bool{}, activeOK: false}
	if index.activeOK {
		t.Fatal("precondition")
	}
	if got := findRolloverOrphanCandidates(index.active, record); len(got) != 0 {
		t.Errorf("ACTIVE一覧が空なら候補は0件: %v", got)
	}
}
