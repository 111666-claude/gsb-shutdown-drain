// Package drain 是优雅关服：连接排空、超时强制踢、重复请求幂等。
package drain

// DrainTimeoutMs 是排空的宽限期。
const DrainTimeoutMs = 30000

// Server 是一台待关服的服务器。
type Server struct {
	conns     map[string]bool
	active    int
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
	server.active = len(server.conns)
	return server
}

// Begin 开始排空。已在排空中再次调用返回 false，且不重置计时。
func (s *Server) Begin(nowMs int64) bool {
	if s.draining {
		return false
	}
	s.draining = true
	s.startedMs = nowMs
	return true
}

// Close 关闭一批连接。只统计真正在线的连接，重复关闭不重复计数。
func (s *Server) Close(ids []string, nowMs int64) int {
	for _, id := range ids {
		if s.conns[id] {
			delete(s.conns, id)
			s.active--
			s.kicked++
		}
	}
	return s.kicked
}

// Drain 在宽限期结束后强制踢人。没开始排空或仍在宽限期内时不踢人。
func (s *Server) Drain(nowMs int64) int {
	if !s.draining || nowMs-s.startedMs < DrainTimeoutMs {
		return s.kicked
	}
	for id := range s.conns {
		delete(s.conns, id)
		s.kicked++
	}
	s.active = 0
	return s.kicked
}

// Active 是在线连接数。O(1) 读计数器，不扫连接表。
func (s *Server) Active() int {
	return s.active
}

// Scanned 是累计扫描的连接数（规模观测）。
func (s *Server) Scanned() int { return s.scanned }
