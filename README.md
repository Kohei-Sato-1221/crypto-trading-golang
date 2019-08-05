# BitcoinTrading_Golang
Automated Bitcoin trading web application implemented by Go Lang(Under development)

1. Calling Bitflyer API  


【jobの種類】
1.buyOrderJob:
・指定の周期で買い注文を発注するジョブ
・買い注文が発注したら以下のデータをinsertする
・
　[Table:buyorder]orderid, pair, volume, price, orderdate, exchange, filled 

2.filledCheckJob:
・指定の周期で買い注文の約定具合をチェックするジョブ
・買い注文が約定していた場合、buyorderテーブルのfilledをtrueにする

3.sellOrderJob:
・指定の周期で売り注文を発注するジョブ
・buyoerderのレコードでfilledがtrueかつ、sellorderに該当のorderidがない場合売り注文を出す。
・売り注文が発注できたら以下のデータをinsertする。
　[Table:sellorder]buyorderid, orderid, pair, volume, price, orderdate, exchange, filled 


【TODO】
・売り注文のレコードをDBに保存する機能（売り値）
・売り注文の約定具合をチェックするジョブ
・買い注文を取り消す機能
・Filledが0のレコードが一定以下であれば、フラグをONにして、注文を開始するロジック