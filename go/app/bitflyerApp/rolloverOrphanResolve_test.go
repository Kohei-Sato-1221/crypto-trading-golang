package bitflyerApp

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/slack"
)

/*
E2（F3 のフェイルセーフ）の回帰テスト。

前回の PlaceOrder はリクエストが届いたのにレスポンスを取りこぼした場合、
DBには記録が無いまま取引所にだけ売り注文が残る（オーファン注文）。
旧 order_id の個別照会では見つからないため、確認せずに再発注すると
同一ポジションに売り注文が2本並ぶ。拘束されていない現物（手動保有分65件）が
あると2本とも約定しうるため、**確認できないときは必ず再発注を見送る**必要がある。

このフェイルセーフは壊れると二重売りに直結するため、内部ヘルパーではなく
resolveRolloverOrphan() を直接呼んで検証する。
*/

// --- テスト用の Slack / DB スタブ ---

/*
setTestSlackClient は slackClient をローカルのテストサーバ向けに差し替える。

resolveRolloverOrphan はフェイルセーフ発動時に必ずSlack通知するため、
slackClient が nil のままだとジョブ関数を直接呼べない。
apiURL を渡すと sendMessageToSlackV2（http.PostForm）が使われるので、
外部（slack.com）へは一切リクエストが飛ばない。
戻り値で通知本文を取り出せる。
*/
func setTestSlackClient(t *testing.T) func() []string {
	t.Helper()

	var posted []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		posted = append(posted, string(body))
		w.WriteHeader(http.StatusOK)
	}))
	original := slackClient
	slackClient = slack.NewSlack("test-token", "#ch", "#err", server.URL)
	t.Cleanup(func() {
		slackClient = original
		server.Close()
	})
	return func() []string { return posted }
}

/*
orphanStubDB は models.OrderIDExists の結果を制御するためのダミー sql ドライバ。

exists=true/false を返す経路と、DB照合そのものが失敗する経路の両方を再現する。
*/
type orphanStubDB struct {
	exists   bool
	queryErr error
}

var orphanStub orphanStubDB

type orphanStubDriver struct{}

func (orphanStubDriver) Open(string) (driver.Conn, error) { return orphanStubConn{}, nil }

type orphanStubConn struct{}

func (orphanStubConn) Prepare(string) (driver.Stmt, error) { return orphanStubStmt{}, nil }
func (orphanStubConn) Close() error                        { return nil }
func (orphanStubConn) Begin() (driver.Tx, error)           { return nil, errors.New("begin is not supported") }

type orphanStubStmt struct{}

func (orphanStubStmt) Close() error  { return nil }
func (orphanStubStmt) NumInput() int { return -1 }
func (orphanStubStmt) Exec([]driver.Value) (driver.Result, error) {
	return nil, errors.New("exec is not supported")
}
func (orphanStubStmt) Query([]driver.Value) (driver.Rows, error) {
	if orphanStub.queryErr != nil {
		return nil, orphanStub.queryErr
	}
	return &orphanStubRows{value: orphanStub.exists}, nil
}

type orphanStubRows struct {
	value bool
	done  bool
}

func (*orphanStubRows) Columns() []string { return []string{"exists"} }
func (*orphanStubRows) Close() error      { return nil }
func (r *orphanStubRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = r.value
	return nil
}

func init() {
	sql.Register("orphanstub", orphanStubDriver{})
}

// setStubDB は models.AppDB をダミーDBへ差し替える。
func setStubDB(t *testing.T, exists bool, queryErr error) {
	t.Helper()

	db, err := sql.Open("orphanstub", "")
	if err != nil {
		t.Fatalf("failed to open stub db: %v", err)
	}
	orphanStub = orphanStubDB{exists: exists, queryErr: queryErr}
	original := models.AppDB
	models.AppDB = db
	t.Cleanup(func() {
		models.AppDB = original
		orphanStub = orphanStubDB{}
		db.Close()
	})
}

// --- フェイルセーフ本体 ---

// ACTIVE一覧を取得できていない場合は再発注を見送ること（フェイルセーフ）。
func TestResolveRolloverOrphanRequiresActiveList(t *testing.T) {
	messages := setTestSlackClient(t)
	record := orphanTestRecord()

	// index は非nilだが activeOK=false（GetChildOrdersAll が失敗したケース）
	index := &rolloverOrderIndex{completed: map[string]bool{}, activeOK: false}
	orphan, proceed := resolveRolloverOrphan(index, record)
	if proceed {
		t.Error("ACTIVE一覧が無いのに再発注してよいと判定している（二重売りのリスク）")
	}
	if orphan != nil {
		t.Errorf("判定不能なのにオーファン注文を返している: %+v", orphan)
	}
	if len(messages()) == 0 {
		t.Error("フェイルセーフ発動時にSlack通知が飛んでいない")
	}
}

