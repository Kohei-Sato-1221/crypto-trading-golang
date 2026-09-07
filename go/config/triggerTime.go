package config

import (
	"log"
	"strconv"
	"strings"
)

/*
NormalizeTriggerTime は config.ini から読んだ日次ジョブの実行時刻(JST)を検証し、
使えない値なら既定値へフォールバックする。

carlescere/scheduler の At() は "HH:MM:SS" / "HH:MM" / "HH" を受け付け、
解釈できない値では Job にエラーを持たせるだけで Run() も静かに失敗する。
service.go は Run() の戻り値を見ていないため、設定ミスがあると
「そのジョブが二度と発火しない」という形で無言のデグレになる。
これを防ぐため、scheduler.parseTime と同じ規則でここで先に弾き、
既定値に倒したうえでログへ警告を残す。

（Slack通知はしない。NewConfig() は Slack クライアントの初期化より前に走るため）

不正とみなす値:
  - 空文字（キー未設定・値なし）
  - コロン区切りが4つ以上
  - 数値としてパースできないチャンク
  - hour > 23 / minute > 59 / second > 59、または負値
*/
func NormalizeTriggerTime(value, defaultValue string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultValue
	}
	if !isValidTriggerTime(trimmed) {
		log.Printf("【config】trigger time %q は解釈できないため既定値 %q を使用します", value, defaultValue)
		return defaultValue
	}
	return trimmed
}

/*
isValidTriggerTime は carlescere/scheduler の parseTime が受け付ける形式かを返す。
scheduler 側は "8"（時のみ）・"08:35"・"08:35:30" を許容する。
*/
func isValidTriggerTime(value string) bool {
	chunks := strings.Split(value, ":")
	if len(chunks) < 1 || len(chunks) > 3 {
		return false
	}

	limits := []int{23, 59, 59}
	for i, chunk := range chunks {
		n, err := strconv.Atoi(chunk)
		if err != nil {
			return false
		}
		if n < 0 || n > limits[i] {
			return false
		}
	}
	return true
}
