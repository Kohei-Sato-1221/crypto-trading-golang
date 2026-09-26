#!/bin/bash
# =============================================================================
# ラズパイ上の bfTradingApp の状態を一括収集する調査スクリプト（参照系のみ）
#
#   使い方（Pi 上で実行）:
#     bash scripts/diagnose-pi.sh 2>&1 | tee /tmp/diag-$(date +%Y%m%d-%H%M).txt
#
#   このスクリプトは読み取りしか行わない。サービスの停止・再起動・発注は一切しない。
#   journalctl が「権限がない」と言う場合のみ sudo を付けて再実行する。
# =============================================================================

SVC=bfTradingApp.service
APPDIR=/home/pi/workspace/crypto-trading-golang
LOG=$APPDIR/go/trading.log

sec() { echo; echo "════════ $1 ════════"; }
run() { echo "\$ $*"; eval "$@" 2>&1 | sed 's/^/  /'; }

echo "collected_at: $(date '+%Y-%m-%d %H:%M:%S %Z')"

# -----------------------------------------------------------------------------
# 1. 時刻と再起動の実績
#    「再起動後に動いていないのか」を判定する起点。Pi は RTC が無いため
#    起動直後の時刻ずれがジョブ発火時刻を狂わせる（bfTradingApp.service のコメント参照）
# -----------------------------------------------------------------------------
sec "1. 時刻・時刻同期・起動履歴"
run uptime
run timedatectl                                   # System clock synchronized: yes であること
run "systemctl is-enabled systemd-time-wait-sync.service"  # enabled でないと time-sync.target が無意味
run "journalctl --list-boots | tail -10"          # 実際に何時に再起動しているか
run "journalctl -b -0 -u systemd-timesyncd --no-pager | head -20"  # 起動時にどれだけ時刻が飛んだか

# -----------------------------------------------------------------------------
# 2. サービスの生死と再起動回数
#    ここだけで「落ちているのか／上がったまま無言なのか」がほぼ分かる
# -----------------------------------------------------------------------------
sec "2. サービスの状態"
run "systemctl status $SVC --no-pager -l | head -30"
run "systemctl show $SVC -p NRestarts -p ExecMainStartTimestamp -p ExecMainPID -p ActiveEnterTimestamp -p Result"
echo
echo "--- 過去7日の起動/終了/失敗イベントのみ抽出（systemd 側の記録） ---"
run "journalctl -u $SVC --since '7 days ago' --no-pager | grep -Ei 'Started|Stopped|Stopping|Scheduled restart|Main process exited|Failed|killed|watchdog|core-dump' | tail -60"

# -----------------------------------------------------------------------------
# 3. カーネル側の死因（OOM / SDカード / 電源）
#    Pi でサービスが「静かにおかしくなる」原因の大半はここ
# -----------------------------------------------------------------------------
sec "3. OOM・SDカード・電源"
run "journalctl -k --since '7 days ago' --no-pager | grep -Ei 'out of memory|oom-kill|killed process' | tail -20"
run "journalctl -k --since '7 days ago' --no-pager | grep -Ei 'mmc|I/O error|EXT4-fs error|remounting.*read-only|Remounted' | tail -30"
run "journalctl -k --since '7 days ago' --no-pager | grep -Ei 'under-voltage|undervoltage|throttl' | tail -20"
if command -v vcgencmd >/dev/null; then
  run "vcgencmd get_throttled"     # 0x0 以外なら電源/熱の問題。0x50000/0x50005 等は要対処
  run "vcgencmd measure_temp"
fi
echo "  # ファイルシステムが ro になっていないか（ro ならアプリは動いていても何も書けない）"
run "findmnt -no TARGET,FSTYPE,OPTIONS / | head -5"
run "touch $APPDIR/go/.diag_write_test && echo '  -> 書き込みOK' && rm -f $APPDIR/go/.diag_write_test"

# -----------------------------------------------------------------------------
# 4. ストレージ
#    trading.log は systemd の append: とアプリ自身の両方が書くため 2 重に増える。
#    logrotate は入っていない（手動 clear-trading-log.sh のみ）
# -----------------------------------------------------------------------------
sec "4. ストレージ"
run "df -h /"
run "df -i /"                                     # inode 枯渇も「書けない」を起こす
run "ls -lh $LOG $APPDIR/go/trading_bk.log 2>/dev/null"
run "du -sh /var/log /var/log/journal 2>/dev/null"

# -----------------------------------------------------------------------------
# 5. メモリ・CPU（プロセス単体の実測値）
# -----------------------------------------------------------------------------
sec "5. メモリ・CPU"
run "free -h"
run "ps -o pid,etime,%cpu,%mem,rss,vsz,nlwp,stat,cmd -C bfTradingApp"
run "cat /proc/pressure/memory 2>/dev/null"
run "cat /proc/pressure/io 2>/dev/null"
PID=$(pgrep -x bfTradingApp | head -1)
if [ -n "$PID" ]; then
  run "grep -E 'VmRSS|Threads|FDSize' /proc/$PID/status"
  run "ls /proc/$PID/fd | wc -l"                  # goroutine/FD リークの目安
