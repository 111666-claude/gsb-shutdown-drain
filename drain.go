// Package drain 是优雅关服：连接排空、超时强制踢、重复请求幂等。
// 缺陷：Begin 重复调用会重置排空计时、Close 重复关闭重复计数、Drain 不等超时就全踢、Active 全扫。
package drain

// DrainTimeoutMs 是排空的宽限期。
const DrainTimeoutMs = 30000

// Server 是一台待关服的服务器。
type Server struct {
	conns     map[string]bool
	draining  bool
	startedMs int64
	kicked    int
	scanned   int
}

// NewServer 按在线连接建服务器。
func NewServer(conns []string) *Server {
	server := &Server{conns: map[string]bool{}}
	for _, id := range conns {
		server.conns[id] = true
	}
	return server
}

// Begin 开始排空。缺陷：重复调用会重置计时，而且总是返回 true。
func (s *Server) Begin(nowMs int64) bool {
	s.draining = true
	s.startedMs = nowMs
	return true
}

// Close 关闭一批连接。缺陷：不看连接是否还在线，重复关闭也累加踢出数。
func (s *Server) Close(ids []string, nowMs int64) int {
	for _, id := range ids {
		delete(s.conns, id)
		s.kicked++
	}
	return s.kicked
}

// Drain 宽限期结束后强制踢人。缺陷：不等超时就把所有剩余连接踢掉。
func (s *Server) Drain(nowMs int64) int {
	for id := range s.conns {
		delete(s.conns, id)
		s.kicked++
	}
	return s.kicked
}

// Active 是在线连接数。缺陷：每次全扫连接表。
func (s *Server) Active() int {
	s.scanned += len(s.conns)
	return len(s.conns)
}

// Scanned 是累计扫描的连接数（规模观测）。
func (s *Server) Scanned() int { return s.scanned }
