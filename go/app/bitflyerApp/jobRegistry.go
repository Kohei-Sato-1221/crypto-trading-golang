package bitflyerApp

import (
	"fmt"
	"log"
	"strings"

	"github.com/carlescere/scheduler"
)

/*
jobRegistry は scheduler へのジョブ登録をまとめ、登録に失敗したジョブを集約する。

# なぜ必要か（F30-2）

carlescere/scheduler の登録は多段で無言化する。

 1. Every().Day().At(v) は v が解釈できないとき Job.err に格納するだけで、
    戻り値は *Job のみ。呼び出し側からはエラーが見えない
 2. Run() は (*Job, error) を返し j.err があれば nil, err を返すが、
    従来の service.go は戻り値を捨てていた
 3. その結果ゴルーチンが起動されず、そのジョブは二度と発火しない
 4. ログにもSlackにも何も出ず、プロセスは正常に動き続ける

登録が20本近くあり、1本欠けても気づけない。特に買い注文ジョブ12本は
すべて同じ trigger_time_01 を使うため、1つの設定ミスで発注が全滅する。
そこで登録を daily / interval / dailyWithoutWrap のヘルパーに集約し、
Run() の戻り値を必ず検査して失敗を溜め込む。

# 登録失敗時にプロセスを落とさない理由

失敗を検知しても os.Exit しない。起動時に1通のSlackエラー通知とログを出し、
登録できたジョブはそのまま動かし続ける。理由は次の3点。

  - 実行環境は systemd の Restart=always / RestartSec=10 である。設定ミスは
    再起動しても直らないため、落とすと10秒間隔の再起動ループになり、
    そのたびにSlackへ通知が飛ぶ（通知が埋もれて逆に気づけなくなる）
  - 部分稼働のほうが安全側である。例えば trigger_time_02（損益レポート）だけが
    壊れた場合、プロセスを落とすと syncBuyOrders / filledCheck / placeSellOrder が
    止まり、既に取引所に出ている買い注文が約定しても売り注文が発注されない。
    「レポートが来ない」より「建玉が放置される」ほうが損害が大きい
  - F30-1 で trigger_time_01〜09 はすべて NormalizeTriggerTime() を通り
    既定値へ倒れるため、設定由来の At() 失敗はそもそも起きない。ここに残る
    失敗要因は Every(0) やチェーン誤りといった実装バグで、ビルド・テストで
    捕捉すべきものである

つまりこの仕掛けの目的は「止めること」ではなく「無言をやめること」にある。
気づいた人間が config.ini を直して再起動する、という運用を前提にしている。
*/
type jobRegistry struct {
	registered int
	failures   []jobRegistrationFailure
}

// jobRegistrationFailure は scheduler へのジョブ登録が失敗した1件を表す。
type jobRegistrationFailure struct {
	Name     string // ジョブ名（ログ・Slackに出す識別子）
	Schedule string // スケジュール指定（"06:30" / "90秒ごと" など）
	Err      error
}

/*
record は登録結果を1件受け取り、失敗なら集約する。

scheduler に触れない純粋なロジックとして daily / interval から切り出してある。
（登録処理そのものはゴルーチンを起動する副作用があるためテストしにくい）
*/
func (r *jobRegistry) record(name, schedule string, err error) {
	if err != nil {
		r.failures = append(r.failures, jobRegistrationFailure{
			Name:     name,
			Schedule: schedule,
			Err:      err,
		})
		return
	}
	r.registered++
}

// hasFailures は登録に失敗したジョブが1件でもあるかを返す。
func (r *jobRegistry) hasFailures() bool {
	return len(r.failures) > 0
}

// total は登録を試みたジョブの総数を返す。
func (r *jobRegistry) total() int {
	return r.registered + len(r.failures)
}

/*
failureReport は登録失敗をSlack/ログ向けの1つのメッセージにまとめる。
失敗が無ければ空文字を返す。

「どのジョブが」「どの設定値で」「なぜ」失敗したかを列挙し、
そのジョブが二度と発火しないことと対処方法を明記する。
*/
func (r *jobRegistry) failureReport() string {
	if !r.hasFailures() {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "【ERROR】スケジュール登録に失敗したジョブがあります（%d/%d件）\n", len(r.failures), r.total())
	b.WriteString("下記のジョブは二度と発火しません。config.ini の該当キーを修正してアプリを再起動してください。\n")
	for _, f := range r.failures {
		fmt.Fprintf(&b, "- %s（schedule=%q）: %v\n", f.Name, f.Schedule, f.Err)
	}
	return strings.TrimRight(b.String(), "\n")
}

/*
daily は毎日 at（"HH:MM" 等）に実行するジョブを登録する。
ジョブは wrapJob() でラップし、シャットダウン中の実行ブロックと
実行中ジョブの追跡の対象にする。
*/
func (r *jobRegistry) daily(name, at string, job func()) {
	_, err := scheduler.Every().Day().At(at).Run(wrapJob(job))
	r.record(name, at, err)
}

/*
dailyWithoutWrap は毎日 at に実行するジョブを wrapJob() を通さずに登録する。

gracefulShutdown 専用。gracefulShutdown は runningJobs.Wait() で実行中ジョブの
完了を待つため、自分自身が wrapJob() で runningJobs に加算されていると
自分の完了を待ち続けてタイムアウトするまで進めなくなる。
またシャットダウン中フラグで自分自身がスキップされてしまう。
*/
func (r *jobRegistry) dailyWithoutWrap(name, at string, job func()) {
	_, err := scheduler.Every().Day().At(at).Run(job)
	r.record(name, at, err)
}

// interval は seconds 秒ごとに実行するジョブを登録する。
func (r *jobRegistry) interval(name string, seconds int, job func()) {
	_, err := scheduler.Every(seconds).Seconds().Run(wrapJob(job))
	r.record(name, fmt.Sprintf("%d秒ごと", seconds), err)
}

/*
reportRegistrationResult は登録結果をログに残し、
失敗が1件でもあれば起動時に1通だけSlackへエラー通知する。

プロセスは落とさない（理由は jobRegistry のコメントを参照）。
*/
func (r *jobRegistry) reportRegistrationResult() {
	log.Printf("【StartBfService】スケジュール登録: 成功 %d件 / 失敗 %d件", r.registered, len(r.failures))
	if !r.hasFailures() {
		return
	}

	report := r.failureReport()
	log.Println(report)
	if slackClient == nil {
		log.Println("【StartBfService】Slackクライアント未初期化のため、登録失敗の通知を送信できません")
		return
	}
	if err := slackClient.PostMessage(report, true); err != nil {
		log.Printf("【StartBfService】スケジュール登録失敗のSlack通知に失敗しました: %v", err)
	}
}
