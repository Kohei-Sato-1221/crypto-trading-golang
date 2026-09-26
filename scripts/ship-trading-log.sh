#!/bin/bash
# =============================================================================
# trading.log を1時間ごとに Supabase Storage へ退避し、ローカルを空にする
#
#   依存: curl / gzip / flock / python3  ← すべて Raspberry Pi OS に標準で入っている
#         awscli は不要。
#
#   cron（pi ユーザー）:
#     5 * * * * /home/pi/workspace/crypto-trading-golang/scripts/ship-trading-log.sh run
#     20 4 * * 0 /home/pi/workspace/crypto-trading-golang/scripts/ship-trading-log.sh prune
#
#   サブコマンド:
#     run        ローテート（cp して即 truncate）→ 未送信分をまとめて送信
#     flush      ローテートせず、溜まった未送信分の再送のみ
#     prune      保持期間を過ぎたオブジェクトを Storage から削除
#     prune dry  prune の対象を表示するだけ（削除しない）
#     selftest   trading.log に触らず疎通確認だけ行う
#
# ■ 設計上ゆずれない点
#
#   1. mv ではなく cp + truncate（copytruncate方式）を使う。
#      trading.log は systemd の StandardOutput=append: と、アプリ自身の
#      utils.LogSetting() の2つが O_APPEND で開いている。mv で inode を差し替えると
#      両者は元の inode に書き続け、以後のログが行方不明になる。
#      truncate なら O_APPEND が常に EOF へ書くため、サービス再起動なしで安全に空にできる。
#
#   2. アップロードが成功するまでローカルから消さない。
#      cp -> アップロード -> 成功したら削除。ネットワーク断でもログを失わない。
#      失敗分は spool に残り、次回の実行で再送される。
#
#   3. ローテートはアップロードの成否を待たない。
#      ディスク逼迫の解消（本来の目的）を、外部サービスの可用性に依存させない。
#
#   4. prune は自分が作ったファイル名にしか触らない。
#      Supabase Storage にはライフサイクル（自動削除）設定が無いため保持期間の管理は
#      自前で行う必要があるが、バケット内の見知らぬオブジェクトは絶対に削除しない。
#
#   ※ 本スクリプト導入後は clear-trading-log.sh / backUp.sh を使わないこと。
#      退避前にログを消すと証拠が失われる。
# =============================================================================
set -uo pipefail

MODE=${1:-run}
SUBMODE=${2:-}

# ---- 設定の読み込み（機密情報はリポジトリ外に置く。600 で保護すること） ------
CONF=${TRADING_LOG_SHIPPER_CONF:-/home/pi/.config/trading-log-shipper.env}
if [ -r "$CONF" ]; then
  # shellcheck disable=SC1090
  . "$CONF"
fi

LOG_FILE=${LOG_FILE:-/home/pi/workspace/crypto-trading-golang/go/trading.log}
SPOOL=${SPOOL:-/home/pi/trading-log-spool}
SHIP_LOG=${SHIP_LOG:-/home/pi/trading-log-shipper.log}
KEY_PREFIX=${KEY_PREFIX:-trading-log}
HOSTTAG=${HOSTTAG:-$(hostname -s)}
INCLUDE_JOURNAL=${INCLUDE_JOURNAL:-1}
MIN_BYTES=${MIN_BYTES:-64}
MAX_SPOOL_DAYS=${MAX_SPOOL_DAYS:-14}
SPOOL_WARN_COUNT=${SPOOL_WARN_COUNT:-6}
RETENTION_DAYS=${RETENTION_DAYS:-90}
SLACK_WEBHOOK=${SLACK_WEBHOOK:-}
CURL_MAX_TIME=${CURL_MAX_TIME:-120}

# Supabase（方式A: REST + シークレットキー）
#
# キーは新旧2方式ある。どちらでも動く。
#   新: SUPABASE_SECRET_KEY=sb_secret_...   （現行。ダッシュボードの Secret keys）
#   旧: SUPABASE_SERVICE_KEY=eyJ...         （Legacy anon, service_role API keys タブ）
# 新方式を推奨する。用途ごとに別のキーを発行でき、漏れたときにそれ1本だけ失効できるため。
SUPABASE_URL=${SUPABASE_URL:-}
SUPABASE_SECRET_KEY=${SUPABASE_SECRET_KEY:-${SUPABASE_SERVICE_KEY:-}}
SUPABASE_BUCKET=${SUPABASE_BUCKET:-}

STORAGE_API="${SUPABASE_URL%/}/storage/v1"

mkdir -p "$SPOOL" || exit 1

