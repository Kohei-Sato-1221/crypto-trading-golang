#!/bin/bash
# trading.logを空ファイルに置き換えるスクリプト

LOG_FILE="/home/pi/workspace/crypto-trading-golang/go/trading.log"

# ファイルが存在する場合のみ処理
if [ -f "$LOG_FILE" ]; then
    # 空ファイルに置き換え
    > "$LOG_FILE"
    echo "$(date '+%Y-%m-%d %H:%M:%S') - trading.log cleared" >> "$LOG_FILE"
fi