// index 自体が nil の場合も再発注を見送ること（フェイルセーフ）。
func TestResolveRolloverOrphanNilIndex(t *testing.T) {
	messages := setTestSlackClient(t)

	orphan, proceed := resolveRolloverOrphan(nil, orphanTestRecord())
	if proceed {
		t.Error("index が nil なのに再発注してよいと判定している（二重売りのリスク）")
	}
	if orphan != nil {
		t.Errorf("判定不能なのにオーファン注文を返している: %+v", orphan)
	}
	if len(messages()) == 0 {
		t.Error("フェイルセーフ発動時にSlack通知が飛んでいない")
	}
}

// DB照合に失敗した場合は再発注を見送ること（フェイルセーフ）。
func TestResolveRolloverOrphanDBLookupFailure(t *testing.T) {
	messages := setTestSlackClient(t)
	setStubDB(t, false, errors.New("db is unavailable"))

	record := orphanTestRecord()
	index := &rolloverOrderIndex{
		completed: map[string]bool{},
		active:    []bitflyer.Order{orphanTestOrder("ORPHAN-1")},
		activeOK:  true,
	}

	orphan, proceed := resolveRolloverOrphan(index, record)
	if proceed {
		t.Error("DB照合に失敗したのに再発注してよいと判定している（二重売りのリスク）")
	}
	if orphan != nil {
		t.Errorf("判定不能なのにオーファン注文を返している: %+v", orphan)
	}
	if len(messages()) == 0 {
		t.Error("フェイルセーフ発動時にSlack通知が飛んでいない")
	}
}

// DBに紐づかない同一条件の ACTIVE 注文はオーファンとして返すこと（再発注しない）。
func TestResolveRolloverOrphanFindsOrphan(t *testing.T) {
	setTestSlackClient(t)
	setStubDB(t, false, nil) // OrderIDExists = false（DBに記録が無い）

	record := orphanTestRecord()
	index := &rolloverOrderIndex{
		completed: map[string]bool{},
		active:    []bitflyer.Order{orphanTestOrder("ORPHAN-1")},
		activeOK:  true,
	}

	orphan, proceed := resolveRolloverOrphan(index, record)
	if !proceed {
		t.Fatal("判定はできているので proceed=true の想定")
	}
	if orphan == nil {
		t.Fatal("オーファン注文を検出できていない（このまま再発注すると売り注文が2本並ぶ）")
	}
	if orphan.ChildOrderAcceptanceID != "ORPHAN-1" {
		t.Errorf("OrphanOrderID = %s, want ORPHAN-1", orphan.ChildOrderAcceptanceID)
	}
}

// DBに紐づく注文は他レコードのものなのでオーファンとしないこと。
func TestResolveRolloverOrphanSkipsKnownOrder(t *testing.T) {
	setTestSlackClient(t)
	setStubDB(t, true, nil) // OrderIDExists = true（DBに記録がある）

	record := orphanTestRecord()
	index := &rolloverOrderIndex{
		completed: map[string]bool{},
		active:    []bitflyer.Order{orphanTestOrder("KNOWN-SELL")},
		activeOK:  true,
	}

	orphan, proceed := resolveRolloverOrphan(index, record)
	if !proceed {
		t.Fatal("DB照合できているので proceed=true の想定")
	}
	if orphan != nil {
		t.Errorf("DBに紐づく注文をオーファンと誤判定している: %+v", orphan)
	}
}

// リグレッション: 候補が無ければ再発注してよいと判定すること（DBには触れない）。
func TestResolveRolloverOrphanNoCandidateAllowsReorder(t *testing.T) {
	setTestSlackClient(t)

	record := orphanTestRecord()
	buyOrder := orphanTestOrder("ACTIVE-BUY")
	buyOrder.Side = "BUY"
	otherSize := orphanTestOrder("ACTIVE-OTHERSIZE")
	otherSize.Size = 0.05
	self := orphanTestOrder(record.OrderID)

	index := &rolloverOrderIndex{
		completed: map[string]bool{},
		active:    []bitflyer.Order{buyOrder, otherSize, self},
		activeOK:  true,
	}

	orphan, proceed := resolveRolloverOrphan(index, record)
	if !proceed {
		t.Fatal("候補が無いので再発注してよい想定")
	}
	if orphan != nil {
		t.Errorf("候補が無いのにオーファンを返している: %+v", orphan)
	}

	// ACTIVE一覧が空の場合も同様（activeOK=true なら判定できている）
	empty := &rolloverOrderIndex{completed: map[string]bool{}, activeOK: true}
	if orphan, proceed := resolveRolloverOrphan(empty, record); !proceed || orphan != nil {
		t.Errorf("ACTIVE一覧が空なら再発注可の想定: orphan=%+v proceed=%v", orphan, proceed)
	}
}