# 記録は常に $SHIP_LOG へ。端末から手で叩いたときだけ画面にも出す（cron実行時は静かにする）
say() {
  local line
  line="$(date '+%Y/%m/%d %H:%M:%S') $*"
  printf '%s\n' "$line" >> "$SHIP_LOG"
  [ -t 1 ] && printf '%s\n' "$line"
  return 0
}

# shipper 自身のログが無限に伸びないよう、1MBを超えたら後半だけ残す
trim_ship_log() {
  local sz
  sz=$(stat -c %s "$SHIP_LOG" 2>/dev/null || echo 0)
  if [ "$sz" -gt 1048576 ]; then
    tail -n 2000 "$SHIP_LOG" > "$SHIP_LOG.tmp" && mv "$SHIP_LOG.tmp" "$SHIP_LOG"
  fi
}

require_conf() {
  local missing=""
  [ -n "$SUPABASE_URL" ]         || missing="$missing SUPABASE_URL"
  [ -n "$SUPABASE_SECRET_KEY" ]  || missing="$missing SUPABASE_SECRET_KEY"
  [ -n "$SUPABASE_BUCKET" ]      || missing="$missing SUPABASE_BUCKET"
  if [ -n "$missing" ]; then
    say "[ERROR] 設定が足りない:$missing （$CONF を確認）"
    return 1
  fi
  return 0
}

notify_slack() {
  [ -n "$SLACK_WEBHOOK" ] || return 0
  local json
  json=$(printf '%s' "$1" | python3 -c 'import json,sys; print(json.dumps({"text": sys.stdin.read()}))')
  curl -sS --max-time 20 -X POST -H 'Content-type: application/json' \
    --data "$json" "$SLACK_WEBHOOK" >/dev/null 2>&1
}

# ---- 多重起動防止（前回のアップロードが長引いている最中に重ねない） --------
exec 9>"/tmp/trading-log-shipper.lock" || exit 1
if ! flock -n 9; then
  say "[SKIP] 前回の実行がまだ走っているため何もしない"
  exit 0
fi

# =============================================================================
# アップロード（1ファイル）
#   POST {STORAGE_API}/object/{bucket}/{key}
#   x-upsert: true を付けないと同名オブジェクトへのPOSTが409で弾かれる
#
#   【重要】apikey と Authorization の両方を送る。
#   新方式の sb_secret_... はJWTではないため、Authorization: Bearer だけを送ると
#   ゲートウェイがJWTとしてデコードしようとして失敗し 401 になる。apikey ヘッダーが
#   あればキーの直接参照で解決される。両方送れば新旧どちらのキー形式でも通る。
# =============================================================================
upload_one() {
  local f=$1 key=$2 code body
  body=$(mktemp)
  code=$(curl -sS -o "$body" -w '%{http_code}' --max-time "$CURL_MAX_TIME" \
    -X POST "${STORAGE_API}/object/${SUPABASE_BUCKET}/${key}" \
    -H "Authorization: Bearer ${SUPABASE_SECRET_KEY}" \
    -H "apikey: ${SUPABASE_SECRET_KEY}" \
    -H "x-upsert: true" \
    -H "Content-Type: application/gzip" \
    --data-binary @"$f" 2>>"$SHIP_LOG")
  if [ "$code" = "200" ]; then
    rm -f "$body"
    return 0
  fi
  # 容量超過はここに出る（QuotaExceeded）。原因が分かるよう本文を残す
  say "[ERROR] upload failed http=$code key=$key body=$(head -c 400 "$body" | tr -d '\n')"
  rm -f "$body"
  return 1
}

# ファイル名 host-trading-YYYYMMDD-HHMMSS.log.gz から日付パーティション付きの
# キーを組み立てる（prune で日付プレフィックスを辿れるようにするため）
build_key() {
  local base=$1 ymd
  ymd=$(printf '%s' "$base" | grep -oE '[0-9]{8}-[0-9]{6}' | head -1 | cut -d- -f1)
  if [ -n "$ymd" ]; then
    printf '%s/%s/%s/%s/%s' "$KEY_PREFIX" "${ymd:0:4}" "${ymd:4:2}" "${ymd:6:2}" "$base"
  else
    printf '%s/unsorted/%s' "$KEY_PREFIX" "$base"
  fi
}

