package bitbank

import (
	"net/http"
	"io/ioutil"
	"encoding/json"
	"log"
	"strconv"
)

const baseUrl = "https://public.bitbank.cc/"

type Ticker01 struct {
	Success    int    `json:"success"`
	Data       *Ticker02 `json:"data"`
}

type Ticker02 struct {
	Sell      string  `json:"sell"`
	Buy       string  `json:"buy"`
	High      string  `json:"high"`
	Low       string  `json:"low"`
	Last      string  `json:"last"`
	Vol       string  `json:"vol"`
	Timestamp int     `json:"timestamp"`
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


func GetBBTicker() *ReturnTicker{
	resp, _ := http.Get(baseUrl + "btc_jpy/ticker")
	defer resp.Body.Close()
	
	byteArray, _ := ioutil.ReadAll(resp.Body)
	
	var ticker01 Ticker01
	err := json.Unmarshal(byteArray, &ticker01)
	if err != nil{
		log.Printf("ERROR! GetBBTicker err=%s resp.Body=%s", err.Error(), string(byteArray))
	}
		
	tsell, _ := strconv.ParseFloat(ticker01.Data.Sell, 64)
	tbuy, _  := strconv.ParseFloat(ticker01.Data.Buy, 64)
	thigh, _ := strconv.ParseFloat(ticker01.Data.High, 64)
	tlow, _  := strconv.ParseFloat(ticker01.Data.Low, 64)
	tlast, _ := strconv.ParseFloat(ticker01.Data.Last, 64)
	tvol, _ := strconv.Atoi(ticker01.Data.Vol)
	retTicker := ReturnTicker{
		Sell       : tsell,
		Buy        : tbuy,
		High       : thigh,
		Low        : tlow,
		Last       : tlast,
		Vol        : tvol,
		Timestamp  : ticker01.Data.Timestamp,
	}	
	return &retTicker
}


