package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/ini.v1"
)

/*
F14 のテスト。

service.go にハードコードされていた "22:45"（cancelBuyOrderJob）と "01:20"
（グレースフルシャットダウン）を trigger_time_08 / trigger_time_09 として config 化した。

carlescere/scheduler の At() は解釈できない値でも Run() が静かに失敗するだけで、
service.go は戻り値を見ていない。つまり本番の config.ini に新キーが無い（＝空文字）と
「そのジョブが二度と発火しない」という無言のデグレになる。
NormalizeTriggerTime() がその手前で既定値へ倒すことを検証する。
*/

func TestNormalizeTriggerTime(t *testing.T) {
	const def = "22:45"

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "正常な HH:MM はそのまま", value: "23:45", want: "23:45"},
		{name: "秒まで指定した HH:MM:SS も許容", value: "06:05:30", want: "06:05:30"},
		{name: "時のみの指定も scheduler が許容するため通す", value: "6", want: "6"},
		{name: "境界値 00:00", value: "00:00", want: "00:00"},
		{name: "境界値 23:59:59", value: "23:59:59", want: "23:59:59"},
		{name: "前後の空白は除去する", value: "  05:30  ", want: "05:30"},

		// 本番 config.ini に新キーが無い場合（＝空文字）に既定値へ倒す
		{name: "空文字は既定値へフォールバック（キー未設定）", value: "", want: def},
		{name: "空白のみも既定値へフォールバック", value: "   ", want: def},

		// scheduler.parseTime が弾く値
		{name: "時が範囲外なら既定値", value: "24:00", want: def},
		{name: "分が範囲外なら既定値", value: "22:60", want: def},
		{name: "秒が範囲外なら既定値", value: "22:45:60", want: def},
		{name: "数値でなければ既定値", value: "22:4A", want: def},
		{name: "コロンが多すぎれば既定値", value: "22:45:00:00", want: def},
		{name: "負値は既定値", value: "-1:00", want: def},
		{name: "コロン無しの4桁は時として解釈できないため既定値", value: "2245", want: def},
		{name: "全く関係のない文字列は既定値", value: "morning", want: def},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeTriggerTime(tt.value, def); got != tt.want {
				t.Errorf("NormalizeTriggerTime(%q, %q) = %q, want %q", tt.value, def, got, tt.want)
			}
		})
	}
}

/*
既定値そのものが scheduler に渡して壊れない形式であることを固定する。
ここが崩れると、フォールバックしたのにジョブが発火しないという最悪の形になる。
*/
func TestDefaultTriggerTimesAreValid(t *testing.T) {
	defaults := map[string]string{
		"DefaultTriggerTime01": DefaultTriggerTime01,
		"DefaultTriggerTime02": DefaultTriggerTime02,
		"DefaultTriggerTime03": DefaultTriggerTime03,
		"DefaultTriggerTime04": DefaultTriggerTime04,
		"DefaultTriggerTime05": DefaultTriggerTime05,
		"DefaultTriggerTime06": DefaultTriggerTime06,
		"DefaultTriggerTime07": DefaultTriggerTime07,
		"DefaultTriggerTime08": DefaultTriggerTime08,
		"DefaultTriggerTime09": DefaultTriggerTime09,
	}
	for name, value := range defaults {
		if !isValidTriggerTime(value) {
			t.Errorf("%s = %q は scheduler が解釈できない形式", name, value)
		}
	}

	// 移行前の service.go にハードコードされていた値と一致していること（挙動を変えない）
	if DefaultTriggerTime08 != "22:45" {
		t.Errorf("cancelBuyOrderJob の既定時刻が変わっている: %q", DefaultTriggerTime08)
	}
	if DefaultTriggerTime09 != "01:20" {
		t.Errorf("グレースフルシャットダウンの既定時刻が変わっている: %q", DefaultTriggerTime09)
	}
}

