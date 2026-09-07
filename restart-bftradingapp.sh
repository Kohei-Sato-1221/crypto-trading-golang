#!/bin/bash
# ラズパイ再起動時にbfTradingAppサービスを再起動するスクリプト

LOG_FILE="/home/pi/workspace/crypto-trading-golang/go/trading.log"

# サービスが有効化されているか確認
if systemctl is-enabled bfTradingApp.service > /dev/null 2>&1; then
    # 少し待機してからサービスを再起動（サービスが完全に起動するのを待つ）
    sleep 5
    # サービスを再起動
    systemctl restart bfTradingApp.service
    echo "$(date '+%Y/%m/%d %H:%M:%S'): bfTradingApp.service restarted on boot" >> "$LOG_FILE"
else
    echo "$(date '+%Y/%m/%d %H:%M:%S'): bfTradingApp.service is not enabled" >> "$LOG_FILE"
fi