# =============================================================================
# 1. ローテート
# =============================================================================
rotate() {
  if [ ! -f "$LOG_FILE" ]; then
    say "[SKIP] $LOG_FILE が存在しない"
    return 0
  fi
  local size
  size=$(stat -c %s "$LOG_FILE" 2>/dev/null || echo 0)
  if [ "$size" -lt "$MIN_BYTES" ]; then
    say "[SKIP] ログが ${size} バイトしかないためローテートしない"
    return 0
  fi

  local ts out
  ts=$(date '+%Y%m%d-%H%M%S')
  out="$SPOOL/${HOSTTAG}-trading-${ts}.log"

  if ! cp "$LOG_FILE" "$out"; then
    say "[ERROR] cp に失敗したため truncate を中止した（ログは失っていない）"
    return 1
  fi
  # cp 成功後にのみ空にする。mv を使わない理由は冒頭のコメント参照
  if : > "$LOG_FILE"; then
    say "[ROTATE] ${size} bytes -> $(basename "$out")"
  else
    say "[ERROR] truncate に失敗（ファイルシステムが読み取り専用の可能性）"
  fi
  gzip -f "$out"

  # systemd 側の記録（起動/終了/OOM/再起動回数）も一緒に退避する。
  # アプリのログには残らず journal にしか無い情報で、障害調査で最も効く。
  # 取りこぼしを避けるため70分ぶん取り、境界の重複は許容する
  if [ "$INCLUDE_JOURNAL" = "1" ]; then
    local jout="$SPOOL/${HOSTTAG}-journal-${ts}.log"
    if journalctl -u bfTradingApp.service --since '-70 minutes' --no-pager > "$jout" 2>/dev/null && [ -s "$jout" ]; then
      gzip -f "$jout"
      say "[ROTATE] journal -> $(basename "$jout").gz"
    else
      rm -f "$jout"
      say "[SKIP] journalctl を取得できなかった（権限不足なら pi を systemd-journal グループへ）"
    fi
  fi
}

