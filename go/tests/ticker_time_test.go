package tests

import (
	"testing"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
)

/*
TestTickerDateTime は bitflyer.Ticker.DateTime() が実APIの timestamp 形式を
パースできることを検証する。

Bitflyer の /v1/ticker が返す timestamp はタイムゾーンサフィックスを持たない
（実測: "2026-09-07T08:19:40.143"）。以前は time.Parse(time.RFC3339, ...) を
使っていたため常にパースに失敗しゼロ値を返していた。日時解釈のずれは壊れても
気づきにくいため、実形式・秒精度・RFC3339 のいずれも扱えることを固定する。
*/
func TestTickerDateTime(t *testing.T) {
	tests := []struct {
		name      string
		timestamp string
		want      time.Time
		wantZero  bool
	}{
		{
			name:      "実APIのミリ秒付き・TZサフィックス無し",
			timestamp: "2026-09-07T08:19:40.143",
			want:      time.Date(2026, 9, 7, 8, 19, 40, 143000000, time.UTC),
		},
		{
			name:      "秒精度・TZサフィックス無し",
			timestamp: "2026-09-07T08:19:40",
			want:      time.Date(2026, 9, 7, 8, 19, 40, 0, time.UTC),
		},
		{
			name:      "RFC3339(Zサフィックス付き)も受け付ける",
			timestamp: "2026-09-07T08:19:40Z",
			want:      time.Date(2026, 9, 7, 8, 19, 40, 0, time.UTC),
		},
		{
			name:      "JSTオフセット付きはUTCへ正規化される",
			timestamp: "2026-09-07T17:19:40+09:00",
			want:      time.Date(2026, 9, 7, 8, 19, 40, 0, time.UTC),
		},
		{
			name:      "空文字はゼロ値",
			timestamp: "",
			wantZero:  true,
		},
		{
			name:      "解釈不能な文字列はゼロ値",
			timestamp: "not-a-timestamp",
			wantZero:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticker := &bitflyer.Ticker{Timestamp: tt.timestamp}
			got := ticker.DateTime()

			if tt.wantZero {
				if !got.IsZero() {
					t.Errorf("DateTime() = %v, ゼロ値を期待", got)
				}
				return
			}

			if got.IsZero() {
				t.Fatalf("DateTime() がゼロ値を返した（パース失敗）: timestamp=%q", tt.timestamp)
			}
			if !got.Equal(tt.want) {
				t.Errorf("DateTime() = %v, want %v", got, tt.want)
			}
			if got.Location() != time.UTC {
				t.Errorf("DateTime() の location = %v, want UTC", got.Location())
			}
		})
	}
}

// TestTickerTruncateDateTime は DateTime() の結果が切り捨てに渡ることを確認する。
// パース失敗時にゼロ値が切り捨てられる（＝現在時刻に化けない）ことも固定する。
func TestTickerTruncateDateTime(t *testing.T) {
	ticker := &bitflyer.Ticker{Timestamp: "2026-09-07T08:19:40.143"}
	got := ticker.TruncateDateTime(time.Hour)
	want := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("TruncateDateTime(1h) = %v, want %v", got, want)
	}

	broken := &bitflyer.Ticker{Timestamp: "not-a-timestamp"}
	if !broken.TruncateDateTime(time.Hour).IsZero() {
		t.Errorf("パース失敗時の TruncateDateTime はゼロ値であるべき")
	}
}