/*
リグレッション: config.ini の読み方（インラインコメント付き・キー未設定）が
既存キーと同じ流儀で動くことを、実際に ini.v1 でパースして確認する。
*/
func TestTriggerTimeFromINI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.ini")
	content := `[tradeSetting]
trigger_time_08=23:15 # cancelBuyOrderJob の実行時刻
trigger_time_09= # 値なし
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := ini.Load(path)
	if err != nil {
		t.Fatalf("failed to load temp config: %v", err)
	}
	section := cfg.Section("tradeSetting")

	// インラインコメントは除去され、値だけが残る
	got := NormalizeTriggerTime(section.Key("trigger_time_08").String(), DefaultTriggerTime08)
	if got != "23:15" {
		t.Errorf("インラインコメント付きの値のパースに失敗: got=%q want=%q", got, "23:15")
	}

	// 値が空なら既定値
	got = NormalizeTriggerTime(section.Key("trigger_time_09").String(), DefaultTriggerTime09)
	if got != DefaultTriggerTime09 {
		t.Errorf("空値は既定値になること: got=%q want=%q", got, DefaultTriggerTime09)
	}

	// キー自体が存在しない（本番 config.ini を更新し忘れたケース）でも既定値
	got = NormalizeTriggerTime(section.Key("trigger_time_99").String(), DefaultTriggerTime08)
	if got != DefaultTriggerTime08 {
		t.Errorf("未定義キーは既定値になること: got=%q want=%q", got, DefaultTriggerTime08)
	}
}

/*
F30-1 のテスト。

trigger_time_01〜04 は素の .String() で読まれており、キーが無い・値が壊れている場合に
空文字が入っても、エラーにも警告にもならなかった。空文字は scheduler.At() で弾かれ、
Run() は戻り値でしかエラーを返さないため、そのジョブが二度と発火しないまま
プロセスは正常に動き続ける。特に trigger_time_01 は買い注文ジョブ12本を巻き添えにする。

NormalizeTriggerTime() を 01〜04 にも通したことを、既定値の妥当性と
「既定値が現行の config.ini と同一（＝挙動を変えない）」の2点で担保する。
*/
func TestDefaultTriggerTimes01To04MatchShippedConfig(t *testing.T) {
	// 移行前の go/config.ini にあった値と一致していること（正常設定時の挙動を変えない）
	want := map[string]string{
		"DefaultTriggerTime01": "06:30",
		"DefaultTriggerTime02": "06:45",
		"DefaultTriggerTime03": "18:00",
		"DefaultTriggerTime04": "06:00",
	}
	got := map[string]string{
		"DefaultTriggerTime01": DefaultTriggerTime01,
		"DefaultTriggerTime02": DefaultTriggerTime02,
		"DefaultTriggerTime03": DefaultTriggerTime03,
		"DefaultTriggerTime04": DefaultTriggerTime04,
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s = %q, want %q（既定値は移行前の config.ini と同一でなければならない）", name, got[name], w)
		}
	}
}

/*
リグレッション: 実際に出荷している go/config.ini を読み、
NormalizeTriggerTime() を通しても値が一切変わらないことを確認する。

「フォールバックを足したせいで正常設定の時刻がずれた」という事故を防ぐためのテスト。
*/
func TestShippedConfigTriggerTimesArePassedThrough(t *testing.T) {
	cfg, err := ini.Load(filepath.Join("..", ConfigPath))
	if err != nil {
		t.Fatalf("failed to load go/config.ini: %v", err)
	}
	section := cfg.Section("tradeSetting")

	cases := []struct {
		key        string
		defaultVal string
	}{
		{"trigger_time_01", DefaultTriggerTime01},
		{"trigger_time_02", DefaultTriggerTime02},
		{"trigger_time_03", DefaultTriggerTime03},
		{"trigger_time_04", DefaultTriggerTime04},
		{"trigger_time_05", DefaultTriggerTime05},
		{"trigger_time_06", DefaultTriggerTime06},
		{"trigger_time_07", DefaultTriggerTime07},
		{"trigger_time_08", DefaultTriggerTime08},
		{"trigger_time_09", DefaultTriggerTime09},
	}
	for _, c := range cases {
		raw := section.Key(c.key).String()
		if raw == "" {
			t.Errorf("%s が go/config.ini に無い（既定値へ倒れてしまうため設定漏れ）", c.key)
			continue
		}
		if got := NormalizeTriggerTime(raw, c.defaultVal); got != raw {
			t.Errorf("%s: NormalizeTriggerTime(%q) = %q, want %q（正常設定は素通しでなければならない）", c.key, raw, got, raw)
		}
	}
}

/*
リグレッション: trigger_time_01 のキーが欠けた config.ini でも、
買い注文ジョブが既定値(06:30)で登録されること。デプロイ時の編集ミスで
「買い注文が完全に停止する」状態にならないことを固定する。
*/
func TestTriggerTime01FallsBackWhenKeyMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.ini")
	content := `[tradeSetting]
trigger_time_02=06:45
trigger_time_03=18:00
trigger_time_04=
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	cfg, err := ini.Load(path)
	if err != nil {
		t.Fatalf("failed to load temp config: %v", err)
	}
	section := cfg.Section("tradeSetting")

	// キー自体が存在しない → 既定値
	if got := NormalizeTriggerTime(section.Key("trigger_time_01").String(), DefaultTriggerTime01); got != DefaultTriggerTime01 {
		t.Errorf("trigger_time_01 未設定時は既定値になること: got=%q want=%q", got, DefaultTriggerTime01)
	}
	// 値が空 → 既定値
	if got := NormalizeTriggerTime(section.Key("trigger_time_04").String(), DefaultTriggerTime04); got != DefaultTriggerTime04 {
		t.Errorf("trigger_time_04 空値は既定値になること: got=%q want=%q", got, DefaultTriggerTime04)
	}
	// 有効値はそのまま
	if got := NormalizeTriggerTime(section.Key("trigger_time_02").String(), DefaultTriggerTime02); got != "06:45" {
		t.Errorf("trigger_time_02 有効値は素通しすること: got=%q want=%q", got, "06:45")
	}
}
