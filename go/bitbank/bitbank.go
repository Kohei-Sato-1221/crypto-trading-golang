package bitbank

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

const baseUrl = "https://public.bitbank.cc/"

type Ticker01 struct {
	Success int       `json:"success"`
	Data    *Ticker02 `json:"data"`
}

type Ticker02 struct {
	Sell      string `json:"sell"`
	Buy       string `json:"buy"`
	High      string `json:"high"`
	Low       string `json:"low"`
	Last      string `json:"last"`
	Vol       string `json:"vol"`
	Timestamp int    `json:"timestamp"`
}

type ReturnTicker struct {
	Sell      float64
	Buy       float64
	High      float64
	Low       float64
	Last      float64
	Vol       int
	Timestamp int
}

/*
GetBBTicker は bitbank の公開Ticker APIから価格を取得する。

以前は HTTP エラー・JSONパース失敗・data が null のレスポンスをいずれも握り潰し、
resp.Body / ticker01.Data を nil のまま参照して panic していた。
戻り値は発注価格の算出に使われるため、取得できなかった場合は必ず error を返し、
呼び出し側が「発注しない」側に倒せるようにする。
*/
func GetBBTicker(pair string) (*ReturnTicker, error) {
	resp, err := http.Get(baseUrl + pair + "/ticker")
	if err != nil {
		return nil, fmt.Errorf("failed to call bitbank ticker API (pair=%s): %w", pair, err)
	}
	defer resp.Body.Close()

	byteArray, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read bitbank ticker response (pair=%s): %w", pair, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bitbank ticker API returned status %d (pair=%s)", resp.StatusCode, pair)
	}
	return parseBBTicker(pair, byteArray)
}

/*
parseBBTicker はレスポンスボディを ReturnTicker に変換する。

data が欠けているレスポンス（エラー応答）や数値として解釈できない価格を
nil 参照・ゼロ値のまま通さず、error として返す。
vol だけは小数を含む値が返ることがあり整数化に失敗しても価格算出に影響しないため、
best-effort（失敗時は0）で扱う。
*/
func parseBBTicker(pair string, body []byte) (*ReturnTicker, error) {
	var ticker01 Ticker01
	if err := json.Unmarshal(body, &ticker01); err != nil {
		return nil, fmt.Errorf("failed to parse bitbank ticker response (pair=%s): %w", pair, err)
	}
	if ticker01.Data == nil {
		return nil, fmt.Errorf("bitbank ticker response has no data (pair=%s success=%d)", pair, ticker01.Success)
	}

	parse := func(name, value string) (float64, error) {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse bitbank ticker %s (pair=%s value=%q): %w", name, pair, value, err)
		}
		return parsed, nil
	}

	tsell, err := parse("sell", ticker01.Data.Sell)
	if err != nil {
		return nil, err
	}
	tbuy, err := parse("buy", ticker01.Data.Buy)
	if err != nil {
		return nil, err
	}
	thigh, err := parse("high", ticker01.Data.High)
	if err != nil {
		return nil, err
	}
	tlow, err := parse("low", ticker01.Data.Low)
	if err != nil {
		return nil, err
	}
	tlast, err := parse("last", ticker01.Data.Last)
	if err != nil {
		return nil, err
	}
	tvol, _ := strconv.Atoi(ticker01.Data.Vol)

	return &ReturnTicker{
		Sell:      tsell,
		Buy:       tbuy,
		High:      thigh,
		Low:       tlow,
		Last:      tlast,
		Vol:       tvol,
		Timestamp: ticker01.Data.Timestamp,
	}, nil
}