# =============================================================================
# 2. 未送信分の送信
# =============================================================================
flush() {
  local ok=0 ng=0 f base key remain purged
  shopt -s nullglob
  for f in "$SPOOL"/*.gz; do
    base=$(basename "$f")
    key=$(build_key "$base")
    if upload_one "$f" "$key"; then
      rm -f "$f"; ok=$((ok+1)); say "[SENT] $key"
    else
      ng=$((ng+1))
    fi
  done
  shopt -u nullglob
  say "[FLUSH] 送信成功 ${ok}件 / 失敗 ${ng}件"

  remain=$(find "$SPOOL" -name '*.gz' | wc -l | tr -d ' ')
  if [ "$remain" -ge "$SPOOL_WARN_COUNT" ]; then
    local msg="🚨 trading.log の外部退避が ${remain} 件滞留しています。Supabase Storage の容量超過(QuotaExceeded)か通信障害の可能性。Pi の ${SPOOL} と ${SHIP_LOG} を確認してください"
    say "[WARN] $msg"
    notify_slack "$msg"
  fi

  # 送れないまま溜まり続けてSDカードを埋めるのを防ぐ（本来の目的が崩れるため）
  purged=$(find "$SPOOL" -name '*.gz' -mtime "+$MAX_SPOOL_DAYS" -print -delete | wc -l | tr -d ' ')
  [ "$purged" -gt 0 ] && say "[WARN] ${MAX_SPOOL_DAYS}日以上送れなかった ${purged} 件を削除した"
  return 0
}

# =============================================================================
# 3. 保持期間を過ぎたオブジェクトの削除
#
#   Supabase Storage にはライフサイクル設定が無いので自前で消す。1GBの無料枠を
#   超えると猶予期間の後に新規アップロードがブロックされ、退避そのものが止まる。
#
#   trading-log/YYYY/MM/DD/ の階層を辿り、RETENTION_DAYS より古い日だけを対象にする。
#   削除対象は自分が作ったファイル名パターンに限定する（他のオブジェクトには触らない）。
# =============================================================================
prune() {
  local dry=0
  [ "$SUBMODE" = "dry" ] && dry=1
  if [ "$RETENTION_DAYS" -lt 7 ]; then
    say "[ERROR] RETENTION_DAYS=${RETENTION_DAYS} は短すぎる（7以上にすること）"
    return 1
  fi

  STORAGE_API="$STORAGE_API" SUPABASE_SECRET_KEY="$SUPABASE_SECRET_KEY" \
  SUPABASE_BUCKET="$SUPABASE_BUCKET" KEY_PREFIX="$KEY_PREFIX" \
  RETENTION_DAYS="$RETENTION_DAYS" HOSTTAG="$HOSTTAG" DRY="$dry" \
  python3 - <<'PY' 2>&1 | while IFS= read -r line; do say "[PRUNE] $line"; done
import json, os, re, sys, urllib.request, urllib.error
from datetime import date, timedelta

API    = os.environ["STORAGE_API"]
KEY    = os.environ["SUPABASE_SECRET_KEY"]
BUCKET = os.environ["SUPABASE_BUCKET"]
PREFIX = os.environ["KEY_PREFIX"]
KEEP   = int(os.environ["RETENTION_DAYS"])
DRY    = os.environ["DRY"] == "1"

# 自分が作ったファイルだけを対象にする安全弁
SAFE = re.compile(r'^[A-Za-z0-9._-]+-(trading|journal)-\d{8}-\d{6}\.log\.gz$')

def call(method, path, body=None):
    req = urllib.request.Request(
        f"{API}{path}", method=method,
        data=json.dumps(body).encode() if body is not None else None,
        headers={"Authorization": f"Bearer {KEY}", "apikey": KEY,
                 "Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=60) as r:
            return json.loads(r.read() or b"null")
    except urllib.error.HTTPError as e:
        # 認証エラー(401)や容量超過はここに出る
        print(f"HTTP {e.code} on {method} {path}: {e.read()[:200]!r}")
        return None
    except (urllib.error.URLError, OSError) as e:
        # 通信断。削除は「失敗したら何もしない」で倒す（消しすぎるより消さない方が安全）
        print(f"接続失敗 {method} {path}: {e}")
        return None

FAILED = []

def listing(prefix):
    """prefix 直下のエントリ名を返す。folder は id が None で返る。
    取得に失敗した場合は空を返しつつ FAILED に記録する（成功して0件だった場合と区別するため）"""
    out, offset = [], 0
    while True:
        res = call("POST", f"/object/list/{BUCKET}",
                   {"prefix": prefix, "limit": 100, "offset": offset,
                    "sortBy": {"column": "name", "order": "asc"}})
        if res is None:
            FAILED.append(prefix)
            break
        if not res:
            break
        out += res
        if len(res) < 100:
            break
        offset += 100
    return out

cutoff = date.today() - timedelta(days=KEEP)
victims = []

for y in listing(PREFIX):
    if not re.fullmatch(r'\d{4}', y["name"]):
        continue
    for m in listing(f'{PREFIX}/{y["name"]}'):
        if not re.fullmatch(r'\d{2}', m["name"]):
            continue
        for d in listing(f'{PREFIX}/{y["name"]}/{m["name"]}'):
            if not re.fullmatch(r'\d{2}', d["name"]):
                continue
            try:
                day = date(int(y["name"]), int(m["name"]), int(d["name"]))
            except ValueError:
                continue
            if day >= cutoff:
                continue
            base = f'{PREFIX}/{y["name"]}/{m["name"]}/{d["name"]}'
            for f in listing(base):
                if SAFE.fullmatch(f["name"]):
                    victims.append(f'{base}/{f["name"]}')
                else:
                    print(f"skip (想定外の名前なので触らない): {base}/{f['name']}")

if FAILED:
    print(f"[WARN] 一覧取得に失敗したプレフィックスが {len(FAILED)} 件あるため、"
          f"取りこぼしがある。削除は行った分のみ（先頭: {FAILED[0]}）")
print(f"cutoff={cutoff} 削除対象 {len(victims)} 件" + ("（dry-run）" if DRY else ""))
if not victims or DRY:
    sys.exit(0)

# DELETE /object/{bucket} に prefixes 配列で一括削除する
for i in range(0, len(victims), 100):
    chunk = victims[i:i+100]
    if call("DELETE", f"/object/{BUCKET}", {"prefixes": chunk}) is not None:
        print(f"削除 {len(chunk)} 件 (先頭: {chunk[0]})")
PY
}

# =============================================================================
# 4. 疎通確認（trading.log に一切触らない）
# =============================================================================
selftest() {
  local tmp key
  tmp="$SPOOL/${HOSTTAG}-selftest-$(date '+%Y%m%d-%H%M%S').log"
  printf 'ship-trading-log selftest at %s on %s\n' "$(date -Is)" "$HOSTTAG" > "$tmp"
  gzip -f "$tmp"
  key="${KEY_PREFIX}/selftest/$(basename "$tmp").gz"
  if upload_one "$tmp.gz" "$key"; then
    echo "OK: アップロード成功 -> ${SUPABASE_BUCKET}/${key}"
    say "[SELFTEST] OK $key"
    rm -f "$tmp.gz"
    exit 0
  fi
  echo "NG: アップロード失敗。$SHIP_LOG の末尾を確認してください"
  rm -f "$tmp.gz"
  exit 1
}

trim_ship_log

case "$MODE" in
  run)      require_conf || exit 1; rotate; flush ;;
  flush)    require_conf || exit 1; flush ;;
  prune)    require_conf || exit 1; prune ;;
  selftest) require_conf || exit 1; selftest ;;
  *) echo "usage: $0 [run|flush|prune [dry]|selftest]" >&2; exit 2 ;;
esac
