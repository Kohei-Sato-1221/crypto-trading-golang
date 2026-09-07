package tests

import (
	"strings"
	"testing"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

func insertRolloverSellOrder(t *testing.T, parentID, orderID, productCode string, price, size float64,
	status string, remarks *string, expireDate *time.Time, timestamp time.Time) {
	t.Helper()
	_, err := models.AppDB.Exec(
		`INSERT INTO sell_orders (parentid, order_id, product_code, side, price, size, exchange, status, remarks, expire_date, timestamp)
		 VALUES ($1,$2,$3,'SELL',$4,$5,'bitflyer',$6,$7,$8,$9)`,
		parentID, orderID, productCode, price, size, status, remarks, expireDate, timestamp)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
}

func TestGetSellOrdersToRollover(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	pending := " " + models.RemarkRolloverPending
	manual := " [MANUAL_HOLD]"

	exp27 := now.AddDate(0, 0, 2)  // 期限まで2日 -> 対象(3日前判定)
	exp10 := now.AddDate(0, 0, 10) // 期限まで10日 -> 対象外
	insertRolloverSellOrder(t, "P1", "S-EXPSOON", "BTC_JPY", 5000000, 0.001, "UNFILLED", nil, &exp27, now.AddDate(0, 0, -28))
	insertRolloverSellOrder(t, "P2", "S-EXPFAR", "BTC_JPY", 5000000, 0.001, "UNFILLED", nil, &exp10, now.AddDate(0, 0, -20))
	// expire_date NULL のフォールバック
	insertRolloverSellOrder(t, "P3", "S-NULLOLD", "ETH_JPY", 500000, 0.01, "UNFILLED", nil, nil, now.AddDate(0, 0, -28))
	insertRolloverSellOrder(t, "P4", "S-NULLNEW", "ETH_JPY", 500000, 0.01, "UNFILLED", nil, nil, now.AddDate(0, 0, -5))
	// ROLLOVER_PENDING は除外しない（再試行対象）
	insertRolloverSellOrder(t, "P5", "S-PENDING", "BTC_JPY", 5000000, 0.001, "UNFILLED", &pending, &exp27, now.AddDate(0, 0, -28))
	// 手動保有(CANCELLED)は構造的に対象外
	insertRolloverSellOrder(t, "P6", "S-MANUAL", "BTC_JPY", 5000000, 0.001, "CANCELLED", &manual, nil, now.AddDate(0, 0, -100))
	// FILLED も対象外
	insertRolloverSellOrder(t, "P7", "S-FILLED", "BTC_JPY", 5000000, 0.001, "FILLED", nil, &exp27, now.AddDate(0, 0, -28))
	// 既に失効済み（expire_date < now）は対象外。expireSweepJob の担当
	expPast := now.AddDate(0, 0, -1)
	insertRolloverSellOrder(t, "P8", "S-EXPIRED", "BTC_JPY", 5000000, 0.001, "UNFILLED", nil, &expPast, now.AddDate(0, 0, -31))
	// expire_date NULL かつ想定寿命(27+3=30日)を超えて古いものは失効済みとみなして対象外
	insertRolloverSellOrder(t, "P9", "S-NULLTOOOLD", "ETH_JPY", 500000, 0.01, "UNFILLED", nil, nil, now.AddDate(0, 0, -40))

	records, err := models.GetSellOrdersToRollover(now, 3, 27, 20)
	if err != nil {
		t.Fatalf("GetSellOrdersToRollover failed: %v", err)
	}
	got := map[string]bool{}
	for _, r := range records {
		got[r.OrderID] = true
	}
	want := []string{"S-EXPSOON", "S-NULLOLD", "S-PENDING"}
	notWant := []string{"S-EXPFAR", "S-NULLNEW", "S-MANUAL", "S-FILLED", "S-EXPIRED", "S-NULLTOOOLD"}
	for _, w := range want {
		if !got[w] {
			t.Errorf("expected %s to be a rollover target, got=%v", w, got)
		}
	}
	for _, n := range notWant {
		if got[n] {
			t.Errorf("did NOT expect %s to be a rollover target", n)
		}
	}
	if len(records) != 3 {
		t.Errorf("expected 3 records, got %d", len(records))
	}

	// limit の効きを確認
	limited, err := models.GetSellOrdersToRollover(now, 3, 27, 1)
	if err != nil || len(limited) != 1 {
		t.Errorf("limit not applied: len=%d err=%v", len(limited), err)
	}
}

func TestRolloverSellOrder(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	exp := now.AddDate(0, 0, 2)
	insertRolloverSellOrder(t, "BUY-PARENT-1", "OLD-SELL-1", "ETH_JPY", 512345, 0.02, "UNFILLED", nil, &exp, now.AddDate(0, 0, -28))

	records, err := models.GetSellOrdersToRollover(now, 3, 27, 20)
	if err != nil || len(records) != 1 {
		t.Fatalf("setup failed: len=%d err=%v", len(records), err)
	}
	old := records[0]
	newExpire := now.AddDate(0, 0, 30)

	if err := models.RolloverSellOrder(old, "NEW-SELL-1", old.Price, newExpire); err != nil {
		t.Fatalf("RolloverSellOrder failed: %v", err)
	}

	// 旧レコード
	var status, remarks string
	if err := models.AppDB.QueryRow(`SELECT status, COALESCE(remarks,'') FROM sell_orders WHERE order_id='OLD-SELL-1'`).Scan(&status, &remarks); err != nil {
		t.Fatalf("query old failed: %v", err)
	}
	if status != "CANCELLED" {
		t.Errorf("old status = %s, want CANCELLED", status)
	}
	if !strings.Contains(remarks, "rolled over to NEW-SELL-1") {
		t.Errorf("old remarks missing rollover note: %q", remarks)
	}

	// 新レコード
	var parentID, pc, side, newRemarks string
	var price, size float64
	var expireDate time.Time
	err = models.AppDB.QueryRow(
		`SELECT parentid, product_code, side, price, size, COALESCE(remarks,''), expire_date, status
		   FROM sell_orders WHERE order_id='NEW-SELL-1'`).
		Scan(&parentID, &pc, &side, &price, &size, &newRemarks, &expireDate, &status)
	if err != nil {
		t.Fatalf("query new failed: %v", err)
	}
	if parentID != "BUY-PARENT-1" {
		t.Errorf("parentid = %s, want BUY-PARENT-1 (損益計算の紐付け)", parentID)
	}
	if pc != "ETH_JPY" || side != "SELL" || price != 512345 || size != 0.02 {
		t.Errorf("new record mismatch: pc=%s side=%s price=%v size=%v", pc, side, price, size)
	}
	if newRemarks != "OLD-SELL-1の売り注文の再注文" {
		t.Errorf("new remarks = %q, want 'OLD-SELL-1の売り注文の再注文'", newRemarks)
	}
	if status != "UNFILLED" {
		t.Errorf("new status = %s, want UNFILLED", status)
	}
	if expireDate.UTC().Sub(newExpire).Abs() > time.Second {
		t.Errorf("expire_date = %v, want %v", expireDate.UTC(), newExpire)
	}

	// 新レコードは翌回のローリング対象にならない（期限30日先）
	next, err := models.GetSellOrdersToRollover(now, 3, 27, 20)
	if err != nil {
		t.Fatalf("re-query failed: %v", err)
	}
	if len(next) != 0 {
		t.Errorf("expected no rollover target after rollover, got %d (%v)", len(next), next)
	}
}

func TestRolloverSellOrderRollsBackWhenNotUnfilled(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	exp := now.AddDate(0, 0, 2)
	insertRolloverSellOrder(t, "BUY-PARENT-2", "OLD-SELL-2", "BTC_JPY", 5000000, 0.001, "UNFILLED", nil, &exp, now.AddDate(0, 0, -28))
	records, _ := models.GetSellOrdersToRollover(now, 3, 27, 20)
	old := records[0]

	// 他ジョブが FILLED にした状況を再現
	if _, err := models.AppDB.Exec(`UPDATE sell_orders SET status='FILLED' WHERE order_id='OLD-SELL-2'`); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	err := models.RolloverSellOrder(old, "NEW-SELL-2", old.Price, now.AddDate(0, 0, 30))
	if err == nil {
		t.Fatal("expected error when old record is no longer UNFILLED")
	}
	if !strings.Contains(err.Error(), "NEW-SELL-2") {
		t.Errorf("error should carry the new order id for manual follow-up: %v", err)
	}
	var cnt int
	models.AppDB.QueryRow(`SELECT COUNT(*) FROM sell_orders WHERE order_id='NEW-SELL-2'`).Scan(&cnt)
	if cnt != 0 {
		t.Errorf("new record must NOT be inserted when tx rolled back, got %d", cnt)
	}
}

func TestAppendOrderRemark(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	insertRolloverSellOrder(t, "P", "S-MARK", "BTC_JPY", 5000000, 0.001, "UNFILLED", nil, nil, now)

	marker := " " + models.RemarkRolloverPending
	for i := 0; i < 3; i++ {
		if err := models.AppendOrderRemark(models.TableSellOrders, "S-MARK", marker); err != nil {
			t.Fatalf("AppendOrderRemark failed: %v", err)
		}
	}
	var remarks string
	models.AppDB.QueryRow(`SELECT COALESCE(remarks,'') FROM sell_orders WHERE order_id='S-MARK'`).Scan(&remarks)
	if strings.Count(remarks, models.RemarkRolloverPending) != 1 {
		t.Errorf("marker must be appended only once: %q", remarks)
	}

	// マーカー付きレコードは expireSweep(方式B) の対象から外れる
	// ローリングがまだ再試行しうる窓の内側（timestamp=now）なので sweep からは外れる
	recs, err := models.GetUnfilledOrdersWithoutExpireDate(models.TableSellOrders, "BTC_JPY", now.AddDate(0, 0, -30), 10)
	if err != nil {
		t.Fatalf("GetUnfilledOrdersWithoutExpireDate failed: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("ROLLOVER_PENDING record must be excluded from sweep, got %d", len(recs))
	}
	// ローリングの対象には残る
	roll, err := models.GetSellOrdersToRollover(now.AddDate(0, 0, 28), 3, 27, 10)
	if err != nil {
		t.Fatalf("GetSellOrdersToRollover failed: %v", err)
	}
	if len(roll) != 1 {
		t.Errorf("ROLLOVER_PENDING record must remain a rollover target, got %d", len(roll))
	}
}

// 部分約定時にキャンセル前へDBのsizeを残数量へ補正できることを確認する。
// キャンセル済みの注文はAPIから消えて outstanding_size を再取得できないため、
// この事前補正が「再発注失敗 → 翌日リトライ」で元の過大な数量を再発注しないための担保になる。
func TestUpdateOrderSizeWithRemark(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	exp := now.AddDate(0, 0, 2)
	insertRolloverSellOrder(t, "BUY-PARENT-3", "OLD-SELL-3", "ETH_JPY", 512345, 0.05, "UNFILLED", nil, &exp, now.AddDate(0, 0, -28))

	remark := " / partially filled: size 0.05 -> 0.02 (executed=0.03)"
	if err := models.UpdateOrderSizeWithRemark(models.TableSellOrders, "OLD-SELL-3", 0.02, remark); err != nil {
		t.Fatalf("UpdateOrderSizeWithRemark failed: %v", err)
	}

	var size float64
	var remarks string
	models.AppDB.QueryRow(`SELECT size, COALESCE(remarks,'') FROM sell_orders WHERE order_id='OLD-SELL-3'`).Scan(&size, &remarks)
	if size != 0.02 {
		t.Errorf("size = %v, want 0.02", size)
	}
	if !strings.Contains(remarks, "partially filled") {
		t.Errorf("remarks missing partial fill note: %q", remarks)
	}

	// 補正後の size がローリング（＝再発注とINSERT）に引き継がれること
	records, err := models.GetSellOrdersToRollover(now, 3, 27, 20)
	if err != nil || len(records) != 1 {
		t.Fatalf("re-query failed: len=%d err=%v", len(records), err)
	}
	if records[0].Size != 0.02 {
		t.Fatalf("rollover target size = %v, want 0.02", records[0].Size)
	}
	if err := models.RolloverSellOrder(records[0], "NEW-SELL-3", records[0].Price, now.AddDate(0, 0, 30)); err != nil {
		t.Fatalf("RolloverSellOrder failed: %v", err)
	}
	var newSize float64
	var newRemarks string
	models.AppDB.QueryRow(`SELECT size, COALESCE(remarks,'') FROM sell_orders WHERE order_id='NEW-SELL-3'`).Scan(&newSize, &newRemarks)
	if newSize != 0.02 {
		t.Errorf("new record size = %v, want 0.02 (実際に再発注した数量)", newSize)
	}
	if newRemarks != "OLD-SELL-3の売り注文の再注文" {
		t.Errorf("new remarks = %q", newRemarks)
	}

	// status が UNFILLED でない行は補正できない（フェイルセーフ）
	if err := models.UpdateOrderSizeWithRemark(models.TableSellOrders, "OLD-SELL-3", 0.01, remark); err == nil {
		t.Error("expected an error when the row is no longer UNFILLED")
	}
}

/*
F3 の回帰テスト（DB側）。

前回の PlaceOrder がレスポンスを取りこぼしただけで成立していた場合、取引所には
DBに紐づかない売り注文（オーファン注文）が残る。これを「未知の注文」と判定する
models.OrderIDExists と、再発注せずにDBへ取り込む経路を検証する。
*/
func TestOrderIDExists(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	exp := now.AddDate(0, 0, 2)
	insertRolloverSellOrder(t, "P-EXIST", "S-KNOWN", "ETH_JPY", 512345, 0.02, "UNFILLED", nil, &exp, now.AddDate(0, 0, -28))
	// status に関係なく「DBに存在するか」で判定する（約定済み・キャンセル済みも既知の注文）
	insertRolloverSellOrder(t, "P-EXIST2", "S-KNOWN-CANCELLED", "ETH_JPY", 512345, 0.02, "CANCELLED", nil, &exp, now.AddDate(0, 0, -28))

	for _, id := range []string{"S-KNOWN", "S-KNOWN-CANCELLED"} {
		exists, err := models.OrderIDExists(models.TableSellOrders, id)
		if err != nil {
			t.Fatalf("OrderIDExists(%s) failed: %v", id, err)
		}
		if !exists {
			t.Errorf("%s はDBに存在するのに未知の注文と判定された（オーファンと誤認して取り込む恐れ）", id)
		}
	}

	exists, err := models.OrderIDExists(models.TableSellOrders, "S-ORPHAN")
	if err != nil {
		t.Fatalf("OrderIDExists failed: %v", err)
	}
	if exists {
		t.Error("DBに無い order_id が既知と判定された（オーファンを見逃して二重売りになる）")
	}

	// buy_orders 側と混線しないこと
	insertTestBuyOrder(t, "B-KNOWN", "ETH_JPY", 500000, 0.02, "UNFILLED")
	if exists, err := models.OrderIDExists(models.TableSellOrders, "B-KNOWN"); err != nil || exists {
		t.Errorf("buy_orders の order_id が sell_orders に存在すると判定された: exists=%v err=%v", exists, err)
	}

	if _, err := models.OrderIDExists(models.TableSellOrders, ""); err == nil {
		t.Error("空の order_id はエラーにすべき")
	}
}

// オーファン注文をDBへ取り込むと、再発注せずにローリングが完了すること。
func TestAdoptOrphanSellOrderKeepsSingleActiveOrder(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	pending := " " + models.RemarkRolloverPending
	exp := now.AddDate(0, 0, 2)
	insertRolloverSellOrder(t, "BUY-PARENT-ORPHAN", "OLD-SELL-ORPHAN", "ETH_JPY", 512345, 0.02,
		"UNFILLED", &pending, &exp, now.AddDate(0, 0, -28))

	records, err := models.GetSellOrdersToRollover(now, 3, 27, 20)
	if err != nil || len(records) != 1 {
		t.Fatalf("setup failed: len=%d err=%v", len(records), err)
	}
	old := records[0]

	// 取引所にだけ存在していたオーファン注文を、再発注せずそのまま取り込む
	orphanExpire := now.AddDate(0, 0, 30)
	if err := models.RolloverSellOrder(old, "ORPHAN-SELL-1", old.Price, orphanExpire); err != nil {
		t.Fatalf("adopt failed: %v", err)
	}

	var status string
	models.AppDB.QueryRow(`SELECT status FROM sell_orders WHERE order_id='OLD-SELL-ORPHAN'`).Scan(&status)
	if status != "CANCELLED" {
		t.Errorf("old status = %s, want CANCELLED", status)
	}

	var parentID string
	var price, size float64
	var newStatus string
	err = models.AppDB.QueryRow(
		`SELECT parentid, price, size, status FROM sell_orders WHERE order_id='ORPHAN-SELL-1'`).
		Scan(&parentID, &price, &size, &newStatus)
	if err != nil {
		t.Fatalf("query adopted record failed: %v", err)
	}
	if parentID != "BUY-PARENT-ORPHAN" {
		t.Errorf("parentid = %s, want BUY-PARENT-ORPHAN（損益計算の紐付け）", parentID)
	}
	if price != 512345 || size != 0.02 {
		t.Errorf("adopted record mismatch: price=%v size=%v", price, size)
	}
	if newStatus != "UNFILLED" {
		t.Errorf("adopted status = %s, want UNFILLED", newStatus)
	}

	// 取り込み後は order_id が既知になり、翌日以降にオーファンと誤認されない
	if exists, err := models.OrderIDExists(models.TableSellOrders, "ORPHAN-SELL-1"); err != nil || !exists {
		t.Errorf("取り込んだ注文が既知と判定されない: exists=%v err=%v", exists, err)
	}
	// UNFILLED の売り注文はこの1本だけ（＝二重売りになっていない）
	var unfilled int
	models.AppDB.QueryRow(`SELECT COUNT(*) FROM sell_orders WHERE status='UNFILLED'`).Scan(&unfilled)
	if unfilled != 1 {
		t.Errorf("UNFILLED の売り注文が %d 本。二重売りになっている", unfilled)
	}
}
