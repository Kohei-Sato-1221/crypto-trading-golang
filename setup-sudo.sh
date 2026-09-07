#!/bin/bash
# piユーザーがパスワードなしでsystemctl stop/disable bfTradingApp.serviceを実行できるように設定するスクリプト

echo "piユーザーがパスワードなしでsystemctl stop/disable bfTradingApp.serviceを実行できるように設定します..."

# sudoersファイルを作成
echo "pi ALL=(ALL) NOPASSWD: /bin/systemctl stop bfTradingApp.service, /bin/systemctl disable bfTradingApp.service" | sudo tee /etc/sudoers.d/bfTradingApp-stop

# ファイルの権限を設定（sudoersファイルは440である必要がある）
sudo chmod 440 /etc/sudoers.d/bfTradingApp-stop

echo "設定が完了しました。"
echo "テスト: sudo systemctl stop bfTradingApp.service"
echo "テスト: sudo systemctl disable bfTradingApp.service"

