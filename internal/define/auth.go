package define

// User contains only safe identity fields, supplied after authentication.
type User struct {
	ID       string `json:"userId"`
	Username string `json:"username"`
}
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type LoginResult struct {
	UserID string `json:"userId"`
	Token  string `json:"token"`
}
