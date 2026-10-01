package drain

import (
	"fmt"
	"math/rand"
	"testing"
)

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

func TestDrainRespectsGracePeriod(t *testing.T) {
	server := NewServer([]string{"a", "b"})
	server.Begin(1000)
	if got := server.Drain(1000 + DrainTimeoutMs - 1); got != 0 {
		t.Fatalf("宽限期内不该踢人：%d", got)
	}
	if got := server.Active(); got != 2 {
		t.Fatalf("宽限期内连接应该还在：%d", got)
	}
	if got := server.Drain(1000 + DrainTimeoutMs); got != 2 {
		t.Fatalf("宽限期结束应该踢掉两人：%d", got)
	}
	if got := server.Active(); got != 0 {
		t.Fatalf("踢完后不该有在线连接：%d", got)
	}
}

func TestDrainBeforeBeginKicksNobody(t *testing.T) {
	server := NewServer([]string{"a"})
	if got := server.Drain(DrainTimeoutMs * 10); got != 0 {
		t.Fatalf("没开始排空不该踢人：%d", got)
	}
}

func TestBeginIsIdempotent(t *testing.T) {
	server := NewServer([]string{"a"})
	if !server.Begin(0) {
		t.Fatal("第一次开始排空应该返回 true")
	}
	if server.Begin(100) {
		t.Fatal("重复开始排空应该返回 false")
	}
	// 计时从第一次 Begin 算起，不能被重置。
	if got := server.Drain(DrainTimeoutMs); got != 1 {
		t.Fatalf("计时不该被重置：%d", got)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	server := NewServer([]string{"a", "b"})
	server.Close([]string{"a"}, 0)
	if got := server.Close([]string{"a"}, 10); got != 1 {
		t.Fatalf("重复关闭不该重复计数：%d", got)
	}
	if got := server.Close([]string{"ghost"}, 20); got != 1 {
		t.Fatalf("不存在的连接不该计数：%d", got)
	}
	if got := server.Active(); got != 1 {
		t.Fatalf("在线数不对：%d", got)
	}
}

func TestActiveNeverExceedsInitial(t *testing.T) {
	server := NewServer([]string{"a", "b"})
	server.Close([]string{"a", "a", "ghost"}, 0)
	server.Begin(0)
	server.Drain(DrainTimeoutMs)
	if got := server.Active(); got > 2 || got < 0 {
		t.Fatalf("在线数越界：%d", got)
	}
}

func TestActiveQueryCostIsConstant(t *testing.T) {
	conns := make([]string, 0, 500000)
	for index := 0; index < 500000; index++ {
		conns = append(conns, fmt.Sprintf("c-%d", index))
	}
	server := NewServer(conns)
	for query := 0; query < 100000; query++ {
		server.Active()
	}
	if got := server.Scanned(); got > 100000 {
		t.Fatalf("扫描数不该随连接数增长：%d", got)
	}
}

// reference 是「宽限期 + 双向幂等」的参考实现。
type reference struct {
	conns     map[string]bool
	draining  bool
	startedMs int64
	kicked    int
}

func newReference(conns []string) *reference {
	ref := &reference{conns: map[string]bool{}}
	for _, id := range conns {
		ref.conns[id] = true
	}
	return ref
}

func (r *reference) begin(nowMs int64) bool {
	if r.draining {
		return false
	}
	r.draining = true
	r.startedMs = nowMs
	return true
}

func (r *reference) close(ids []string) int {
	for _, id := range ids {
		if r.conns[id] {
			delete(r.conns, id)
			r.kicked++
		}
	}
	return r.kicked
}

func (r *reference) drain(nowMs int64) int {
	if !r.draining || nowMs-r.startedMs < DrainTimeoutMs {
		return r.kicked
	}
	r.kicked += len(r.conns)
	r.conns = map[string]bool{}
	return r.kicked
}

func (r *reference) active() int { return len(r.conns) }

func TestMatchesReferenceOnRandomSequences(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for trial := 0; trial < 200; trial++ {
		ids := make([]string, 0, 8)
		for index := 0; index < 8; index++ {
			ids = append(ids, fmt.Sprintf("c-%d", index))
		}
		initial := ids[:rng.Intn(len(ids))+1]
		server := NewServer(initial)
		ref := newReference(initial)
		now := rng.Int63n(1000)
		for step := 0; step < 40; step++ {
			switch rng.Intn(4) {
			case 0:
				if got, want := server.Begin(now), ref.begin(now); got != want {
					t.Fatalf("trial %d step %d: Begin=%v 参考=%v", trial, step, got, want)
				}
			case 1:
				batch := make([]string, 0, 4)
				for count := rng.Intn(4); count >= 0; count-- {
					batch = append(batch, ids[rng.Intn(len(ids))])
				}
				if got, want := server.Close(batch, now), ref.close(batch); got != want {
					t.Fatalf("trial %d step %d: Close 踢出数=%d 参考=%d", trial, step, got, want)
				}
			case 2:
				if got, want := server.Drain(now), ref.drain(now); got != want {
					t.Fatalf("trial %d step %d: Drain 踢出数=%d 参考=%d", trial, step, got, want)
				}
			case 3:
				now += rng.Int63n(DrainTimeoutMs * 2)
			}
			if got, want := server.Active(), ref.active(); got != want {
				t.Fatalf("trial %d step %d: 在线数=%d 参考=%d", trial, step, got, want)
			}
		}
	}
}
