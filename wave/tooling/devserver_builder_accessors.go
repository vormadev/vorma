package tooling

// getBuilder returns the current builder instance safely.
func (s *server) getBuilder() *Builder {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.builder
}

// setBuilder sets the builder instance safely.
func (s *server) setBuilder(b *Builder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.builder = b
}
