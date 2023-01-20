package main

type Session struct {
	Options *ScanRepositoriesConfig
	Config  *Config
}

func (s *Session) InitLogger() {
}
