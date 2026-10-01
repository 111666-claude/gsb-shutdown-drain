package drain

import (
	"math/rand"
	"strconv"
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

func TestDrainWaitsForGracePeriod(t *testing.T) {
	server := NewServer([]string{"a", "b"})
	server.Begin(1000)
	if got := server.Drain(1000 + DrainTimeoutMs - 1); got != 0 {
		t.Fatalf("宽限期内不该踢人：kicked=%d", got)
	}
	if got := server.Active(); got != 2 {
		t.Fatalf("宽限期内连接应仍在线：active=%d", got)
	}
	if got := server.Drain(1000 + DrainTimeoutMs); got != 2 {
		t.Fatalf("宽限期一到应踢光剩余连接：kicked=%d", got)
	}
	if got := server.Active(); got != 0 {
		t.Fatalf("踢光后在线数应为 0：active=%d", got)
	}
}

func TestDrainBeforeBeginKicksNobody(t *testing.T) {
	server := NewServer([]string{"a"})
	if got := server.Drain(10 * DrainTimeoutMs); got != 0 {
		t.Fatalf("没开始排空不该踢人：kicked=%d", got)
	}
	if got := server.Active(); got != 1 {
		t.Fatalf("没开始排空连接应在线：active=%d", got)
	}
}

func TestBeginIsIdempotent(t *testing.T) {
	server := NewServer([]string{"a"})
	if !server.Begin(0) {
		t.Fatal("第一次 Begin 应返回 true")
	}
	if server.Begin(100) {
		t.Fatal("排空中再次 Begin 应返回 false")
	}
	// 计时从第一次 Begin 算起，不应被第二次重置。
	if got := server.Drain(100 + DrainTimeoutMs - 1); got != 1 {
		t.Fatalf("重复 Begin 不应重置计时：kicked=%d", got)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	server := NewServer([]string{"a", "b"})
	if got := server.Close([]string{"a"}, 0); got != 1 {
		t.Fatalf("第一次关闭应算一个：kicked=%d", got)
	}
	if got := server.Close([]string{"a", "a", "ghost"}, 10); got != 1 {
		t.Fatalf("重复关闭与未知连接不应重复计数：kicked=%d", got)
	}
	if got := server.Active(); got != 1 {
		t.Fatalf("在线数应剩 1：active=%d", got)
	}
}

func TestActiveQueryCostIsConstant(t *testing.T) {
	const total = 500000
	conns := make([]string, 0, total)
	for index := 0; index < total; index++ {
		conns = append(conns, "c-"+strconv.Itoa(index))
	}
	server := NewServer(conns)
	const queries = 100000
	for query := 0; query < queries; query++ {
		if got := server.Active(); got != total {
			t.Fatalf("在线数不对：%d", got)
		}
	}
	if got := server.Scanned(); got > 3000 {
		t.Fatalf("扫描数不应随连接数线性增长：scanned=%d", got)
	}
}

// refServer 是「宽限期 + 双向幂等」的参考实现，用来对照 Server。
type refServer struct {
	online    map[string]bool
	draining  bool
	startedMs int64
	kicked    int
}

func newRefServer(conns []string) *refServer {
	ref := &refServer{online: map[string]bool{}}
	for _, id := range conns {
		ref.online[id] = true
	}
	return ref
}

func (r *refServer) begin(nowMs int64) bool {
	if r.draining {
		return false
	}
	r.draining = true
	r.startedMs = nowMs
	return true
}

func (r *refServer) close(ids []string) int {
	for _, id := range ids {
		if r.online[id] {
			delete(r.online, id)
			r.kicked++
		}
	}
	return r.kicked
}

func (r *refServer) drain(nowMs int64) int {
	if r.draining && nowMs-r.startedMs >= DrainTimeoutMs {
		r.kicked += len(r.online)
		r.online = map[string]bool{}
	}
	return r.kicked
}

func (r *refServer) active() int { return len(r.online) }

type scriptStep struct {
	op  string
	ids []string
	now int64
}

func TestMatchesReferenceModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20261001))
	pool := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	for trial := 0; trial < 200; trial++ {
		conns := append([]string(nil), pool[:rng.Intn(len(pool)+1)]...)
		var script []scriptStep
		var now int64
		steps := 1 + rng.Intn(30)
		for step := 0; step < steps; step++ {
			now += rng.Int63n(2 * DrainTimeoutMs)
			switch rng.Intn(3) {
			case 0:
				script = append(script, scriptStep{op: "begin", now: now})
			case 1:
				var ids []string
				for count := rng.Intn(4); count >= 0; count-- {
					ids = append(ids, pool[rng.Intn(len(pool))])
				}
				script = append(script, scriptStep{op: "close", ids: ids, now: now})
			case 2:
				script = append(script, scriptStep{op: "drain", now: now})
			}
		}
		server := NewServer(conns)
		replay := NewServer(conns)
		ref := newRefServer(conns)
		for step, op := range script {
			switch op.op {
			case "begin":
				got, again := server.Begin(op.now), replay.Begin(op.now)
				if want := ref.begin(op.now); got != want {
					t.Fatalf("trial %d step %d: Begin=%v，参考=%v", trial, step, got, want)
				}
				if got != again {
					t.Fatalf("trial %d step %d: 重放 Begin 结果不一致", trial, step)
				}
			case "close":
				got, again := server.Close(op.ids, op.now), replay.Close(op.ids, op.now)
				if want := ref.close(op.ids); got != want {
					t.Fatalf("trial %d step %d: Close 踢出数=%d，参考=%d", trial, step, got, want)
				}
				if got != again {
					t.Fatalf("trial %d step %d: 重放 Close 结果不一致", trial, step)
				}
			case "drain":
				got, again := server.Drain(op.now), replay.Drain(op.now)
				if want := ref.drain(op.now); got != want {
					t.Fatalf("trial %d step %d: Drain 踢出数=%d，参考=%d", trial, step, got, want)
				}
				if got != again {
					t.Fatalf("trial %d step %d: 重放 Drain 结果不一致", trial, step)
				}
			}
			if got, want := server.Active(), ref.active(); got != want {
				t.Fatalf("trial %d step %d: 在线数=%d，参考=%d", trial, step, got, want)
			}
			if server.Active() > len(conns) {
				t.Fatalf("trial %d step %d: 在线数超过初始连接数", trial, step)
			}
			if server.Scanned() != 0 {
				t.Fatalf("trial %d step %d: 查询不该扫描连接表", trial, step)
			}
		}
	}
}
