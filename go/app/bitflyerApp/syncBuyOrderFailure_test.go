package bitflyerApp

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

/*
F9 の回帰テスト。

models 層のエラーがログ出力のみで Slack に届かず、
「取引所には注文があるがDBに記録されていない」「expire_date が入らず失効検出が効かない」
といった状態が無言で進んでいた。models 側が失敗を返すようになったので、
app 側がそれを通知本文へ正しく展開できることを検証する。

Slack・DB・取引所APIに触れない純粋関数のみを対象にする。
*/

func syncFailure(op, orderID string) models.SyncBuyOrderFailure {
	return models.SyncBuyOrderFailure{
		Operation:   op,
		OrderID:     orderID,
		ProductCode: "ETH_JPY",
		Side:        "BUY",
		Price:       512345.67,
		Size:        0.02,
		Strategy:    11,
		Err:         errors.New("boom"),
	}
}

// 失敗が無ければ通知しない（正常時に空通知が飛ばないことの回帰確認）。
func TestFormatSyncBuyOrderFailuresEmpty(t *testing.T) {
	if msg := formatSyncBuyOrderFailures("ETH_JPY", nil); msg != "" {
		t.Errorf("failures なしでは通知しない想定だが本文が生成された: %q", msg)
	}
	if msg := formatSyncBuyOrderFailures("ETH_JPY", []models.SyncBuyOrderFailure{}); msg != "" {
		t.Errorf("空スライスでは通知しない想定だが本文が生成された: %q", msg)
	}
}

// 通知本文に OrderID / Price / Size / Strategy 等のコンテキストが含まれること（開発ルール）。
func TestFormatSyncBuyOrderFailuresIncludesContext(t *testing.T) {
	msg := formatSyncBuyOrderFailures("ETH_JPY", []models.SyncBuyOrderFailure{syncFailure("insert", "ORDER-1")})

	for _, want := range []string{"ORDER-1", "ETH_JPY", "BUY", "512345.67", "0.02", "Strategy:11", "insert", "boom"} {
		if !strings.Contains(msg, want) {
			t.Errorf("通知本文に %q が含まれていない: %s", want, msg)
		}
	}
	if !strings.HasPrefix(msg, "🚨") {
		t.Errorf("エラー通知は 🚨 で始まる想定: %s", msg)
	}
}

// 3種類の失敗（count / insert / expire_date）がすべて本文に載ること。
func TestFormatSyncBuyOrderFailuresAllOperations(t *testing.T) {
	failures := []models.SyncBuyOrderFailure{
		syncFailure("count", "ORDER-C"),
		syncFailure("insert", "ORDER-I"),
		syncFailure("expire_date", "ORDER-E"),
	}
	msg := formatSyncBuyOrderFailures("BTC_JPY", failures)

	if !strings.Contains(msg, "3件") {
		t.Errorf("件数が本文に含まれていない: %s", msg)
	}
	for _, want := range []string{"ORDER-C", "ORDER-I", "ORDER-E", "count", "insert", "expire_date"} {
		if !strings.Contains(msg, want) {
			t.Errorf("通知本文に %q が含まれていない: %s", want, msg)
		}
	}
}

// 恒常的な失敗で通知が肥大しないよう、明細は syncFailureSampleSize 件で打ち切られること。
func TestFormatSyncBuyOrderFailuresTruncatesDetails(t *testing.T) {
	failures := make([]models.SyncBuyOrderFailure, 0, syncFailureSampleSize+3)
	for i := 0; i < syncFailureSampleSize+3; i++ {
		failures = append(failures, syncFailure("insert", fmt.Sprintf("ORDER-%d", i)))
	}
	msg := formatSyncBuyOrderFailures("BTC_JPY", failures)

	if !strings.Contains(msg, fmt.Sprintf("%d件", len(failures))) {
		t.Errorf("総件数が本文に含まれていない: %s", msg)
	}
	if !strings.Contains(msg, "...他3件") {
		t.Errorf("打ち切りの表示が無い: %s", msg)
	}
	if strings.Contains(msg, fmt.Sprintf("ORDER-%d", syncFailureSampleSize)) {
		t.Errorf("サンプル上限を超える明細が載っている: %s", msg)
	}
	if !strings.Contains(msg, "ORDER-0") {
		t.Errorf("先頭の明細が載っていない: %s", msg)
	}
}
