#!/bin/bash
# システムリソース使用率を正確に表示するスクリプト

echo "=== システムリソース使用状況 ==="
echo ""

# CPU使用率の計算（topコマンドから直接計算）
CPU_LINE=$(top -bn1 | grep "Cpu(s)")
if [ -n "$CPU_LINE" ]; then
    # us + sy + wa を合計（idを除く）
    CPU_US=$(echo "$CPU_LINE" | grep -oE '[0-9]+\.[0-9]+[[:space:]]+us' | awk '{print $1}')
    CPU_SY=$(echo "$CPU_LINE" | grep -oE '[0-9]+\.[0-9]+[[:space:]]+sy' | awk '{print $1}')
    CPU_WA=$(echo "$CPU_LINE" | grep -oE '[0-9]+\.[0-9]+[[:space:]]+wa' | awk '{print $1}')
    CPU_HI=$(echo "$CPU_LINE" | grep -oE '[0-9]+\.[0-9]+[[:space:]]+hi' | awk '{print $1}')
    CPU_SI=$(echo "$CPU_LINE" | grep -oE '[0-9]+\.[0-9]+[[:space:]]+si' | awk '{print $1}')
    CPU_ST=$(echo "$CPU_LINE" | grep -oE '[0-9]+\.[0-9]+[[:space:]]+st' | awk '{print $1}')
    
    # 各値を0に初期化（見つからない場合）
    CPU_US=${CPU_US:-0}
    CPU_SY=${CPU_SY:-0}
    CPU_WA=${CPU_WA:-0}
    CPU_HI=${CPU_HI:-0}
    CPU_SI=${CPU_SI:-0}
    CPU_ST=${CPU_ST:-0}
    
    # CPU使用率を計算（us + sy + wa + hi + si + st）
    CPU_USAGE=$(echo "scale=2; $CPU_US + $CPU_SY + $CPU_WA + $CPU_HI + $CPU_SI + $CPU_ST" | bc 2>/dev/null)
    
    if [ -n "$CPU_USAGE" ]; then
        echo "CPU使用率: ${CPU_USAGE}%"
    else
        echo "CPU使用率: 計算エラー"
        echo "デバッグ情報:"
        echo "  us=$CPU_US, sy=$CPU_SY, wa=$CPU_WA"
        echo "  hi=$CPU_HI, si=$CPU_SI, st=$CPU_ST"
    fi
else
    echo "CPU使用率: 取得エラー（topコマンドの出力が見つかりません）"
fi

# メモリ使用率
MEM_TOTAL=$(free | grep Mem | awk '{print $2}')
MEM_USED=$(free | grep Mem | awk '{print $3}')
MEM_FREE=$(free | grep Mem | awk '{print $4}')
MEM_AVAILABLE=$(free | grep Mem | awk '{print $7}')

if [ -n "$MEM_AVAILABLE" ] && [ "$MEM_AVAILABLE" != "0" ]; then
    MEM_USAGE=$(echo "scale=2; ($MEM_TOTAL - $MEM_AVAILABLE) * 100 / $MEM_TOTAL" | bc)
else
    MEM_USAGE=$(echo "scale=2; $MEM_USED * 100 / $MEM_TOTAL" | bc)
fi

echo "メモリ使用率: ${MEM_USAGE}%"
echo ""
echo "メモリ詳細:"
free -h
echo ""
echo "CPU詳細:"
top -bn1 | grep "Cpu(s)" | head -1
