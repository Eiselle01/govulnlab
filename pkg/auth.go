package pkg

// IsAdmin 判断session是否管理员
func IsAdmin(s *SessionData) bool {
	return s.Role == "admin"
}