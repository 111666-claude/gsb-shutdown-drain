// Package drain 是优雅关服：连接排空、超时强制踢、重复请求幂等。
package drain

// DrainTimeoutMs 是排空的宽限期。
const DrainTimeoutMs = 30000

// Server 是一台待关服的服务器。
type Server struct {
	conns     map[string]struct{}
	active    int
	draining  bool
	startedMs int64
	kicked    int
	scanned   int
}

// NewServer 按在线连接建服务器。
func NewServer(conns []string) *Server {
	server := &Server{conns: map[string]struct{}{}}
	for _, id := range conns {
		if _, ok := server.conns[id]; !ok {
			server.conns[id] = struct{}{}
			server.active++
		}
	}
	return server
}

// Begin 开始排空。已在排空中返回 false，且不重置计时。
func (s *Server) Begin(nowMs int64) bool {
	if s.draining {
		return false
	}
	s.draining = true
	s.startedMs = nowMs
	return true
}

// Close 关闭一批连接，只统计真正在线的连接，返回累计踢出数。
func (s *Server) Close(ids []string, nowMs int64) int {
	for _, id := range ids {
		if _, ok := s.conns[id]; ok {
			delete(s.conns, id)
			s.active--
			s.kicked++
		}
	}
	return s.kicked
}

// Drain 宽限期结束后强制踢人，返回累计踢出数；宽限期内不踢人。
func (s *Server) Drain(nowMs int64) int {
	if !s.draining || nowMs-s.startedMs < DrainTimeoutMs {
		return s.kicked
	}
	for id := range s.conns {
		delete(s.conns, id)
	}
	s.kicked += s.active
	s.active = 0
	return s.kicked
}

// Active 是在线连接数，单次摊还 O(1)。
func (s *Server) Active() int {
	s.scanned++
	return s.active
}

// Scanned 是累计扫描的连接数（规模观测）。
func (s *Server) Scanned() int { return s.scanned }