fi

# -----------------------------------------------------------------------------
# 6. アプリログの健全性
#    「上がっているのにジョブが動いていない」を見抜く。90秒間隔ジョブが
#    直近に動いていれば生きている。止まっていればスケジューラが死んでいる
# -----------------------------------------------------------------------------
sec "6. アプリログ（trading.log）"
run "tail -40 $LOG"
echo
echo "--- 直近24時間のジョブ発火状況（日時プレフィックス別の件数） ---"
run "grep -E '【order】|syncBuyOrders|filledCheck|reconcile|expireSweep|rollover' $LOG | tail -20"
echo
echo "--- ERROR / panic / シャットダウンの痕跡 ---"
run "grep -nE 'panic:|goroutine [0-9]+ \[|\[ERROR\]|グレースフルシャットダウン|スケジュール登録に失敗' $LOG | tail -40"
echo
echo "--- 1時間ごとのログ行数（アプリが無言になった時刻を特定する） ---"
run "awk '{print \$1, substr(\$2,1,2)}' $LOG | uniq -c | tail -40"

# -----------------------------------------------------------------------------
# 7. 誰がいつ再起動しているのか
# -----------------------------------------------------------------------------
sec "7. 再起動・停止のスケジュール実体"
run "crontab -l"
run "sudo -n crontab -l -u root 2>/dev/null || echo '  (root crontab は sudo が必要)'"
run "ls -l /etc/cron.d/ 2>/dev/null"
run "grep -rIl 'reboot\|shutdown' /etc/cron.d /etc/crontab /etc/cron.daily 2>/dev/null"
run "systemctl list-timers --all --no-pager | head -20"
run "systemctl is-enabled $SVC restart-bftradingapp.service"

# -----------------------------------------------------------------------------
# 8. 日次ジョブの発火時刻（「発火していない」のか「ズレた時刻に発火している」のか）
#
#    carlescere/scheduler の daily ジョブは、登録時の壁時計から次回までの
#    「相対時間」を1回だけ計算し、モノトニックタイマーで待つ（発火後に再計算）。
#    そのため起動後にNTPが時刻を補正すると、その日の発火時刻は補正量ぶんズレる。
#    Pi は RTC を持たないため、毎晩の再起動でこれが毎晩再発しうる。
# -----------------------------------------------------------------------------
sec "8. 日次ジョブの発火実績"
echo "  # TZ が JST でなければ daily ジョブは丸ごとズレる（At(\"06:15\") は time.Local 基準）"
run "timedatectl | grep -Ei 'Time zone|synchronized|RTC'"
echo
echo "  # 起動時のジョブ登録件数。成功26件/失敗0件が正常"
run "grep 'スケジュール登録' $LOG | tail -5"
echo
echo "  # アプリの起動時刻（＝スケジュール登録時刻）"
run "grep '【StartBfService】start' $LOG | tail -7"
echo
echo "  # 起動時のNTP補正量。ここがズレ量Δの実測値になる"
run "journalctl -b -0 -u systemd-timesyncd --no-pager | grep -Ei 'adjust|jump|synchronized|leap' | head -10"
echo
echo "  # 各日次ジョブが実際に何時に発火したか（reconcile は local と UTC の両方を出す）"
run "grep '【reconcile】start of job' $LOG | tail -10"
run "grep '【sendResultsJob】Start of job' $LOG | tail -10"
run "grep -E '【expireSweep|【rollover|savePriceHistory' $LOG | grep -i start | tail -10"
echo
echo "  # ★決定的な切り分け: 06:15 前後に90秒間隔ジョブのログがあるか"
echo "  #   ある  → プロセスは生きていた。日次ジョブのタイマーだけがズレている（時刻補正が原因）"
echo "  #   ない  → その時刻にプロセスが死んでいた（2章・3章を見る）"
run "grep -E '^20[0-9]{2}/[0-9]{2}/[0-9]{2} 06:(0[5-9]|1[0-9]|2[0-9])' $LOG | tail -30"

# -----------------------------------------------------------------------------
# 9. 稼働中のコードとバイナリの同一性
#    reconcileJob / expireSweepJob / rolloverSellOrderJob は
#    feature/order-lifecycle-overhaul にしか無い。main のバイナリなら存在しない
# -----------------------------------------------------------------------------
sec "9. 稼働中のコード・バイナリ"
run "git -C $APPDIR branch --show-current"
run "git -C $APPDIR log --oneline -3"
run "git -C $APPDIR status --short"
run "ls -l --time-style=long-iso $APPDIR/go/bfTradingApp"
echo "  # バイナリに日次ジョブが含まれているか（含まれなければ発火するはずがない）"
run "strings $APPDIR/go/bfTradingApp | grep -c reconcile"
run "systemctl show $SVC -p ExecMainStartTimestamp"
echo "  # ExecMainStartTimestamp がバイナリの更新時刻より古ければ、古いバイナリが動いている"

sec "収集完了"
echo "この出力をそのまま貼ってもらえれば切り分けできます。"
