package drain

import "testing"

func TestActiveCountsConnections(t *testing.T) {
	if got := NewServer([]string{"a", "b"}).Active(); got != 2 {
		t.Fatalf("在线连接数不对：%d", got)
	}
}

func TestBeginReturnsTrue(t *testing.T) {
	if !NewServer([]string{"a"}).Begin(0) {
		t.Fatal("第一次开始排空应该返回 true")
	}
}

func TestFirstCloseCounts(t *testing.T) {
	server := NewServer([]string{"a", "b"})
	if got := server.Close([]string{"a"}, 0); got != 1 {
		t.Fatalf("第一次关闭应该只算一个：%d", got)
	}
}

func TestScannedStartsAtZero(t *testing.T) {
	if NewServer(nil).Scanned() != 0 {
		t.Fatal("初始扫描数应该是 0")
	}
}
