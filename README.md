# BitcoinTrading_Golang
Automated Bitcoin trading web application implemented by Go Lang(Under development)

【Build for Amazon Linux】  
GOOS=linux GOARCH=amd64 go build src/main/main.go

【Send binary file to EC2】  
scp -i ~/.ssh/xxxxxxx.pem ./main ec2-user@xx.xx.xx.xx:/home/ec2-user/application/
scp -i ~/.ssh/xxxxxxx.pem ./config.ini ec2-user@xx.xx.xx.xx:/home/ec2-user/application/
scp -i ~/.ssh/xxxxxxx.pem ./private_config.ini ec2-user@xx.xx.xx.xx:/home/ec2-user/application/


【kind of jobs】  
1.buyOrderJob:  
・指定の周期で買い注文を発注するジョブ  
・買い注文が発注したら以下のデータをinsertする(filledはデフォルトの0)  
　[Table:buyorder]orderid, pair, volume, price, orderdate, exchange, filled  

2.filledCheckJob:  
・指定の周期で買い・売り注文の約定具合をチェックするジョブ  
・買い注文が約定していた場合、buy_orders, sell_ordersテーブルのfilledを2にする  

3.sellOrderJob:  
・指定の周期で売り注文を発注するジョブ  
・buy_oerdersのレコードでfilledが1の場合売り注文を出す。  
・売り注文が発注できたら以下のデータをinsertする。  
　[Table:sellorder]buyorderid, orderid, pair, volume, price, orderdate, exchange, filled 
・また、buy_ordersのfilledを2にupdateする 


## TODO 
・手動で入れた買い注文をDBに登録するジョブ
・買い注文を取り消す機能  
・複数の注文パターン（売値・買値のロジックを複数に）
・注文量・注文金額・最大並行注文数・ジョブの実行間隔をパラメーター化  
・利益計算の方法（bitflyerの機能の損益見れるから、優先度は低い   
